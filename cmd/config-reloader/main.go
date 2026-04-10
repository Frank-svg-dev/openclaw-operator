package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

var lastAllowedOriginsModTime time.Time
var lastChannelsModTime time.Time
var lastAgentsModTime time.Time
var lastModelsModTime time.Time

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Printf("Failed to get user home directory: %v\n", err)
			return path
		}
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func main() {
	configPath := "/root/.openclaw"
	openclawJsonPath := filepath.Join(configPath, "openclaw.json")
	allowedOriginsJsonPath := "/etc/openclaw-config/allowed-origins/allowed-origins.json"
	channelsJsonPath := "/etc/openclaw-config/channels/channels.json"
	agentsJsonPath := "/etc/openclaw-config/agents/agents.json"
	modelsJsonPath := "/etc/openclaw-config/models/models.json"

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("Failed to create watcher: %v\n", err)
		os.Exit(1)
	}
	defer watcher.Close()

	err = watcher.Add(configPath)
	if err != nil {
		fmt.Printf("Failed to watch config path: %v\n", err)
		os.Exit(1)
	}

	err = watcher.Add("/etc/openclaw-config")
	if err != nil {
		fmt.Printf("Failed to watch config map path: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Config reloader started, watching:", configPath, "and", "/etc/openclaw-config")

	reloadConfig(openclawJsonPath, allowedOriginsJsonPath, channelsJsonPath, agentsJsonPath, modelsJsonPath)

	initializeModTimes(allowedOriginsJsonPath, channelsJsonPath, agentsJsonPath, modelsJsonPath)

	go pollConfigChanges(openclawJsonPath, allowedOriginsJsonPath, channelsJsonPath, agentsJsonPath, modelsJsonPath)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Remove == fsnotify.Remove || event.Op&fsnotify.Rename == fsnotify.Rename {
				if filepath.Base(event.Name) == "allowed-origins.json" || filepath.Base(event.Name) == "channels.json" || filepath.Base(event.Name) == "agents.json" || filepath.Base(event.Name) == "models.json" {
					fmt.Printf("Detected change in %s, reloading...\n", filepath.Base(event.Name))
					time.Sleep(500 * time.Millisecond)
					reloadConfig(openclawJsonPath, allowedOriginsJsonPath, channelsJsonPath, agentsJsonPath, modelsJsonPath)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("Watcher error: %v\n", err)
		}
	}
}

func pollConfigChanges(openclawJsonPath, allowedOriginsJsonPath, channelsJsonPath, agentsJsonPath, modelsJsonPath string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		checkAndReload(openclawJsonPath, allowedOriginsJsonPath, channelsJsonPath, agentsJsonPath, modelsJsonPath)
	}
}

func initializeModTimes(allowedOriginsJsonPath, channelsJsonPath, agentsJsonPath, modelsJsonPath string) {
	if info, err := os.Stat(allowedOriginsJsonPath); err == nil {
		lastAllowedOriginsModTime = info.ModTime()
	}

	if info, err := os.Stat(channelsJsonPath); err == nil {
		lastChannelsModTime = info.ModTime()
	}

	if info, err := os.Stat(agentsJsonPath); err == nil {
		lastAgentsModTime = info.ModTime()
	}

	if info, err := os.Stat(modelsJsonPath); err == nil {
		lastModelsModTime = info.ModTime()
	}
}

func checkAndReload(openclawJsonPath, allowedOriginsJsonPath, channelsJsonPath, agentsJsonPath, modelsJsonPath string) {
	allowedOriginsInfo, err := os.Stat(allowedOriginsJsonPath)
	if err == nil {
		if allowedOriginsInfo.ModTime().After(lastAllowedOriginsModTime) {
			fmt.Printf("Detected change in allowed-origins.json (polling), reloading...\n")
			lastAllowedOriginsModTime = allowedOriginsInfo.ModTime()
			time.Sleep(500 * time.Millisecond)
			reloadConfig(openclawJsonPath, allowedOriginsJsonPath, channelsJsonPath, agentsJsonPath, modelsJsonPath)
			return
		}
	}

	channelsInfo, err := os.Stat(channelsJsonPath)
	if err == nil {
		if channelsInfo.ModTime().After(lastChannelsModTime) {
			fmt.Printf("Detected change in channels.json (polling), reloading...\n")
			lastChannelsModTime = channelsInfo.ModTime()
			time.Sleep(500 * time.Millisecond)
			reloadConfig(openclawJsonPath, allowedOriginsJsonPath, channelsJsonPath, agentsJsonPath, modelsJsonPath)
			return
		}
	}

	agentsInfo, err := os.Stat(agentsJsonPath)
	if err == nil {
		if agentsInfo.ModTime().After(lastAgentsModTime) {
			fmt.Printf("Detected change in agents.json (polling), reloading...\n")
			lastAgentsModTime = agentsInfo.ModTime()
			time.Sleep(500 * time.Millisecond)
			reloadConfig(openclawJsonPath, allowedOriginsJsonPath, channelsJsonPath, agentsJsonPath, modelsJsonPath)
			return
		}
	}

	modelsInfo, err := os.Stat(modelsJsonPath)
	if err == nil {
		if modelsInfo.ModTime().After(lastModelsModTime) {
			fmt.Printf("Detected change in models.json (polling), reloading...\n")
			lastModelsModTime = modelsInfo.ModTime()
			time.Sleep(500 * time.Millisecond)
			reloadConfig(openclawJsonPath, allowedOriginsJsonPath, channelsJsonPath, agentsJsonPath, modelsJsonPath)
			return
		}
	}
}

func reloadConfig(openclawJsonPath, allowedOriginsJsonPath, channelsJsonPath, agentsJsonPath, modelsJsonPath string) {
	var config map[string]interface{}

	configData, err := os.ReadFile(openclawJsonPath)
	if err != nil {
		fmt.Printf("Failed to read openclaw.json: %v\n", err)
		return
	}

	if err := json.Unmarshal(configData, &config); err != nil {
		fmt.Printf("Failed to parse openclaw.json: %v\n", err)
		return
	}

	allowedOriginsData, err := os.ReadFile(allowedOriginsJsonPath)
	if err == nil {
		var allowedOriginsConfig map[string]interface{}
		if err := json.Unmarshal(allowedOriginsData, &allowedOriginsConfig); err == nil {
			if gateway, ok := allowedOriginsConfig["gateway"].(map[string]interface{}); ok {
				if controlUI, ok := gateway["controlUi"].(map[string]interface{}); ok {
					if _, ok := config["gateway"].(map[string]interface{}); !ok {
						config["gateway"] = make(map[string]interface{})
					}
					if _, ok := config["gateway"].(map[string]interface{})["controlUi"].(map[string]interface{}); !ok {
						config["gateway"].(map[string]interface{})["controlUi"] = make(map[string]interface{})
					}

					if allowedOrigins, ok := controlUI["allowedOrigins"]; ok {
						config["gateway"].(map[string]interface{})["controlUi"].(map[string]interface{})["allowedOrigins"] = allowedOrigins
						fmt.Println("Updated gateway.controlUi.allowedOrigins in openclaw.json")
					}

					if allowInsecureAuth, ok := controlUI["allowInsecureAuth"]; ok {
						config["gateway"].(map[string]interface{})["controlUi"].(map[string]interface{})["allowInsecureAuth"] = allowInsecureAuth
						fmt.Println("Updated gateway.controlUi.allowInsecureAuth in openclaw.json")
					}

					if dangerouslyDisableDeviceAuth, ok := controlUI["dangerouslyDisableDeviceAuth"]; ok {
						config["gateway"].(map[string]interface{})["controlUi"].(map[string]interface{})["dangerouslyDisableDeviceAuth"] = dangerouslyDisableDeviceAuth
						fmt.Println("Updated gateway.controlUi.dangerouslyDisableDeviceAuth in openclaw.json")
					}
				}
			}
		}
	}

	channelsData, err := os.ReadFile(channelsJsonPath)
	if err == nil {
		var channelsConfig map[string]interface{}
		if err := json.Unmarshal(channelsData, &channelsConfig); err == nil {
			if channels, ok := channelsConfig["channels"]; ok {
				config["channels"] = channels
				fmt.Println("Updated channels in openclaw.json")
			}
		}
	}

	modelsData, err := os.ReadFile(modelsJsonPath)
	if err != nil {
		fmt.Printf("Failed to read models.json: %v\n", err)
	} else {
		fmt.Printf("Successfully read models.json from %s\n", modelsJsonPath)
		var modelsConfig map[string]interface{}
		if err := json.Unmarshal(modelsData, &modelsConfig); err == nil {
			fmt.Printf("Successfully parsed models.json: %+v\n", modelsConfig)
			if models, ok := modelsConfig["models"]; ok {
				config["models"] = models
				fmt.Println("Updated models in openclaw.json")
			} else {
				fmt.Printf("models.json does not contain 'models' key, available keys: %v\n", getMapKeys(modelsConfig))
			}
		} else {
			fmt.Printf("Failed to parse models.json: %v, content: %s\n", err, string(modelsData))
		}
	}

	agentsData, err := os.ReadFile(agentsJsonPath)
	if err == nil {
		var agentsConfig map[string]interface{}
		if err := json.Unmarshal(agentsData, &agentsConfig); err == nil {
			if agents, ok := agentsConfig["agents"]; ok {
				config["agents"] = agents
				fmt.Println("Updated agents in openclaw.json")

				if agentsMap, ok := agents.(map[string]interface{}); ok {
					createAgentDirectories(agentsMap)
				}
			}
		}
	}

	updatedConfig, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal updated config: %v\n", err)
		return
	}

	if err := os.WriteFile(openclawJsonPath, updatedConfig, 0644); err != nil {
		fmt.Printf("Failed to write updated config: %v\n", err)
		return
	}

	fmt.Println("Successfully reloaded openclaw.json")
}

func createAgentDirectories(agents map[string]interface{}) {
	configPath := "/root/.openclaw"
	agentsPath := filepath.Join(configPath, "agents")
	openclawJsonPath := filepath.Join(configPath, "openclaw.json")

	var openclawConfig map[string]interface{}
	configData, err := os.ReadFile(openclawJsonPath)
	if err != nil {
		fmt.Printf("Failed to read openclaw.json: %v\n", err)
		return
	}
	if err := json.Unmarshal(configData, &openclawConfig); err != nil {
		fmt.Printf("Failed to parse openclaw.json: %v\n", err)
		return
	}

	if list, ok := agents["list"].([]interface{}); ok {
		for _, item := range list {
			if agent, ok := item.(map[string]interface{}); ok {
				if id, ok := agent["id"].(string); ok {
					agentDir := filepath.Join(agentsPath, id)
					if err := os.MkdirAll(agentDir, 0755); err != nil {
						fmt.Printf("Failed to create agent directory %s: %v\n", agentDir, err)
					} else {
						fmt.Printf("Created agent directory: %s\n", agentDir)
					}

					if workspace, ok := agent["workspace"].(string); ok && workspace != "" {
						expandedWorkspace := expandTilde(workspace)
						if err := os.MkdirAll(expandedWorkspace, 0755); err != nil {
							fmt.Printf("Failed to create workspace directory %s: %v\n", expandedWorkspace, err)
						} else {
							fmt.Printf("Created workspace directory: %s\n", expandedWorkspace)
						}
					}

					agentSubDir := filepath.Join(agentDir, "agent")
					if err := os.MkdirAll(agentSubDir, 0755); err != nil {
						fmt.Printf("Failed to create agent subdirectory %s: %v\n", agentSubDir, err)
					} else {
						fmt.Printf("Created agent subdirectory: %s\n", agentSubDir)
					}

					if model, ok := agent["model"].(map[string]interface{}); ok {
						if primary, ok := model["primary"].(string); ok && primary != "" {
							provider := extractProvider(primary)
							if provider != "" {
								createModelJson(agentSubDir, provider, primary, openclawConfig)
							}
						}
					}
				}
			}
		}
	}
}

func extractProvider(model string) string {
	for i, c := range model {
		if c == '/' {
			return model[:i]
		}
	}
	return ""
}

func createModelJson(agentDir, provider, model string, openclawConfig map[string]interface{}) {
	modelsJsonPath := filepath.Join(agentDir, "models.json")

	if models, ok := openclawConfig["models"].(map[string]interface{}); ok {
		if providers, ok := models["providers"].(map[string]interface{}); ok {
			if providerConfig, ok := providers[provider].(map[string]interface{}); ok {
				modelsJson := map[string]interface{}{
					"providers": map[string]interface{}{
						provider: providerConfig,
					},
				}

				jsonData, err := json.MarshalIndent(modelsJson, "", "  ")
				if err != nil {
					fmt.Printf("Failed to marshal models.json: %v\n", err)
					return
				}

				if err := os.WriteFile(modelsJsonPath, jsonData, 0644); err != nil {
					fmt.Printf("Failed to write models.json: %v\n", err)
				} else {
					fmt.Printf("Created models.json: %s\n", modelsJsonPath)
				}
			}
		}
	}
}
