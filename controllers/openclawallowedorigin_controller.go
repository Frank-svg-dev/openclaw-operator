package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openclawiov1 "github.com/Frank-svg-dev/openclaw-operator/api/v1"
)

type AllowedOriginConfig struct {
	Gateway struct {
		ControlUI struct {
			AllowedOrigins []string `json:"allowedOrigins"`
		} `json:"controlUi"`
	} `json:"gateway"`
}

type OpenClawAllowedOriginReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *OpenClawAllowedOriginReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	origin := &openclawiov1.OpenClawAllowedOrigin{}
	err := r.Get(ctx, req.NamespacedName, origin)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("OpenClawAllowedOrigin resource not found. Reconciling all ConfigMaps to ensure consistency.")
			return r.reconcileAllConfigMaps(ctx, req.Namespace)
		}
		logger.Error(err, "Failed to get OpenClawAllowedOrigin resource.")
		return ctrl.Result{}, err
	}

	openclawName := origin.Spec.OpenclawRef.Name
	if openclawName == "" {
		logger.Info("OpenClawAllowedOrigin has no openclawRef, skipping.")
		return ctrl.Result{}, nil
	}

	return r.reconcileConfigMapForOpenclaw(ctx, req.Namespace, openclawName)
}

func (r *OpenClawAllowedOriginReconciler) reconcileConfigMapForOpenclaw(ctx context.Context, namespace, openclawName string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	origins := &openclawiov1.OpenClawAllowedOriginList{}
	err := r.List(ctx, origins, client.InNamespace(namespace))
	if err != nil {
		logger.Error(err, "Failed to list OpenClawAllowedOrigin resources.")
		return ctrl.Result{}, err
	}

	var allowedOrigins []string
	var currentOrigin *openclawiov1.OpenClawAllowedOrigin
	for _, o := range origins.Items {
		if o.Spec.OpenclawRef.Name == openclawName && o.Spec.Enabled {
			allowedOrigins = append(allowedOrigins, o.Spec.Origin)
			currentOrigin = &o
		}
	}

	configMapName := openclawName + "-allowed-origins"

	if len(allowedOrigins) == 0 {
		logger.Info("No enabled allowed origins found for OpenClaw instance, deleting ConfigMap.", "ConfigMap", configMapName)
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: namespace,
			},
		}
		err = r.Delete(ctx, configMap)
		if err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "Failed to delete ConfigMap.", "ConfigMap", configMapName)
			if currentOrigin != nil {
				currentOrigin.Status.Phase = string(openclawiov1.OpenClawAllowedOriginPhaseError)
				currentOrigin.Status.Message = fmt.Sprintf("Failed to delete ConfigMap: %v", err)
				_ = r.Status().Update(ctx, currentOrigin)
			}
			return ctrl.Result{}, err
		}
		logger.Info("ConfigMap deleted successfully.", "ConfigMap", configMapName)
		return ctrl.Result{}, nil
	}

	config := AllowedOriginConfig{}
	config.Gateway.ControlUI.AllowedOrigins = allowedOrigins

	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		logger.Error(err, "Failed to marshal config to JSON.")
		if currentOrigin != nil {
			currentOrigin.Status.Phase = string(openclawiov1.OpenClawAllowedOriginPhaseError)
			currentOrigin.Status.Message = fmt.Sprintf("Failed to marshal config: %v", err)
			_ = r.Status().Update(ctx, currentOrigin)
		}
		return ctrl.Result{}, err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
			Labels: map[string]string{
				"app": openclawName,
			},
		},
		Data: map[string]string{
			"allowed-origins.json": string(configJSON),
		},
	}

	foundConfigMap := &corev1.ConfigMap{}
	err = r.Get(ctx, client.ObjectKeyFromObject(configMap), foundConfigMap)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Creating a new ConfigMap.", "ConfigMap.Namespace", configMap.Namespace, "ConfigMap.Name", configMap.Name)
			err = r.Create(ctx, configMap)
			if err != nil {
				logger.Error(err, "Failed to create new ConfigMap.", "ConfigMap.Namespace", configMap.Namespace, "ConfigMap.Name", configMap.Name)
				if currentOrigin != nil {
					currentOrigin.Status.Phase = string(openclawiov1.OpenClawAllowedOriginPhaseError)
					currentOrigin.Status.Message = fmt.Sprintf("Failed to create ConfigMap: %v", err)
					_ = r.Status().Update(ctx, currentOrigin)
				}
				return ctrl.Result{}, err
			}
		} else {
			logger.Error(err, "Failed to get ConfigMap.")
			if currentOrigin != nil {
				currentOrigin.Status.Phase = string(openclawiov1.OpenClawAllowedOriginPhaseError)
				currentOrigin.Status.Message = fmt.Sprintf("Failed to get ConfigMap: %v", err)
				_ = r.Status().Update(ctx, currentOrigin)
			}
			return ctrl.Result{}, err
		}
	} else {
		foundConfigMap.Data = configMap.Data
		logger.Info("Updating ConfigMap.", "ConfigMap.Namespace", configMap.Namespace, "ConfigMap.Name", configMap.Name)
		err = r.Update(ctx, foundConfigMap)
		if err != nil {
			logger.Error(err, "Failed to update ConfigMap.", "ConfigMap.Namespace", configMap.Namespace, "ConfigMap.Name", configMap.Name)
			if currentOrigin != nil {
				currentOrigin.Status.Phase = string(openclawiov1.OpenClawAllowedOriginPhaseError)
				currentOrigin.Status.Message = fmt.Sprintf("Failed to update ConfigMap: %v", err)
				_ = r.Status().Update(ctx, currentOrigin)
			}
			return ctrl.Result{}, err
		}
	}

	if currentOrigin != nil {
		currentOrigin.Status.Phase = string(openclawiov1.OpenClawAllowedOriginPhaseReady)
		currentOrigin.Status.Message = fmt.Sprintf("Successfully updated ConfigMap %s with %d allowed origins", configMapName, len(allowedOrigins))
		err = r.Status().Update(ctx, currentOrigin)
		if err != nil {
			logger.Error(err, "Failed to update OpenClawAllowedOrigin status.")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *OpenClawAllowedOriginReconciler) reconcileAllConfigMaps(ctx context.Context, namespace string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	origins := &openclawiov1.OpenClawAllowedOriginList{}
	err := r.List(ctx, origins, client.InNamespace(namespace))
	if err != nil {
		logger.Error(err, "Failed to list OpenClawAllowedOrigin resources.")
		return ctrl.Result{}, err
	}

	openclawToOrigins := make(map[string][]string)
	for _, o := range origins.Items {
		if o.Spec.OpenclawRef.Name != "" && o.Spec.Enabled {
			openclawToOrigins[o.Spec.OpenclawRef.Name] = append(openclawToOrigins[o.Spec.OpenclawRef.Name], o.Spec.Origin)
		}
	}

	configMaps := &corev1.ConfigMapList{}
	err = r.List(ctx, configMaps, client.InNamespace(namespace))
	if err != nil {
		logger.Error(err, "Failed to list ConfigMaps.")
		return ctrl.Result{}, err
	}

	for _, cm := range configMaps.Items {
		if !strings.HasSuffix(cm.Name, "-allowed-origins") {
			continue
		}

		openclawName := strings.TrimSuffix(cm.Name, "-allowed-origins")
		allowedOrigins, exists := openclawToOrigins[openclawName]

		if !exists || len(allowedOrigins) == 0 {
			logger.Info("No enabled allowed origins found for OpenClaw instance, deleting ConfigMap.", "ConfigMap", cm.Name)
			err = r.Delete(ctx, &cm)
			if err != nil && !errors.IsNotFound(err) {
				logger.Error(err, "Failed to delete ConfigMap.", "ConfigMap", cm.Name)
				continue
			}
			logger.Info("ConfigMap deleted successfully.", "ConfigMap", cm.Name)
			continue
		}

		config := AllowedOriginConfig{}
		config.Gateway.ControlUI.AllowedOrigins = allowedOrigins

		configJSON, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			logger.Error(err, "Failed to marshal config to JSON.")
			continue
		}

		cm.Data = map[string]string{
			"allowed-origins.json": string(configJSON),
		}

		logger.Info("Updating ConfigMap.", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
		err = r.Update(ctx, &cm)
		if err != nil {
			logger.Error(err, "Failed to update ConfigMap.", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
			continue
		}
	}

	return ctrl.Result{}, nil
}

func (r *OpenClawAllowedOriginReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openclawiov1.OpenClawAllowedOrigin{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
