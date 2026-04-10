package controllers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openclawiov1 "github.com/Frank-svg-dev/openclaw-operator/api/v1"
)

const openclawFinalizer = "openclaw.io/finalizer"

type OpenclawReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *OpenclawReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	openclaw := &openclawiov1.Openclaw{}
	err := r.Get(ctx, req.NamespacedName, openclaw)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Openclaw resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Openclaw resource.")
		return ctrl.Result{}, err
	}

	if openclaw.ObjectMeta.DeletionTimestamp != nil {
		if containsOpenClawString(openclaw.ObjectMeta.Finalizers, openclawFinalizer) {
			logger.Info("Deleting OpenClaw, cleaning up all related resources", "name", openclaw.Name)
			if err := r.deleteAllRelatedResources(ctx, openclaw); err != nil {
				return ctrl.Result{}, err
			}
			openclaw.ObjectMeta.Finalizers = removeOpenClawString(openclaw.ObjectMeta.Finalizers, openclawFinalizer)
			if err := r.Update(ctx, openclaw); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !containsOpenClawString(openclaw.ObjectMeta.Finalizers, openclawFinalizer) {
		openclaw.ObjectMeta.Finalizers = append(openclaw.ObjectMeta.Finalizers, openclawFinalizer)
		if err := r.Update(ctx, openclaw); err != nil {
			return ctrl.Result{}, err
		}
	}

	secret := r.secretForOpenclaw(openclaw)
	secretFound := &corev1.Secret{}
	err = r.Get(ctx, client.ObjectKeyFromObject(secret), secretFound)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Creating a new Secret.", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
			err = r.Create(ctx, secret)
			if err != nil {
				logger.Error(err, "Failed to create new Secret.", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
		logger.Error(err, "Failed to get Secret.")
		return ctrl.Result{}, err
	}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      openclaw.Name + "-gateway",
			Namespace: openclaw.Namespace,
		},
	}
	saFound := &corev1.ServiceAccount{}
	err = r.Get(ctx, client.ObjectKeyFromObject(sa), saFound)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Creating a new ServiceAccount.", "ServiceAccount.Namespace", sa.Namespace, "ServiceAccount.Name", sa.Name)
			err = r.Create(ctx, sa)
			if err != nil {
				logger.Error(err, "Failed to create new ServiceAccount.", "ServiceAccount.Namespace", sa.Namespace, "ServiceAccount.Name", sa.Name)
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
		logger.Error(err, "Failed to get ServiceAccount.")
		return ctrl.Result{}, err
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      openclaw.Name + "-gateway-job-reader",
			Namespace: openclaw.Namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      openclaw.Name + "-gateway",
				Namespace: openclaw.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     openclaw.Name + "-gateway-job-reader",
		},
	}
	roleBindingFound := &rbacv1.RoleBinding{}
	err = r.Get(ctx, client.ObjectKeyFromObject(roleBinding), roleBindingFound)
	if err != nil {
		if errors.IsNotFound(err) {
			role := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      openclaw.Name + "-gateway-job-reader",
					Namespace: openclaw.Namespace,
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"batch"},
						Resources: []string{"jobs"},
						Verbs:     []string{"get", "list", "watch"},
					},
				},
			}
			roleFound := &rbacv1.Role{}
			err = r.Get(ctx, client.ObjectKeyFromObject(role), roleFound)
			if err != nil {
				if errors.IsNotFound(err) {
					logger.Info("Creating a new Role.", "Role.Namespace", role.Namespace, "Role.Name", role.Name)
					err = r.Create(ctx, role)
					if err != nil {
						logger.Error(err, "Failed to create new Role.", "Role.Namespace", role.Namespace, "Role.Name", role.Name)
						return ctrl.Result{}, err
					}
				} else {
					logger.Error(err, "Failed to get Role.")
					return ctrl.Result{}, err
				}
			}

			logger.Info("Creating a new RoleBinding.", "RoleBinding.Namespace", roleBinding.Namespace, "RoleBinding.Name", roleBinding.Name)
			err = r.Create(ctx, roleBinding)
			if err != nil {
				logger.Error(err, "Failed to create new RoleBinding.", "RoleBinding.Namespace", roleBinding.Namespace, "RoleBinding.Name", roleBinding.Name)
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
		logger.Error(err, "Failed to get RoleBinding.")
		return ctrl.Result{}, err
	}

	pvc := r.pvcForOpenclaw(openclaw)
	pvcFound := &corev1.PersistentVolumeClaim{}
	err = r.Get(ctx, client.ObjectKeyFromObject(pvc), pvcFound)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Creating a new PVC.", "PVC.Namespace", pvc.Namespace, "PVC.Name", pvc.Name)
			err = r.Create(ctx, pvc)
			if err != nil {
				logger.Error(err, "Failed to create new PVC.", "PVC.Namespace", pvc.Namespace, "PVC.Name", pvc.Name)
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
		logger.Error(err, "Failed to get PVC.")
		return ctrl.Result{}, err
	}

	job := r.jobForOpenclaw(openclaw)
	jobFound := &batchv1.Job{}
	err = r.Get(ctx, client.ObjectKeyFromObject(job), jobFound)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Creating a new Job.", "Job.Namespace", job.Namespace, "Job.Name", job.Name)
			err = r.Create(ctx, job)
			if err != nil {
				logger.Error(err, "Failed to create new Job.", "Job.Namespace", job.Namespace, "Job.Name", job.Name)
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
		logger.Error(err, "Failed to get Job.")
		return ctrl.Result{}, err
	}

	sts := r.statefulSetForOpenclaw(openclaw)
	stsFound := &appsv1.StatefulSet{}
	err = r.Get(ctx, client.ObjectKeyFromObject(sts), stsFound)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Creating a new StatefulSet.", "StatefulSet.Namespace", sts.Namespace, "StatefulSet.Name", sts.Name)
			err = r.Create(ctx, sts)
			if err != nil {
				logger.Error(err, "Failed to create new StatefulSet.", "StatefulSet.Namespace", sts.Namespace, "StatefulSet.Name", sts.Name)
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
		logger.Error(err, "Failed to get StatefulSet.")
		return ctrl.Result{}, err
	}

	replicas := openclaw.Spec.Replicas
	if replicas == nil {
		defaultReplicas := int32(1)
		replicas = &defaultReplicas
	}
	if *stsFound.Spec.Replicas != *replicas {
		stsFound.Spec.Replicas = replicas
		err = r.Update(ctx, stsFound)
		if err != nil {
			logger.Error(err, "Failed to update StatefulSet.", "StatefulSet.Namespace", stsFound.Namespace, "StatefulSet.Name", stsFound.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	svc := r.serviceForOpenclaw(openclaw)
	svcFound := &corev1.Service{}
	err = r.Get(ctx, client.ObjectKeyFromObject(svc), svcFound)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Creating a new Service.", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			err = r.Create(ctx, svc)
			if err != nil {
				logger.Error(err, "Failed to create new Service.", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
		logger.Error(err, "Failed to get Service.")
		return ctrl.Result{}, err
	}

	if openclaw.Spec.Privacy != nil && *openclaw.Spec.Privacy {
		conchProxySecret := r.secretForConchProxy(openclaw)
		conchProxySecretFound := &corev1.Secret{}
		err = r.Get(ctx, client.ObjectKeyFromObject(conchProxySecret), conchProxySecretFound)
		if err != nil {
			if errors.IsNotFound(err) {
				logger.Info("Creating a new conch-proxy Secret.", "Secret.Namespace", conchProxySecret.Namespace, "Secret.Name", conchProxySecret.Name)
				err = r.Create(ctx, conchProxySecret)
				if err != nil {
					logger.Error(err, "Failed to create new conch-proxy Secret.", "Secret.Namespace", conchProxySecret.Namespace, "Secret.Name", conchProxySecret.Name)
					return ctrl.Result{}, err
				}
				return ctrl.Result{Requeue: true}, nil
			}
			logger.Error(err, "Failed to get conch-proxy Secret.")
			return ctrl.Result{}, err
		}

		conchProxyDeployment := r.deploymentForConchProxy(openclaw)
		conchProxyDeploymentFound := &appsv1.Deployment{}
		err = r.Get(ctx, client.ObjectKeyFromObject(conchProxyDeployment), conchProxyDeploymentFound)
		if err != nil {
			if errors.IsNotFound(err) {
				logger.Info("Creating a new conch-proxy Deployment.", "Deployment.Namespace", conchProxyDeployment.Namespace, "Deployment.Name", conchProxyDeployment.Name)
				err = r.Create(ctx, conchProxyDeployment)
				if err != nil {
					logger.Error(err, "Failed to create new conch-proxy Deployment.", "Deployment.Namespace", conchProxyDeployment.Namespace, "Deployment.Name", conchProxyDeployment.Name)
					return ctrl.Result{}, err
				}
				return ctrl.Result{Requeue: true}, nil
			}
			logger.Error(err, "Failed to get conch-proxy Deployment.")
			return ctrl.Result{}, err
		}

		conchProxyService := r.serviceForConchProxy(openclaw)
		conchProxyServiceFound := &corev1.Service{}
		err = r.Get(ctx, client.ObjectKeyFromObject(conchProxyService), conchProxyServiceFound)
		if err != nil {
			if errors.IsNotFound(err) {
				logger.Info("Creating a new conch-proxy Service.", "Service.Namespace", conchProxyService.Namespace, "Service.Name", conchProxyService.Name)
				err = r.Create(ctx, conchProxyService)
				if err != nil {
					logger.Error(err, "Failed to create new conch-proxy Service.", "Service.Namespace", conchProxyService.Namespace, "Service.Name", conchProxyService.Name)
					return ctrl.Result{}, err
				}
				return ctrl.Result{Requeue: true}, nil
			}
			logger.Error(err, "Failed to get conch-proxy Service.")
			return ctrl.Result{}, err
		}
	}

	err = r.createDefaultResources(ctx, openclaw)
	if err != nil {
		logger.Error(err, "Failed to create default resources.")
		return ctrl.Result{}, err
	}

	openclaw.Status.Replicas = *stsFound.Spec.Replicas
	openclaw.Status.ReadyReplicas = stsFound.Status.ReadyReplicas
	if stsFound.Status.ReadyReplicas == *replicas {
		openclaw.Status.Phase = string(openclawiov1.OpenclawPhaseRunning)
	} else {
		openclaw.Status.Phase = string(openclawiov1.OpenclawPhasePending)
	}

	err = r.Status().Update(ctx, openclaw)
	if err != nil && !errors.IsNotFound(err) {
		logger.Error(err, "Failed to update Openclaw status.")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *OpenclawReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openclawiov1.Openclaw{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&batchv1.Job{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

func (r *OpenclawReconciler) secretForOpenclaw(m *openclawiov1.Openclaw) *corev1.Secret {
	gatewayToken := m.Spec.GatewayToken
	if gatewayToken == "" {
		b := make([]byte, 32)
		rand.Read(b)
		gatewayToken = hex.EncodeToString(b)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name + "-secret",
			Namespace: m.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"OPENCLAW_GATEWAY_TOKEN": gatewayToken,
			"CUSTOM_API_KEY":         m.Spec.CustomAPIKey,
		},
	}
	ctrl.SetControllerReference(m, secret, r.Scheme)
	return secret
}

func (r *OpenclawReconciler) secretForConchProxy(m *openclawiov1.Openclaw) *corev1.Secret {
	b := make([]byte, 32)
	rand.Read(b)
	sensitiveToken := hex.EncodeToString(b)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name + "-conch-proxy-secrets",
			Namespace: m.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"upstream-api-key": m.Spec.CustomAPIKey,
			"slm-api-key":      m.Spec.SLMAPIKey,
			"sensitive-token":  sensitiveToken,
		},
	}
	ctrl.SetControllerReference(m, secret, r.Scheme)
	return secret
}

func (r *OpenclawReconciler) pvcForOpenclaw(m *openclawiov1.Openclaw) *corev1.PersistentVolumeClaim {
	accessModes := []corev1.PersistentVolumeAccessMode{}
	for _, mode := range m.Spec.Storage.AccessModes {
		accessModes = append(accessModes, corev1.PersistentVolumeAccessMode(mode))
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name + "-data",
			Namespace: m.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: accessModes,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(m.Spec.Storage.Storage),
				},
			},
		},
	}
	ctrl.SetControllerReference(m, pvc, r.Scheme)
	return pvc
}

func (r *OpenclawReconciler) jobForOpenclaw(m *openclawiov1.Openclaw) *batchv1.Job {
	isPrivacy := "false"
	if m.Spec.Privacy != nil && *m.Spec.Privacy {
		isPrivacy = "true"
	}

	b := make([]byte, 32)
	rand.Read(b)
	sensitiveToken := hex.EncodeToString(b)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name + "-onboard",
			Namespace: m.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: func(i int32) *int32 { return &i }(2),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": m.Name + "-onboard"},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: func(i int64) *int64 { return &i }(1000),
					},
					Containers: []corev1.Container{
						{
							Name:            "onboard",
							Image:           m.Spec.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{
									Name:  "HOME",
									Value: "/root",
								},
								{
									Name: "OPENCLAW_GATEWAY_TOKEN",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: m.Name + "-secret"},
											Key:                  "OPENCLAW_GATEWAY_TOKEN",
										},
									},
								},
								{
									Name: "CUSTOM_API_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: m.Name + "-secret"},
											Key:                  "CUSTOM_API_KEY",
										},
									},
								},
							},
							Command: []string{"/bin/sh", "-lc"},
							Args: []string{
								`set -eu
mkdir -p /root/.openclaw
mkdir -p /root/.openclaw/workspace
mkdir -p /root/.openclaw/skills

if [ -f /root/.openclaw/openclaw.json ]; then
  echo "openclaw.json already exists, skip onboarding"
  exit 0
fi

CUSTOM_BASE_URL="` + m.Spec.CustomBaseURL + `"
if [ "` + isPrivacy + `" = "true" ]; then
  CUSTOM_BASE_URL="http://` + m.Name + `-conch-proxy.` + m.Namespace + `:80"
  echo "Using conch-proxy as base URL: $CUSTOM_BASE_URL"
fi

openclaw onboard --non-interactive \
  --mode local \
  --auth-choice custom-api-key \
  --custom-base-url "$CUSTOM_BASE_URL" \
  --custom-model-id "` + m.Spec.CustomModelID + `" \
  --custom-provider-id "` + m.Spec.CustomProviderID + `" \
  --custom-compatibility ` + m.Spec.CustomCompatibility + ` \
  --secret-input-mode ref \
  --gateway-auth token \
  --gateway-token-ref-env OPENCLAW_GATEWAY_TOKEN \
  --gateway-port ` + fmt.Sprintf("%d", m.Spec.GatewayPort) + ` \
  --gateway-bind ` + m.Spec.GatewayBind + ` \
  --accept-risk \
  --skip-health

if [ "` + isPrivacy + `" = "true" ]; then
	echo "Installing hermitcrab plugin..."
    openclaw plugins install /plugin/hermitcrab/
	echo "Enabling hermitcrab plugin..."
	openclaw plugins enable hermitcrab
	echo "Initializing hermitcrab plugin config..."
	openclaw config set 'plugins.entries.hermitcrab' '{
        "enabled": true,
        "config": {
          "token": {
            "secret": "` + sensitiveToken + `"
          },
          "llm": {
            "baseUrl": "` + m.Spec.SLMAPIURL + `",
            "apiKey": "` + m.Spec.SLMAPIKey + `",
            "model": "` + m.Spec.SLMModelID + `"
          },
          "profile": {
            "enabled": true,
            "trustThreshold": 0.8
          },
          "riskThresholds": {
            "autoAllow": "LOW",
            "humanReview": ["MEDIUM", "HIGH"]
          }
        }
      }' --json
	echo "Installed hermitcrab plugin"
fi

echo "Openclaw Instance Initialized Successfully"
`,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/root/.openclaw",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: m.Name + "-data",
								},
							},
						},
					},
				},
			},
		},
	}
	ctrl.SetControllerReference(m, job, r.Scheme)
	return job
}

func (r *OpenclawReconciler) statefulSetForOpenclaw(m *openclawiov1.Openclaw) *appsv1.StatefulSet {
	replicas := m.Spec.Replicas
	if replicas == nil {
		defaultReplicas := int32(1)
		replicas = &defaultReplicas
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name + "-gateway",
			Namespace: m.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: m.Name,
			Replicas:    replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": m.Name + "-gateway"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": m.Name + "-gateway"},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: m.Name + "-gateway",
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: func(i int64) *int64 { return &i }(1000),
					},
					InitContainers: []corev1.Container{
						{
							Name:            "wait-for-onboard",
							Image:           m.Spec.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"/bin/sh", "-lc"},
							Args: []string{
								`set -eu
echo "Waiting for onboard job to complete..."
JOB_NAME="` + m.Name + `-onboard"
NAMESPACE="` + m.Namespace + `"
MAX_WAIT_TIME=600
ELAPSED=0

# Get token and CA cert for in-cluster access
TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
CACERT=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
APISERVER=https://kubernetes.default.svc

while [ $ELAPSED -lt $MAX_WAIT_TIME ]; do
  # Check if job succeeded via Kubernetes API
  RESPONSE=$(curl -s -k -H "Authorization: Bearer $TOKEN" "$APISERVER/apis/batch/v1/namespaces/$NAMESPACE/jobs/$JOB_NAME" 2>/dev/null || echo "{}")
  SUCCEEDED=$(echo "$RESPONSE" | grep -o '"succeeded":[^,}]*' | grep -o '[0-9]*' || echo "0")
  FAILED=$(echo "$RESPONSE" | grep -o '"failed":[^,}]*' | grep -o '[0-9]*' || echo "0")
  
  if [ "$SUCCEEDED" != "" ] && [ "$SUCCEEDED" -ge 1 ]; then
    echo "Onboard job completed successfully!"
    exit 0
  fi
  
  if [ "$FAILED" != "" ] && [ "$FAILED" -ge 1 ]; then
    echo "Onboard job failed!"
    exit 1
  fi
  
  echo "Onboard job not completed yet, waiting..."
  sleep 5
  ELAPSED=$((ELAPSED + 5))
done

echo "Timeout waiting for onboard job to complete!"
exit 1`,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/root/.openclaw",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "gateway",
							Image:           m.Spec.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: m.Spec.GatewayPort,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "HOME",
									Value: "/root",
								},
								{
									Name: "OPENCLAW_GATEWAY_TOKEN",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: m.Name + "-secret"},
											Key:                  "OPENCLAW_GATEWAY_TOKEN",
										},
									},
								},
								{
									Name: "CUSTOM_API_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: m.Name + "-secret"},
											Key:                  "CUSTOM_API_KEY",
										},
									},
								},
							},
							Command: []string{"/bin/sh", "-lc"},
							Args: []string{
								`set -eu
test -f /root/.openclaw/openclaw.json
exec openclaw gateway`,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/readyz",
										Port: intstr.FromInt(int(m.Spec.GatewayPort)),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
								TimeoutSeconds:      3,
								FailureThreshold:    12,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt(int(m.Spec.GatewayPort)),
									},
								},
								InitialDelaySeconds: 20,
								PeriodSeconds:       20,
								TimeoutSeconds:      3,
								FailureThreshold:    6,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    getResourceQuantity(m.Spec.Resources.Requests.CPU),
									corev1.ResourceMemory: getResourceQuantity(m.Spec.Resources.Requests.Memory),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    getResourceQuantity(m.Spec.Resources.Limits.CPU),
									corev1.ResourceMemory: getResourceQuantity(m.Spec.Resources.Limits.Memory),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/root/.openclaw",
								},
							},
						},
						{
							Name:            "config-reloader",
							Image:           "10.29.15.63/ghcr.m.daocloud.io/openclaw/config-reloader:v0.9",
							ImagePullPolicy: corev1.PullIfNotPresent,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/root/.openclaw",
								},
								{
									Name:      "allowed-origins",
									MountPath: "/etc/openclaw-config/allowed-origins",
								},
								{
									Name:      "channels",
									MountPath: "/etc/openclaw-config/channels",
								},
								{
									Name:      "agents",
									MountPath: "/etc/openclaw-config/agents",
								},
								{
									Name:      "models",
									MountPath: "/etc/openclaw-config/models",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: m.Name + "-data",
								},
							},
						},
						{
							Name: "allowed-origins",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: m.Name + "-allowed-origins",
									},
									Optional: func() *bool { b := true; return &b }(),
								},
							},
						},
						{
							Name: "channels",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: m.Name + "-channels",
									},
									Optional: func() *bool { b := true; return &b }(),
								},
							},
						},
						{
							Name: "agents",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: m.Name + "-agents",
									},
									Optional: func() *bool { b := true; return &b }(),
								},
							},
						},
						{
							Name: "models",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: m.Name + "-models",
									},
									Optional: func() *bool { b := true; return &b }(),
								},
							},
						},
					},
				},
			},
		},
	}
	ctrl.SetControllerReference(m, sts, r.Scheme)
	return sts
}

func (r *OpenclawReconciler) deploymentForConchProxy(m *openclawiov1.Openclaw) *appsv1.Deployment {
	replicas := int32(1)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name + "-conch-proxy",
			Namespace: m.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": m.Name + "-conch-proxy"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": m.Name + "-conch-proxy"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "conch-proxy",
							Image:           "10.29.15.63/ghcr.m.daocloud.io/openclaw/conch-proxy:v0.1.0",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8080,
									Name:          "http",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "PORT",
									Value: "8080",
								},
								{
									Name:  "UPSTREAM_API_URL",
									Value: m.Spec.CustomBaseURL,
								},
								{
									Name: "UPSTREAM_API_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: m.Name + "-conch-proxy-secrets"},
											Key:                  "upstream-api-key",
										},
									},
								},
								{
									Name:  "SLM_API_URL",
									Value: m.Spec.SLMAPIURL,
								},
								{
									Name:  "SLM_MODEL",
									Value: m.Spec.SLMModelID,
								},
								{
									Name: "SLM_API_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: m.Name + "-conch-proxy-secrets"},
											Key:                  "slm-api-key",
										},
									},
								},
								{
									Name:  "PROXY_URL",
									Value: "",
								},
								{
									Name:  "ENABLE_INTERCEPTOR",
									Value: "true",
								},
								{
									Name: "SENSITIVE_TOKEN",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: m.Name + "-conch-proxy-secrets"},
											Key:                  "sensitive-token",
										},
									},
								},
								{
									Name:  "REDIS_ADDR",
									Value: m.Spec.Redis.Address,
								},
								{
									Name:  "REDIS_PASSWORD",
									Value: m.Spec.Redis.Password,
								},
								{
									Name:  "REDIS_DB",
									Value: strconv.Itoa(m.Spec.Redis.DB),
								},
								{
									Name:  "UNLOCK_TTL_MINUTES",
									Value: "60",
								},
								{
									Name:  "ENABLE_TLS",
									Value: "false",
								},
								{
									Name:  "TLS_CERT_FILE",
									Value: "/app/server.crt",
								},
								{
									Name:  "TLS_KEY_FILE",
									Value: "/app/server.key",
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 15,
								PeriodSeconds:       20,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
						},
					},
				},
			},
		},
	}
	ctrl.SetControllerReference(m, deployment, r.Scheme)
	return deployment
}

func (r *OpenclawReconciler) serviceForConchProxy(m *openclawiov1.Openclaw) *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name + "-conch-proxy",
			Namespace: m.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"app": m.Name + "-conch-proxy"},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}
	ctrl.SetControllerReference(m, service, r.Scheme)
	return service
}

func (r *OpenclawReconciler) serviceForOpenclaw(m *openclawiov1.Openclaw) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": m.Name + "-gateway"},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       m.Spec.GatewayPort,
					TargetPort: intstr.FromInt(int(m.Spec.GatewayPort)),
				},
			},
			Type: getServiceType(m.Spec.ServiceType),
		},
	}
	ctrl.SetControllerReference(m, svc, r.Scheme)
	return svc
}

func getResourceQuantity(s string) resource.Quantity {
	if s == "" {
		return resource.Quantity{}
	}
	return resource.MustParse(s)
}

func getServiceType(t string) corev1.ServiceType {
	switch t {
	case "NodePort":
		return corev1.ServiceTypeNodePort
	case "LoadBalancer":
		return corev1.ServiceTypeLoadBalancer
	default:
		return corev1.ServiceTypeClusterIP
	}
}

func (r *OpenclawReconciler) createDefaultResources(ctx context.Context, openclaw *openclawiov1.Openclaw) error {
	err := r.createDefaultAllowedOrigins(ctx, openclaw)
	if err != nil {
		return err
	}

	err = r.createDefaultAgentDefaults(ctx, openclaw)
	if err != nil {
		return err
	}

	err = r.createDefaultAgent(ctx, openclaw)
	if err != nil {
		return err
	}

	err = r.createDefaultModels(ctx, openclaw)
	if err != nil {
		return err
	}

	return nil
}

func (r *OpenclawReconciler) deleteAllRelatedResources(ctx context.Context, openclaw *openclawiov1.Openclaw) error {
	logger := log.FromContext(ctx)
	namespace := openclaw.Namespace
	openclawName := openclaw.Name

	logger.Info("Deleting all related resources for OpenClaw", "name", openclawName, "namespace", namespace)

	if err := r.deleteOpenClawModels(ctx, namespace, openclawName); err != nil {
		logger.Error(err, "Failed to delete OpenClawModels")
		return err
	}

	if err := r.deleteOpenClawAgents(ctx, namespace, openclawName); err != nil {
		logger.Error(err, "Failed to delete OpenClawAgents")
		return err
	}

	if err := r.deleteOpenClawAgentDefaults(ctx, namespace, openclawName); err != nil {
		logger.Error(err, "Failed to delete OpenClawAgentDefaults")
		return err
	}

	if err := r.deleteOpenClawAllowedOrigins(ctx, namespace, openclawName); err != nil {
		logger.Error(err, "Failed to delete OpenClawAllowedOrigins")
		return err
	}

	if openclaw.Spec.Privacy != nil && *openclaw.Spec.Privacy {
		if err := r.deleteConchProxyResources(ctx, namespace, openclawName); err != nil {
			logger.Error(err, "Failed to delete conch-proxy resources")
			return err
		}
	}

	logger.Info("All related resources deleted successfully", "name", openclawName)
	return nil
}

func (r *OpenclawReconciler) deleteConchProxyResources(ctx context.Context, namespace, openclawName string) error {
	// Delete conch-proxy Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      openclawName + "-conch-proxy",
			Namespace: namespace,
		},
	}
	if err := r.Delete(ctx, service); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	// Delete conch-proxy Deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      openclawName + "-conch-proxy",
			Namespace: namespace,
		},
	}
	if err := r.Delete(ctx, deployment); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	// Delete conch-proxy Secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      openclawName + "-conch-proxy-secrets",
			Namespace: namespace,
		},
	}
	if err := r.Delete(ctx, secret); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func (r *OpenclawReconciler) deleteOpenClawModels(ctx context.Context, namespace, openclawName string) error {
	modelsList := &openclawiov1.OpenClawModelsList{}
	err := r.List(ctx, modelsList, client.InNamespace(namespace))
	if err != nil {
		return err
	}

	for _, models := range modelsList.Items {
		if models.Spec.OpenclawRef.Name == openclawName {
			if err := r.Delete(ctx, &models); err != nil {
				if !errors.IsNotFound(err) {
					return err
				}
			}
		}
	}
	return nil
}

func (r *OpenclawReconciler) deleteOpenClawAgents(ctx context.Context, namespace, openclawName string) error {
	agentList := &openclawiov1.OpenClawAgentList{}
	err := r.List(ctx, agentList, client.InNamespace(namespace))
	if err != nil {
		return err
	}

	for _, agent := range agentList.Items {
		if agent.Spec.OpenclawRef.Name == openclawName {
			if err := r.Delete(ctx, &agent); err != nil {
				if !errors.IsNotFound(err) {
					return err
				}
			}
		}
	}
	return nil
}

func (r *OpenclawReconciler) deleteOpenClawAgentDefaults(ctx context.Context, namespace, openclawName string) error {
	defaultsList := &openclawiov1.OpenClawAgentDefaultsList{}
	err := r.List(ctx, defaultsList, client.InNamespace(namespace))
	if err != nil {
		return err
	}

	for _, defaults := range defaultsList.Items {
		if defaults.Spec.OpenclawRef.Name == openclawName {
			if err := r.Delete(ctx, &defaults); err != nil {
				if !errors.IsNotFound(err) {
					return err
				}
			}
		}
	}
	return nil
}

func (r *OpenclawReconciler) deleteOpenClawAllowedOrigins(ctx context.Context, namespace, openclawName string) error {
	originsList := &openclawiov1.OpenClawAllowedOriginList{}
	err := r.List(ctx, originsList, client.InNamespace(namespace))
	if err != nil {
		return err
	}

	for _, origin := range originsList.Items {
		if origin.Spec.OpenclawRef.Name == openclawName {
			if err := r.Delete(ctx, &origin); err != nil {
				if !errors.IsNotFound(err) {
					return err
				}
			}
		}
	}
	return nil
}

func containsOpenClawString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeOpenClawString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return
}

func (r *OpenclawReconciler) createDefaultModels(ctx context.Context, openclaw *openclawiov1.Openclaw) error {
	if openclaw.Spec.CustomProviderID == "" || openclaw.Spec.CustomModelID == "" {
		return nil
	}

	apikey := openclaw.Spec.CustomAPIKey
	baseURL := openclaw.Spec.CustomBaseURL

	if openclaw.Spec.Privacy != nil && *openclaw.Spec.Privacy {
		apikey = fmt.Sprintf("sk-%s-conch-proxy-apikey", openclaw.Name)
		baseURL = fmt.Sprintf("http://%s-conch-proxy.%s.svc.cluster.local:80/v1", openclaw.Name, openclaw.Namespace)
	}

	providers := []openclawiov1.Provider{
		{
			Name:    openclaw.Spec.CustomProviderID,
			API:     "openai-completions",
			APIKey:  apikey,
			BaseURL: baseURL,
			Models: []openclawiov1.Model{
				{
					ContextWindow: 16000,
					Cost: openclawiov1.Cost{
						CacheRead:  0,
						CacheWrite: 0,
						Input:      0,
						Output:     0,
					},
					ID:        openclaw.Spec.CustomModelID,
					Input:     []string{"text"},
					MaxTokens: 4096,
					Name:      openclaw.Spec.CustomModelID,
					Reasoning: false,
				},
			},
		},
	}

	models := &openclawiov1.OpenClawModels{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-models", openclaw.Name),
			Namespace: openclaw.Namespace,
		},
		Spec: openclawiov1.OpenClawModelsSpec{
			OpenclawRef: openclawiov1.OpenclawRef{
				Name: openclaw.Name,
			},
			Mode:      "merge",
			Providers: providers,
		},
	}

	err := ctrl.SetControllerReference(openclaw, models, r.Scheme)
	if err != nil {
		return err
	}

	found := &openclawiov1.OpenClawModels{}
	err = r.Get(ctx, client.ObjectKeyFromObject(models), found)
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.Create(ctx, models)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

func (r *OpenclawReconciler) createDefaultAllowedOrigins(ctx context.Context, openclaw *openclawiov1.Openclaw) error {
	origins := []string{"http://127.0.0.1:18789", "http://localhost:18789"}

	// 如果 serviceType 为 NodePort，添加 master 节点 IP + NodePort
	if openclaw.Spec.ServiceType == "NodePort" {
		// 获取服务信息
		svc := &corev1.Service{}
		svcKey := client.ObjectKey{Name: openclaw.Name, Namespace: openclaw.Namespace}
		err := r.Get(ctx, svcKey, svc)
		if err == nil && svc.Spec.Type == corev1.ServiceTypeNodePort {
			// 获取 NodePort
			var nodePort int32
			for _, port := range svc.Spec.Ports {
				if port.Name == "http" {
					nodePort = port.NodePort
					break
				}
			}

			if nodePort > 0 {
				// 获取 master 节点
				nodes := &corev1.NodeList{}
				err := r.List(ctx, nodes)
				if err == nil {
					for _, node := range nodes.Items {
						// 检查是否为 master 节点
						if isMasterNode(node) {
							// 获取节点 IP
							var nodeIP string
							for _, addr := range node.Status.Addresses {
								if addr.Type == corev1.NodeInternalIP {
									nodeIP = addr.Address
									break
								}
							}
							if nodeIP != "" {
								// 添加 NodePort origin
								nodePortOrigin := fmt.Sprintf("http://%s:%d", nodeIP, nodePort)
								origins = append(origins, nodePortOrigin)
								break
							}
						}
					}
				}
			}
		}
	}

	for i, origin := range origins {
		allowedOrigin := &openclawiov1.OpenClawAllowedOrigin{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-allowed-origin-%d", openclaw.Name, i),
				Namespace: openclaw.Namespace,
			},
			Spec: openclawiov1.OpenClawAllowedOriginSpec{
				OpenclawRef: openclawiov1.OpenclawRef{
					Name: openclaw.Name,
				},
				Origin:  origin,
				Enabled: true,
				UseHTTP: true,
			},
		}

		err := ctrl.SetControllerReference(openclaw, allowedOrigin, r.Scheme)
		if err != nil {
			return err
		}

		found := &openclawiov1.OpenClawAllowedOrigin{}
		err = r.Get(ctx, client.ObjectKeyFromObject(allowedOrigin), found)
		if err != nil {
			if errors.IsNotFound(err) {
				err = r.Create(ctx, allowedOrigin)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}

	return nil
}

func isMasterNode(node corev1.Node) bool {
	// 检查节点是否有 master 标签
	if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
		return true
	}
	// 检查节点是否有 control-plane 标签
	if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; ok {
		return true
	}
	return false
}

func (r *OpenclawReconciler) createDefaultAgentDefaults(ctx context.Context, openclaw *openclawiov1.Openclaw) error {
	model := fmt.Sprintf("%s/%s", openclaw.Spec.CustomProviderID, openclaw.Spec.CustomModelID)

	agentDefaults := &openclawiov1.OpenClawAgentDefaults{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-agent-defaults", openclaw.Name),
			Namespace: openclaw.Namespace,
		},
		Spec: openclawiov1.OpenClawAgentDefaultsSpec{
			OpenclawRef: openclawiov1.OpenclawRef{
				Name: openclaw.Name,
			},
			PrimaryModel: model,
			Workspace:    "~/.openclaw/workspace",
		},
	}

	err := ctrl.SetControllerReference(openclaw, agentDefaults, r.Scheme)
	if err != nil {
		return err
	}

	found := &openclawiov1.OpenClawAgentDefaults{}
	err = r.Get(ctx, client.ObjectKeyFromObject(agentDefaults), found)
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.Create(ctx, agentDefaults)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

func (r *OpenclawReconciler) createDefaultAgent(ctx context.Context, openclaw *openclawiov1.Openclaw) error {
	model := fmt.Sprintf("%s/%s", openclaw.Spec.CustomProviderID, openclaw.Spec.CustomModelID)

	agent := &openclawiov1.OpenClawAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-default-agent", openclaw.Name),
			Namespace: openclaw.Namespace,
		},
		Spec: openclawiov1.OpenClawAgentSpec{
			OpenclawRef: openclawiov1.OpenclawRef{
				Name: openclaw.Name,
			},
			ID:      "main",
			Name:    "main",
			Enabled: true,
			Default: true,
			Model:   model,
		},
	}

	err := ctrl.SetControllerReference(openclaw, agent, r.Scheme)
	if err != nil {
		return err
	}

	found := &openclawiov1.OpenClawAgent{}
	err = r.Get(ctx, client.ObjectKeyFromObject(agent), found)
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.Create(ctx, agent)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}
