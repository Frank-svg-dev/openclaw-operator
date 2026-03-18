package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openclawiov1 "github.com/Frank-svg-dev/openclaw-operator/api/v1"
)

const modelsFinalizer = "openclaw.io/models-finalizer"

type OpenClawModelsReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *OpenClawModelsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	models := &openclawiov1.OpenClawModels{}
	err := r.Get(ctx, req.NamespacedName, models)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("OpenClawModels resource not found. Ignoring since must be deleted.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get OpenClawModels resource.")
		return ctrl.Result{}, err
	}

	if models.ObjectMeta.DeletionTimestamp != nil {
		if containsString(models.ObjectMeta.Finalizers, modelsFinalizer) {
			logger.Info("Deleting OpenClawModels, cleaning up ConfigMap", "name", models.Name)
			if err := r.deleteConfigMap(ctx, models); err != nil {
				return ctrl.Result{}, err
			}
			models.ObjectMeta.Finalizers = removeString(models.ObjectMeta.Finalizers, modelsFinalizer)
			if err := r.Update(ctx, models); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !containsString(models.ObjectMeta.Finalizers, modelsFinalizer) {
		models.ObjectMeta.Finalizers = append(models.ObjectMeta.Finalizers, modelsFinalizer)
		if err := r.Update(ctx, models); err != nil {
			return ctrl.Result{}, err
		}
	}

	openclawName := models.Spec.OpenclawRef.Name
	if openclawName == "" {
		logger.Info("OpenClawModels has no openclawRef, skipping.")
		return ctrl.Result{}, nil
	}

	return r.reconcileConfigMapForOpenclaw(ctx, req.Namespace, openclawName, models)
}

func (r *OpenClawModelsReconciler) deleteConfigMap(ctx context.Context, models *openclawiov1.OpenClawModels) error {
	logger := log.FromContext(ctx)
	namespace := models.Namespace
	openclawName := models.Spec.OpenclawRef.Name
	if openclawName == "" {
		return nil
	}

	configMapName := openclawName + "-models"
	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Name: configMapName, Namespace: namespace}, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("ConfigMap already deleted", "ConfigMap", configMapName)
			return nil
		}
		logger.Error(err, "Failed to get ConfigMap for deletion", "ConfigMap", configMapName)
		return err
	}

	err = r.Delete(ctx, configMap)
	if err != nil {
		logger.Error(err, "Failed to delete ConfigMap", "ConfigMap", configMapName)
		return err
	}
	logger.Info("ConfigMap deleted successfully", "ConfigMap", configMapName)
	return nil
}

func (r *OpenClawModelsReconciler) reconcileConfigMapForOpenclaw(ctx context.Context, namespace, openclawName string, models *openclawiov1.OpenClawModels) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	configMapName := openclawName + "-models"
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
		},
	}

	existingConfigMap := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Name: configMapName, Namespace: namespace}, existingConfigMap)
	configMapExists := err == nil
	notFound := errors.IsNotFound(err)

	if !configMapExists && !notFound {
		logger.Error(err, "Failed to get ConfigMap.")
		models.Status.Phase = openclawiov1.OpenClawModelsPhaseError
		models.Status.Message = fmt.Sprintf("Failed to get ConfigMap: %v", err)
		_ = r.Status().Update(ctx, models)
		return ctrl.Result{}, err
	}

	if !configMapExists && notFound {
		logger.Info("ConfigMap does not exist, will create it.", "ConfigMap", configMapName)
	}

	modelsConfig := map[string]interface{}{
		"mode":      models.Spec.Mode,
		"providers": make(map[string]interface{}),
	}

	for _, provider := range models.Spec.Providers {
		providerConfig := map[string]interface{}{
			"api":     provider.API,
			"apiKey":  provider.APIKey,
			"baseUrl": provider.BaseURL,
			"models":  make([]interface{}, 0),
		}

		for _, model := range provider.Models {
			modelConfig := map[string]interface{}{
				"contextWindow": model.ContextWindow,
				"cost": map[string]interface{}{
					"cacheRead":  model.Cost.CacheRead,
					"cacheWrite": model.Cost.CacheWrite,
					"input":      model.Cost.Input,
					"output":     model.Cost.Output,
				},
				"id":        model.ID,
				"input":     model.Input,
				"maxTokens": model.MaxTokens,
				"name":      model.Name,
				"reasoning": model.Reasoning,
			}
			providerConfig["models"] = append(providerConfig["models"].([]interface{}), modelConfig)
		}

		modelsConfig["providers"].(map[string]interface{})[provider.Name] = providerConfig
	}

	finalConfig := map[string]interface{}{
		"models": modelsConfig,
	}

	configJSON, err := json.MarshalIndent(finalConfig, "", "  ")
	if err != nil {
		logger.Error(err, "Failed to marshal models config to JSON.")
		models.Status.Phase = openclawiov1.OpenClawModelsPhaseError
		models.Status.Message = fmt.Sprintf("Failed to marshal config: %v", err)
		_ = r.Status().Update(ctx, models)
		return ctrl.Result{}, err
	}

	if !configMapExists {
		configMap.Data = map[string]string{
			"models.json": string(configJSON),
		}
		err = r.Create(ctx, configMap)
		if err != nil {
			logger.Error(err, "Failed to create ConfigMap.", "ConfigMap", configMapName)
			models.Status.Phase = openclawiov1.OpenClawModelsPhaseError
			models.Status.Message = fmt.Sprintf("Failed to create ConfigMap: %v", err)
			_ = r.Status().Update(ctx, models)
			return ctrl.Result{}, err
		}
		logger.Info("ConfigMap created successfully.", "ConfigMap", configMapName)
	} else {
		existingConfigMap.Data = map[string]string{
			"models.json": string(configJSON),
		}
		err = r.Update(ctx, existingConfigMap)
		if err != nil {
			logger.Error(err, "Failed to update ConfigMap.", "ConfigMap", configMapName)
			models.Status.Phase = openclawiov1.OpenClawModelsPhaseError
			models.Status.Message = fmt.Sprintf("Failed to update ConfigMap: %v", err)
			_ = r.Status().Update(ctx, models)
			return ctrl.Result{}, err
		}
		logger.Info("ConfigMap updated successfully.", "ConfigMap", configMapName)
	}

	models.Status.Phase = openclawiov1.OpenClawModelsPhaseReady
	models.Status.Message = "ConfigMap synced successfully"
	_ = r.Status().Update(ctx, models)

	return ctrl.Result{}, nil
}

func (r *OpenClawModelsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openclawiov1.OpenClawModels{}).
		Complete(r)
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return
}
