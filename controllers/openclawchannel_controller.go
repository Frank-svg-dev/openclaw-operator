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
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openclawiov1 "github.com/Frank-svg-dev/openclaw-operator/api/v1"
)

type ChannelConfig struct {
	Channels map[string]ChannelTypeConfig `json:"channels"`
}

type ChannelTypeConfig struct {
	Enabled        bool                                `json:"enabled"`
	AppId          string                              `json:"appId"`
	AppSecret      string                              `json:"appSecret"`
	RequireMention string                              `json:"requireMention,omitempty"`
	GroupPolicy    string                              `json:"groupPolicy,omitempty"`
	Groups         map[string]openclawiov1.GroupConfig `json:"groups,omitempty"`
}

type OpenClawChannelReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *OpenClawChannelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	channel := &openclawiov1.OpenClawChannel{}
	err := r.Get(ctx, req.NamespacedName, channel)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("OpenClawChannel resource not found. Reconciling all ConfigMaps to ensure consistency.")
			return r.reconcileAllConfigMaps(ctx, req.Namespace)
		}
		logger.Error(err, "Failed to get OpenClawChannel resource.")
		return ctrl.Result{}, err
	}

	openclawName := channel.Spec.OpenclawRef.Name
	if openclawName == "" {
		logger.Info("OpenClawChannel has no openclawRef, skipping.")
		return ctrl.Result{}, nil
	}

	return r.reconcileConfigMapForOpenclaw(ctx, req.Namespace, openclawName, channel)
}

func (r *OpenClawChannelReconciler) reconcileConfigMapForOpenclaw(ctx context.Context, namespace, openclawName string, currentChannel *openclawiov1.OpenClawChannel) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	channels := &openclawiov1.OpenClawChannelList{}
	err := r.List(ctx, channels, client.InNamespace(namespace))
	if err != nil {
		logger.Error(err, "Failed to list OpenClawChannel resources.")
		return ctrl.Result{}, err
	}

	config := ChannelConfig{
		Channels: make(map[string]ChannelTypeConfig),
	}

	for _, ch := range channels.Items {
		if ch.Spec.OpenclawRef.Name != openclawName || !ch.Spec.Enabled {
			continue
		}

		channelType := ch.Spec.Type

		appIdSecret := &corev1.Secret{}
		err = r.Get(ctx, types.NamespacedName{
			Name:      ch.Spec.SecretRefs.AppId.Name,
			Namespace: namespace,
		}, appIdSecret)
		if err != nil {
			logger.Error(err, "Failed to get appId secret.", "Secret.Name", ch.Spec.SecretRefs.AppId.Name)
			if currentChannel != nil {
				currentChannel.Status.Phase = string(openclawiov1.OpenClawChannelPhaseError)
				currentChannel.Status.Message = fmt.Sprintf("Failed to get appId secret: %v", err)
				_ = r.Status().Update(ctx, currentChannel)
			}
			return ctrl.Result{}, err
		}

		appSecretSecret := &corev1.Secret{}
		err = r.Get(ctx, types.NamespacedName{
			Name:      ch.Spec.SecretRefs.AppSecret.Name,
			Namespace: namespace,
		}, appSecretSecret)
		if err != nil {
			logger.Error(err, "Failed to get appSecret secret.", "Secret.Name", ch.Spec.SecretRefs.AppSecret.Name)
			if currentChannel != nil {
				currentChannel.Status.Phase = string(openclawiov1.OpenClawChannelPhaseError)
				currentChannel.Status.Message = fmt.Sprintf("Failed to get appSecret secret: %v", err)
				_ = r.Status().Update(ctx, currentChannel)
			}
			return ctrl.Result{}, err
		}

		appIdValue := string(appIdSecret.Data[ch.Spec.SecretRefs.AppId.Key])
		appSecretValue := string(appSecretSecret.Data[ch.Spec.SecretRefs.AppSecret.Key])

		config.Channels[channelType] = ChannelTypeConfig{
			Enabled:        true,
			AppId:          appIdValue,
			AppSecret:      appSecretValue,
			RequireMention: ch.Spec.RequireMention,
			GroupPolicy:    ch.Spec.GroupPolicy,
			Groups:         ch.Spec.Groups,
		}
	}

	configMapName := openclawName + "-channels"

	if len(config.Channels) == 0 {
		logger.Info("No enabled channels found for OpenClaw instance, deleting ConfigMap.", "ConfigMap", configMapName)
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: namespace,
			},
		}
		err = r.Delete(ctx, configMap)
		if err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "Failed to delete ConfigMap.", "ConfigMap", configMapName)
			if currentChannel != nil {
				currentChannel.Status.Phase = string(openclawiov1.OpenClawChannelPhaseError)
				currentChannel.Status.Message = fmt.Sprintf("Failed to delete ConfigMap: %v", err)
				_ = r.Status().Update(ctx, currentChannel)
			}
			return ctrl.Result{}, err
		}
		logger.Info("ConfigMap deleted successfully.", "ConfigMap", configMapName)
		return ctrl.Result{}, nil
	}

	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		logger.Error(err, "Failed to marshal config to JSON.")
		if currentChannel != nil {
			currentChannel.Status.Phase = string(openclawiov1.OpenClawChannelPhaseError)
			currentChannel.Status.Message = fmt.Sprintf("Failed to marshal config: %v", err)
			_ = r.Status().Update(ctx, currentChannel)
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
			"channels.json": string(configJSON),
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
				if currentChannel != nil {
					currentChannel.Status.Phase = string(openclawiov1.OpenClawChannelPhaseError)
					currentChannel.Status.Message = fmt.Sprintf("Failed to create ConfigMap: %v", err)
					_ = r.Status().Update(ctx, currentChannel)
				}
				return ctrl.Result{}, err
			}
		} else {
			logger.Error(err, "Failed to get ConfigMap.")
			if currentChannel != nil {
				currentChannel.Status.Phase = string(openclawiov1.OpenClawChannelPhaseError)
				currentChannel.Status.Message = fmt.Sprintf("Failed to get ConfigMap: %v", err)
				_ = r.Status().Update(ctx, currentChannel)
			}
			return ctrl.Result{}, err
		}
	} else {
		foundConfigMap.Data = configMap.Data
		logger.Info("Updating ConfigMap.", "ConfigMap.Namespace", configMap.Namespace, "ConfigMap.Name", configMap.Name)
		err = r.Update(ctx, foundConfigMap)
		if err != nil {
			logger.Error(err, "Failed to update ConfigMap.", "ConfigMap.Namespace", configMap.Namespace, "ConfigMap.Name", configMap.Name)
			if currentChannel != nil {
				currentChannel.Status.Phase = string(openclawiov1.OpenClawChannelPhaseError)
				currentChannel.Status.Message = fmt.Sprintf("Failed to update ConfigMap: %v", err)
				_ = r.Status().Update(ctx, currentChannel)
			}
			return ctrl.Result{}, err
		}
	}

	if currentChannel != nil {
		currentChannel.Status.Phase = string(openclawiov1.OpenClawChannelPhaseReady)
		currentChannel.Status.Message = fmt.Sprintf("Successfully updated ConfigMap %s", configMapName)
		err = r.Status().Update(ctx, currentChannel)
		if err != nil {
			logger.Error(err, "Failed to update OpenClawChannel status.")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *OpenClawChannelReconciler) reconcileAllConfigMaps(ctx context.Context, namespace string) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	channels := &openclawiov1.OpenClawChannelList{}
	err := r.List(ctx, channels, client.InNamespace(namespace))
	if err != nil {
		logger.Error(err, "Failed to list OpenClawChannel resources.")
		return ctrl.Result{}, err
	}

	openclawToChannels := make(map[string][]openclawiov1.OpenClawChannel)
	for _, ch := range channels.Items {
		if ch.Spec.OpenclawRef.Name != "" && ch.Spec.Enabled {
			openclawToChannels[ch.Spec.OpenclawRef.Name] = append(openclawToChannels[ch.Spec.OpenclawRef.Name], ch)
		}
	}

	configMaps := &corev1.ConfigMapList{}
	err = r.List(ctx, configMaps, client.InNamespace(namespace))
	if err != nil {
		logger.Error(err, "Failed to list ConfigMaps.")
		return ctrl.Result{}, err
	}

	for _, cm := range configMaps.Items {
		if !strings.HasSuffix(cm.Name, "-channels") {
			continue
		}

		openclawName := strings.TrimSuffix(cm.Name, "-channels")
		channelList, exists := openclawToChannels[openclawName]

		if !exists || len(channelList) == 0 {
			logger.Info("No enabled channels found for OpenClaw instance, deleting ConfigMap.", "ConfigMap", cm.Name)
			err = r.Delete(ctx, &cm)
			if err != nil && !errors.IsNotFound(err) {
				logger.Error(err, "Failed to delete ConfigMap.", "ConfigMap", cm.Name)
				continue
			}
			logger.Info("ConfigMap deleted successfully.", "ConfigMap", cm.Name)
			continue
		}

		config := ChannelConfig{
			Channels: make(map[string]ChannelTypeConfig),
		}

		for _, ch := range channelList {
			channelType := ch.Spec.Type

			appIdSecret := &corev1.Secret{}
			err = r.Get(ctx, types.NamespacedName{
				Name:      ch.Spec.SecretRefs.AppId.Name,
				Namespace: namespace,
			}, appIdSecret)
			if err != nil {
				logger.Error(err, "Failed to get appId secret.", "Secret.Name", ch.Spec.SecretRefs.AppId.Name)
				continue
			}

			appSecretSecret := &corev1.Secret{}
			err = r.Get(ctx, types.NamespacedName{
				Name:      ch.Spec.SecretRefs.AppSecret.Name,
				Namespace: namespace,
			}, appSecretSecret)
			if err != nil {
				logger.Error(err, "Failed to get appSecret secret.", "Secret.Name", ch.Spec.SecretRefs.AppSecret.Name)
				continue
			}

			appIdValue := string(appIdSecret.Data[ch.Spec.SecretRefs.AppId.Key])
			appSecretValue := string(appSecretSecret.Data[ch.Spec.SecretRefs.AppSecret.Key])

			config.Channels[channelType] = ChannelTypeConfig{
				Enabled:        true,
				AppId:          appIdValue,
				AppSecret:      appSecretValue,
				RequireMention: ch.Spec.RequireMention,
				GroupPolicy:    ch.Spec.GroupPolicy,
				Groups:         ch.Spec.Groups,
			}
		}

		configJSON, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			logger.Error(err, "Failed to marshal config to JSON.")
			continue
		}

		cm.Data = map[string]string{
			"channels.json": string(configJSON),
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

func (r *OpenClawChannelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openclawiov1.OpenClawChannel{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
