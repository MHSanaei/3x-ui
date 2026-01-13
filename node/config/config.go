// Package config provides node configuration management, including API key persistence.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// NodeConfig represents the node's configuration stored on disk.
type NodeConfig struct {
	ApiKey     string `json:"apiKey"`     // API key for authentication with panel
	PanelURL   string `json:"panelUrl"`  // Panel URL (optional, can be set via env var)
	NodeAddress string `json:"nodeAddress"` // Node's own address (optional)
}

var (
	config     *NodeConfig
	configMu   sync.RWMutex
	configPath string
)

// InitConfig initializes the configuration system and loads existing config if available.
// configDir is the directory where config file will be stored (e.g., "bin", "/app/bin").
func InitConfig(configDir string) error {
	configMu.Lock()
	defer configMu.Unlock()

	// Determine config file path
	if configDir == "" {
		// Try common paths
		possibleDirs := []string{"bin", "config", ".", "/app/bin", "/app/config"}
		for _, dir := range possibleDirs {
			if _, err := os.Stat(dir); err == nil {
				configDir = dir
				break
			}
		}
		if configDir == "" {
			configDir = "." // Fallback to current directory
		}
	}

	configPath = filepath.Join(configDir, "node-config.json")

	// Try to load existing config
	if data, err := os.ReadFile(configPath); err == nil {
		var loadedConfig NodeConfig
		if err := json.Unmarshal(data, &loadedConfig); err == nil {
			config = &loadedConfig
			return nil
		}
		// If file exists but is invalid, we'll create a new one
	}

	// Create empty config if file doesn't exist
	config = &NodeConfig{}
	return nil
}

// GetConfig returns the current node configuration.
func GetConfig() *NodeConfig {
	configMu.RLock()
	defer configMu.RUnlock()

	if config == nil {
		return &NodeConfig{}
	}

	// Return a copy to prevent external modifications
	return &NodeConfig{
		ApiKey:      config.ApiKey,
		PanelURL:    config.PanelURL,
		NodeAddress: config.NodeAddress,
	}
}

// SetApiKey sets the API key and saves it to disk.
// If an API key already exists, it will not be overwritten unless force is true.
func SetApiKey(apiKey string, force bool) error {
	configMu.Lock()
	defer configMu.Unlock()

	if config == nil {
		config = &NodeConfig{}
	}

	// Check if API key already exists
	if config.ApiKey != "" && !force {
		return fmt.Errorf("API key already exists. Use force=true to overwrite")
	}

	config.ApiKey = apiKey
	return saveConfig()
}

// SetPanelURL sets the panel URL and saves it to disk.
func SetPanelURL(url string) error {
	configMu.Lock()
	defer configMu.Unlock()

	if config == nil {
		config = &NodeConfig{}
	}

	config.PanelURL = url
	return saveConfig()
}

// SetNodeAddress sets the node address and saves it to disk.
func SetNodeAddress(address string) error {
	configMu.Lock()
	defer configMu.Unlock()

	if config == nil {
		config = &NodeConfig{}
	}

	config.NodeAddress = address
	return saveConfig()
}

// saveConfig saves the current configuration to disk.
func saveConfig() error {
	if configPath == "" {
		return fmt.Errorf("config path not initialized, call InitConfig first")
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal config to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file with proper permissions (readable/writable by owner only)
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetConfigPath returns the path to the config file.
func GetConfigPath() string {
	configMu.RLock()
	defer configMu.RUnlock()
	return configPath
}
