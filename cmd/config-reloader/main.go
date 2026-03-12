package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

var lastAllowedOriginsModTime time.Time
var lastChannelsModTime time.Time

func main() {
	configPath := "/home/node/.openclaw"
	openclawJsonPath := filepath.Join(configPath, "openclaw.json")
	allowedOriginsJsonPath := "/etc/openclaw-config/allowed-origins/allowed-origins.json"
	channelsJsonPath := "/etc/openclaw-config/channels/channels.json"

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

	reloadConfig(openclawJsonPath, allowedOriginsJsonPath, channelsJsonPath)

	initializeModTimes(allowedOriginsJsonPath, channelsJsonPath)

	go pollConfigChanges(openclawJsonPath, allowedOriginsJsonPath, channelsJsonPath)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Write == fsnotify.Write {
				if filepath.Base(event.Name) == "allowed-origins.json" || filepath.Base(event.Name) == "channels.json" {
					fmt.Printf("Detected change in %s, reloading...\n", filepath.Base(event.Name))
					time.Sleep(500 * time.Millisecond)
					reloadConfig(openclawJsonPath, allowedOriginsJsonPath, channelsJsonPath)
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

func pollConfigChanges(openclawJsonPath, allowedOriginsJsonPath, channelsJsonPath string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		checkAndReload(openclawJsonPath, allowedOriginsJsonPath, channelsJsonPath)
	}
}

func initializeModTimes(allowedOriginsJsonPath, channelsJsonPath string) {
	if info, err := os.Stat(allowedOriginsJsonPath); err == nil {
		lastAllowedOriginsModTime = info.ModTime()
	}

	if info, err := os.Stat(channelsJsonPath); err == nil {
		lastChannelsModTime = info.ModTime()
	}
}

func checkAndReload(openclawJsonPath, allowedOriginsJsonPath, channelsJsonPath string) {
	allowedOriginsInfo, err := os.Stat(allowedOriginsJsonPath)
	if err == nil {
		if allowedOriginsInfo.ModTime().After(lastAllowedOriginsModTime) {
			fmt.Printf("Detected change in allowed-origins.json (polling), reloading...\n")
			lastAllowedOriginsModTime = allowedOriginsInfo.ModTime()
			time.Sleep(500 * time.Millisecond)
			reloadConfig(openclawJsonPath, allowedOriginsJsonPath, channelsJsonPath)
			return
		}
	}

	channelsInfo, err := os.Stat(channelsJsonPath)
	if err == nil {
		if channelsInfo.ModTime().After(lastChannelsModTime) {
			fmt.Printf("Detected change in channels.json (polling), reloading...\n")
			lastChannelsModTime = channelsInfo.ModTime()
			time.Sleep(500 * time.Millisecond)
			reloadConfig(openclawJsonPath, allowedOriginsJsonPath, channelsJsonPath)
			return
		}
	}
}

func reloadConfig(openclawJsonPath, allowedOriginsJsonPath, channelsJsonPath string) {
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
					if allowedOrigins, ok := controlUI["allowedOrigins"]; ok {
						if _, ok := config["gateway"].(map[string]interface{}); !ok {
							config["gateway"] = make(map[string]interface{})
						}
						if _, ok := config["gateway"].(map[string]interface{})["controlUi"].(map[string]interface{}); !ok {
							config["gateway"].(map[string]interface{})["controlUi"] = make(map[string]interface{})
						}
						config["gateway"].(map[string]interface{})["controlUi"].(map[string]interface{})["allowedOrigins"] = allowedOrigins
						fmt.Println("Updated gateway.controlUi.allowedOrigins in openclaw.json")
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
