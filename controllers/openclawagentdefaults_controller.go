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

type OpenClawAgentDefaultsReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *OpenClawAgentDefaultsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	agentDefaults := &openclawiov1.OpenClawAgentDefaults{}
	err := r.Get(ctx, req.NamespacedName, agentDefaults)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("OpenClawAgentDefaults resource not found. Reconciling all ConfigMaps to ensure consistency.")
			return r.reconcileAllConfigMaps(ctx, req.Namespace)
		}
		logger.Error(err, "Failed to get OpenClawAgentDefaults resource.")
		return ctrl.Result{}, err
	}

	openclawName := agentDefaults.Spec.OpenclawRef.Name
	if openclawName == "" {
		logger.Info("OpenClawAgentDefaults has no openclawRef, skipping.")
		return ctrl.Result{}, nil
	}

	return r.reconcileConfigMapForOpenclaw(ctx, req.Namespace, openclawName, agentDefaults)
}

func (r *OpenClawAgentDefaultsReconciler) reconcileConfigMapForOpenclaw(ctx context.Context, namespace, openclawName string, currentDefaults *openclawiov1.OpenClawAgentDefaults) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	agentDefaults := &openclawiov1.OpenClawAgentDefaultsList{}
	err := r.List(ctx, agentDefaults, client.InNamespace(namespace))
	if err != nil {
		logger.Error(err, "Failed to list OpenClawAgentDefaults resources.")
		return ctrl.Result{}, err
	}

	var defaultsConfig *DefaultsConfig
	for _, ad := range agentDefaults.Items {
		if ad.Spec.OpenclawRef.Name == openclawName {
			defaultsConfig = &DefaultsConfig{
				Model: ModelConfig{
					Primary: ad.Spec.PrimaryModel,
				},
				Workspace: ad.Spec.Workspace,
			}
			break
		}
	}

	if defaultsConfig == nil {
		logger.Info("No OpenClawAgentDefaults found for OpenClaw instance, deleting ConfigMap.")
		configMapName := openclawName + "-agents"
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: namespace,
			},
		}
		err = r.Delete(ctx, configMap)
		if err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "Failed to delete ConfigMap.", "ConfigMap", configMapName)
			if currentDefaults != nil {
				currentDefaults.Status.Phase = openclawiov1.OpenClawAgentDefaultsPhaseError
				currentDefaults.Status.Message = fmt.Sprintf("Failed to delete ConfigMap: %v", err)
				_ = r.Status().Update(ctx, currentDefaults)
			}
			return ctrl.Result{}, err
		}
		logger.Info("ConfigMap deleted successfully.", "ConfigMap", configMapName)
		return ctrl.Result{}, nil
	}

	agents := &openclawiov1.OpenClawAgentList{}
	err = r.List(ctx, agents, client.InNamespace(namespace))
	if err != nil {
		logger.Error(err, "Failed to list OpenClawAgent resources.")
		return ctrl.Result{}, err
	}

	agentsConfig := make(map[string]interface{})
	agentsConfig["defaults"] = defaultsConfig

	var agentList []AgentListItem
	for _, agent := range agents.Items {
		if agent.Spec.OpenclawRef.Name == openclawName && agent.Spec.Enabled {
			agentListItem := AgentListItem{
				ID:      agent.Spec.ID,
				Name:    agent.Spec.Name,
				Default: agent.Spec.Default,
			}
			agentListItem.Workspace = fmt.Sprintf("~/.openclaw/%s", agent.Spec.ID)
			if agent.Spec.Model != "" {
				agentListItem.Model = &ModelConfig{
					Primary: agent.Spec.Model,
				}
			}
			agentList = append(agentList, agentListItem)
		}
	}

	if len(agentList) > 0 {
		agentsConfig["list"] = agentList
	}

	config := AgentDefaultsConfig{
		Agents: agentsConfig,
	}

	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		logger.Error(err, "Failed to marshal config to JSON.")
		if currentDefaults != nil {
			currentDefaults.Status.Phase = openclawiov1.OpenClawAgentDefaultsPhaseError
			currentDefaults.Status.Message = fmt.Sprintf("Failed to marshal config: %v", err)
			_ = r.Status().Update(ctx, currentDefaults)
		}
		return ctrl.Result{}, err
	}

	configMapName := openclawName + "-agents"
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
			Labels: map[string]string{
				"app": openclawName,
			},
		},
		Data: map[string]string{
			"agents.json": string(configJSON),
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
				if currentDefaults != nil {
					currentDefaults.Status.Phase = openclawiov1.OpenClawAgentDefaultsPhaseError
					currentDefaults.Status.Message = fmt.Sprintf("Failed to create ConfigMap: %v", err)
					_ = r.Status().Update(ctx, currentDefaults)
				}
				return ctrl.Result{}, err
			}
		} else {
			logger.Error(err, "Failed to get ConfigMap.")
			if currentDefaults != nil {
				currentDefaults.Status.Phase = openclawiov1.OpenClawAgentDefaultsPhaseError
				currentDefaults.Status.Message = fmt.Sprintf("Failed to get ConfigMap: %v", err)
				_ = r.Status().Update(ctx, currentDefaults)
			}
			return ctrl.Result{}, err
		}
	} else {
		foundConfigMap.Data = configMap.Data
		logger.Info("Updating ConfigMap.", "ConfigMap.Namespace", configMap.Namespace, "ConfigMap.Name", configMap.Name)
		err = r.Update(ctx, foundConfigMap)
		if err != nil {
			logger.Error(err, "Failed to update ConfigMap.", "ConfigMap.Namespace", configMap.Namespace, "ConfigMap.Name", configMap.Name)
			if currentDefaults != nil {
				currentDefaults.Status.Phase = openclawiov1.OpenClawAgentDefaultsPhaseError
				currentDefaults.Status.Message = fmt.Sprintf("Failed to update ConfigMap: %v", err)
				_ = r.Status().Update(ctx, currentDefaults)
			}
			return ctrl.Result{}, err
		}
	}

	if currentDefaults != nil {
		currentDefaults.Status.Phase = openclawiov1.OpenClawAgentDefaultsPhaseReady
		currentDefaults.Status.Message = fmt.Sprintf("Successfully updated ConfigMap %s", configMapName)
		err = r.Status().Update(ctx, currentDefaults)
		if err != nil {
			logger.Error(err, "Failed to update OpenClawAgentDefaults status.")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *OpenClawAgentDefaultsReconciler) reconcileAllConfigMaps(ctx context.Context, namespace string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	agentDefaults := &openclawiov1.OpenClawAgentDefaultsList{}
	err := r.List(ctx, agentDefaults, client.InNamespace(namespace))
	if err != nil {
		logger.Error(err, "Failed to list OpenClawAgentDefaults resources.")
		return ctrl.Result{}, err
	}

	openclawToDefaults := make(map[string]*openclawiov1.OpenClawAgentDefaults)
	for _, ad := range agentDefaults.Items {
		if ad.Spec.OpenclawRef.Name != "" {
			openclawToDefaults[ad.Spec.OpenclawRef.Name] = &ad
		}
	}

	configMaps := &corev1.ConfigMapList{}
	err = r.List(ctx, configMaps, client.InNamespace(namespace))
	if err != nil {
		logger.Error(err, "Failed to list ConfigMaps.")
		return ctrl.Result{}, err
	}

	for _, cm := range configMaps.Items {
		if !strings.HasSuffix(cm.Name, "-agents") {
			continue
		}

		openclawName := strings.TrimSuffix(cm.Name, "-agents")
		defaults, exists := openclawToDefaults[openclawName]

		if !exists {
			logger.Info("No OpenClawAgentDefaults found for OpenClaw instance, deleting ConfigMap.", "ConfigMap", cm.Name)
			err = r.Delete(ctx, &cm)
			if err != nil && !errors.IsNotFound(err) {
				logger.Error(err, "Failed to delete ConfigMap.", "ConfigMap", cm.Name)
				continue
			}
			logger.Info("ConfigMap deleted successfully.", "ConfigMap", cm.Name)
			continue
		}

		_, err = r.reconcileConfigMapForOpenclaw(ctx, namespace, openclawName, defaults)
		if err != nil {
			logger.Error(err, "Failed to reconcile ConfigMap.", "ConfigMap", cm.Name)
			continue
		}
	}

	return ctrl.Result{}, nil
}

func (r *OpenClawAgentDefaultsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openclawiov1.OpenClawAgentDefaults{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
