package controllers

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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

	openclaw.Status.Replicas = *stsFound.Spec.Replicas
	openclaw.Status.ReadyReplicas = stsFound.Status.ReadyReplicas
	if stsFound.Status.ReadyReplicas == *replicas {
		openclaw.Status.Phase = string(openclawiov1.OpenclawPhaseRunning)
	} else {
		openclaw.Status.Phase = string(openclawiov1.OpenclawPhasePending)
	}

	err = r.Status().Update(ctx, openclaw)
	if err != nil {
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
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot:             func(b bool) *bool { return &b }(true),
								RunAsUser:                func(i int64) *int64 { return &i }(1000),
								AllowPrivilegeEscalation: func(b bool) *bool { return &b }(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "HOME",
									Value: "/home/node",
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
mkdir -p /home/node/.openclaw
mkdir -p /home/node/.openclaw/workspace
mkdir -p /home/node/.openclaw/skills

if [ -f /home/node/.openclaw/openclaw.json ]; then
  echo "openclaw.json already exists, skip onboarding"
  exit 0
fi

openclaw onboard --non-interactive \
  --mode local \
  --auth-choice custom-api-key \
  --custom-base-url "` + m.Spec.CustomBaseURL + `" \
  --custom-model-id "` + m.Spec.CustomModelID + `" \
  --custom-provider-id "` + m.Spec.CustomProviderID + `" \
  --custom-compatibility ` + m.Spec.CustomCompatibility + ` \
  --secret-input-mode ref \
  --gateway-auth token \
  --gateway-token-ref-env OPENCLAW_GATEWAY_TOKEN \
  --gateway-port ` + string(rune(m.Spec.GatewayPort)) + ` \
  --gateway-bind ` + m.Spec.GatewayBind + ` \
  --accept-risk

echo "===== generated config ====="
ls -la /home/node/.openclaw
test -f /home/node/.openclaw/openclaw.json`,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/home/node/.openclaw",
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
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: func(i int64) *int64 { return &i }(1000),
					},
					InitContainers: []corev1.Container{
						{
							Name:            "wait-for-onboard",
							Image:           m.Spec.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot:             func(b bool) *bool { return &b }(true),
								RunAsUser:                func(i int64) *int64 { return &i }(1000),
								AllowPrivilegeEscalation: func(b bool) *bool { return &b }(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							Command: []string{"/bin/sh", "-lc"},
							Args: []string{
								`set -eu
echo "Waiting for openclaw.json to be created by onboard job..."
while [ ! -f /home/node/.openclaw/openclaw.json ]; do
  echo "openclaw.json not found, waiting..."
  sleep 5
done
echo "openclaw.json found, onboard job completed!"`,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/home/node/.openclaw",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "gateway",
							Image:           m.Spec.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot:             func(b bool) *bool { return &b }(true),
								RunAsUser:                func(i int64) *int64 { return &i }(1000),
								AllowPrivilegeEscalation: func(b bool) *bool { return &b }(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: m.Spec.GatewayPort,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "HOME",
									Value: "/home/node",
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
test -f /home/node/.openclaw/openclaw.json
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
									MountPath: "/home/node/.openclaw",
								},
							},
						},
						{
							Name:            "config-reloader",
							Image:           "10.29.231.164/ghcr.m.daocloud.io/openclaw/config-reloader:v0.6",
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot:             func(b bool) *bool { return &b }(true),
								RunAsUser:                func(i int64) *int64 { return &i }(1000),
								AllowPrivilegeEscalation: func(b bool) *bool { return &b }(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/home/node/.openclaw",
								},
								{
									Name:      "allowed-origins",
									MountPath: "/etc/openclaw-config/allowed-origins",
								},
								{
									Name:      "channels",
									MountPath: "/etc/openclaw-config/channels",
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
					},
				},
			},
		},
	}
	ctrl.SetControllerReference(m, sts, r.Scheme)
	return sts
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
