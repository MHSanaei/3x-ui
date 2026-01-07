// Package xray provides XRAY Core management for the node service.
package xray

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/json_util"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

// NodeStats represents traffic and online clients statistics from a node.
type NodeStats struct {
	Traffic       []*xray.Traffic       `json:"traffic"`
	ClientTraffic []*xray.ClientTraffic `json:"clientTraffic"`
	OnlineClients []string               `json:"onlineClients"`
}

// Manager manages the XRAY Core process lifecycle.
type Manager struct {
	process *xray.Process
	lock    sync.Mutex
	config  *xray.Config
}

// NewManager creates a new XRAY manager instance.
func NewManager() *Manager {
	m := &Manager{}
	// Download geo files if missing
	m.downloadGeoFiles()
	// Try to load config from file on startup
	m.LoadConfigFromFile()
	return m
}

// downloadGeoFiles downloads geo data files if they are missing.
// These files are required for routing rules that use geoip/geosite matching.
func (m *Manager) downloadGeoFiles() {
	// Possible bin folder paths (in order of priority)
	binPaths := []string{
		"bin",
		"/app/bin",
		"./bin",
	}

	var binPath string
	for _, path := range binPaths {
		if _, err := os.Stat(path); err == nil {
			binPath = path
			break
		}
	}

	if binPath == "" {
		logger.Debug("No bin folder found, skipping geo files download")
		return
	}

	// List of geo files to download
	geoFiles := []struct {
		URL      string
		FileName string
	}{
		{"https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat", "geoip.dat"},
		{"https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat", "geosite.dat"},
		{"https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat", "geoip_IR.dat"},
		{"https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat", "geosite_IR.dat"},
		{"https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geoip.dat", "geoip_RU.dat"},
		{"https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geosite.dat", "geosite_RU.dat"},
	}

	downloadFile := func(url, destPath string) error {
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("failed to download: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("bad status: %d", resp.StatusCode)
		}

		file, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer file.Close()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		return nil
	}

	for _, file := range geoFiles {
		destPath := filepath.Join(binPath, file.FileName)
		
		// Check if file already exists
		if _, err := os.Stat(destPath); err == nil {
			logger.Debugf("Geo file %s already exists, skipping download", file.FileName)
			continue
		}

		logger.Infof("Downloading geo file: %s", file.FileName)
		if err := downloadFile(file.URL, destPath); err != nil {
			logger.Warningf("Failed to download %s: %v", file.FileName, err)
		} else {
			logger.Infof("Successfully downloaded %s", file.FileName)
		}
	}
}

// LoadConfigFromFile attempts to load XRAY configuration from config.json file.
// It checks multiple possible locations: bin/config.json, config/config.json, and ./config.json
func (m *Manager) LoadConfigFromFile() error {
	// Possible config file paths (in order of priority)
	configPaths := []string{
		"bin/config.json",
		"config/config.json",
		"./config.json",
		"/app/bin/config.json",
		"/app/config/config.json",
	}

	var configData []byte
	var configPath string

	// Try each path until we find a valid config file
	for _, path := range configPaths {
		if _, statErr := os.Stat(path); statErr == nil {
			var readErr error
			configData, readErr = os.ReadFile(path)
			if readErr == nil {
				configPath = path
				break
			}
		}
	}

	// If no config file found, that's okay - node will wait for config from panel
	if configPath == "" {
		logger.Debug("No config.json found, node will wait for configuration from panel")
		return nil
	}

	// Validate JSON
	var configJSON json.RawMessage
	if err := json.Unmarshal(configData, &configJSON); err != nil {
		logger.Warningf("Config file %s contains invalid JSON: %v", configPath, err)
		return fmt.Errorf("invalid JSON in config file: %w", err)
	}

	// Parse full config
	var config xray.Config
	if err := json.Unmarshal(configData, &config); err != nil {
		logger.Warningf("Failed to parse config from %s: %v", configPath, err)
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Check if API inbound exists, if not add it
	hasAPIInbound := false
	for _, inbound := range config.InboundConfigs {
		if inbound.Tag == "api" {
			hasAPIInbound = true
			break
		}
	}

	// If no API inbound found, add a default one
	if !hasAPIInbound {
		logger.Debug("No API inbound found in config, adding default API inbound")
		apiInbound := xray.InboundConfig{
			Tag:      "api",
			Port:     62789, // Default API port
			Protocol: "tunnel",
			Listen:   json_util.RawMessage(`"127.0.0.1"`),
			Settings: json_util.RawMessage(`{"address":"127.0.0.1"}`),
		}
		// Add API inbound at the beginning
		config.InboundConfigs = append([]xray.InboundConfig{apiInbound}, config.InboundConfigs...)
		// Update configData with the new inbound
		configData, _ = json.MarshalIndent(&config, "", "  ")
	}

	// Check if config has inbounds (after adding API inbound)
	if len(config.InboundConfigs) == 0 {
		logger.Debug("Config file found but no inbounds configured, skipping XRAY start")
		return nil
	}

	// Apply the loaded config (this will start XRAY)
	logger.Infof("Loading XRAY configuration from %s", configPath)
	if err := m.ApplyConfig(configData); err != nil {
		logger.Errorf("Failed to apply config from file: %v", err)
		return fmt.Errorf("failed to apply config: %w", err)
	}

	logger.Info("XRAY started successfully from config file")
	return nil
}

// IsRunning returns true if XRAY is currently running.
func (m *Manager) IsRunning() bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.process != nil && m.process.IsRunning()
}

// GetStatus returns the current status of XRAY.
func (m *Manager) GetStatus() map[string]interface{} {
	m.lock.Lock()
	defer m.lock.Unlock()

	status := map[string]interface{}{
		"running": m.process != nil && m.process.IsRunning(),
		"version": "Unknown",
		"uptime":  0,
	}

	if m.process != nil && m.process.IsRunning() {
		status["version"] = m.process.GetVersion()
		status["uptime"] = m.process.GetUptime()
	}

	return status
}

// ApplyConfig applies a new XRAY configuration and restarts if needed.
func (m *Manager) ApplyConfig(configJSON []byte) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	var newConfig xray.Config
	if err := json.Unmarshal(configJSON, &newConfig); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// If XRAY is running and config is the same, skip restart
	if m.process != nil && m.process.IsRunning() {
		oldConfig := m.process.GetConfig()
		if oldConfig != nil && oldConfig.Equals(&newConfig) {
			logger.Info("Config unchanged, skipping restart")
			return nil
		}
		// Stop existing process
		if err := m.process.Stop(); err != nil {
			logger.Warningf("Failed to stop existing XRAY: %v", err)
		}
	}

	// Start new process with new config
	m.config = &newConfig
	m.process = xray.NewProcess(&newConfig)
	if err := m.process.Start(); err != nil {
		return fmt.Errorf("failed to start XRAY: %w", err)
	}

	logger.Info("XRAY configuration applied successfully")
	return nil
}

// Reload reloads XRAY configuration without full restart (if supported).
// Falls back to restart if reload is not available.
func (m *Manager) Reload() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.process == nil || !m.process.IsRunning() {
		return errors.New("XRAY is not running")
	}

	// XRAY doesn't support hot reload, so we need to restart
	// Save current config
	if m.config == nil {
		return errors.New("no config to reload")
	}

	// Stop and restart
	if err := m.process.Stop(); err != nil {
		return fmt.Errorf("failed to stop XRAY: %w", err)
	}

	m.process = xray.NewProcess(m.config)
	if err := m.process.Start(); err != nil {
		return fmt.Errorf("failed to restart XRAY: %w", err)
	}

	logger.Info("XRAY reloaded successfully")
	return nil
}

// ForceReload forcefully reloads XRAY even if it's not running or hung.
// It stops XRAY if running, loads config from file if available, and restarts.
func (m *Manager) ForceReload() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Stop XRAY if it's running (even if hung)
	if m.process != nil {
		// Try to stop gracefully, but don't fail if it's hung
		_ = m.process.Stop()
		// Give it a moment to stop
		time.Sleep(500 * time.Millisecond)
		// Force kill if still running
		if m.process.IsRunning() {
			logger.Warning("XRAY process appears hung, forcing stop")
			// Process will be cleaned up by finalizer or on next start
		}
		m.process = nil
	}

	// Try to load config from file first (if available)
	configPaths := []string{
		"bin/config.json",
		"config/config.json",
		"./config.json",
		"/app/bin/config.json",
		"/app/config/config.json",
	}
	
	var configData []byte
	var configPath string
	
	// Find config file
	for _, path := range configPaths {
		if _, statErr := os.Stat(path); statErr == nil {
			var readErr error
			configData, readErr = os.ReadFile(path)
			if readErr == nil {
				configPath = path
				break
			}
		}
	}
	
	// If config file found, try to use it
	if configPath != "" {
		var config xray.Config
		if err := json.Unmarshal(configData, &config); err == nil {
			// Check if config has inbounds
			if len(config.InboundConfigs) > 0 {
				// Check if API inbound exists
				hasAPIInbound := false
				for _, inbound := range config.InboundConfigs {
					if inbound.Tag == "api" {
						hasAPIInbound = true
						break
					}
				}
				
				// Add API inbound if missing
				if !hasAPIInbound {
					apiInbound := xray.InboundConfig{
						Tag:      "api",
						Port:     62789,
						Protocol: "tunnel",
						Listen:   json_util.RawMessage(`"127.0.0.1"`),
						Settings: json_util.RawMessage(`{"address":"127.0.0.1"}`),
					}
					config.InboundConfigs = append([]xray.InboundConfig{apiInbound}, config.InboundConfigs...)
					configData, _ = json.MarshalIndent(&config, "", "  ")
				}
				
				// Apply config from file
				m.config = &config
				m.process = xray.NewProcess(&config)
				if err := m.process.Start(); err == nil {
					logger.Infof("XRAY force reloaded successfully from config file %s", configPath)
					return nil
				}
			}
		}
		// If loading from file failed, continue with saved config
	}

	// If no config file, try to use saved config
	if m.config == nil {
		return errors.New("no config available to reload")
	}

	// Restart with saved config
	m.process = xray.NewProcess(m.config)
	if err := m.process.Start(); err != nil {
		return fmt.Errorf("failed to restart XRAY: %w", err)
	}

	logger.Info("XRAY force reloaded successfully")
	return nil
}

// Stop stops the XRAY process.
func (m *Manager) Stop() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.process == nil || !m.process.IsRunning() {
		return nil
	}

	return m.process.Stop()
}

// GetStats returns traffic and online clients statistics from XRAY.
func (m *Manager) GetStats(reset bool) (*NodeStats, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.process == nil || !m.process.IsRunning() {
		return nil, errors.New("XRAY is not running")
	}

	// Get API port from process
	apiPort := m.process.GetAPIPort()
	if apiPort == 0 {
		return nil, errors.New("XRAY API port is not available")
	}

	// Create XrayAPI instance and initialize
	xrayAPI := &xray.XrayAPI{}
	if err := xrayAPI.Init(apiPort); err != nil {
		return nil, fmt.Errorf("failed to initialize XrayAPI: %w", err)
	}
	defer xrayAPI.Close()

	// Get traffic statistics
	traffics, clientTraffics, err := xrayAPI.GetTraffic(reset)
	if err != nil {
		return nil, fmt.Errorf("failed to get traffic: %w", err)
	}

	// Get online clients from process
	onlineClients := m.process.GetOnlineClients()

	// Also check online clients from traffic (clients with traffic > 0)
	onlineFromTraffic := make(map[string]bool)
	for _, ct := range clientTraffics {
		if ct.Up+ct.Down > 0 {
			onlineFromTraffic[ct.Email] = true
		}
	}

	// Merge online clients
	onlineSet := make(map[string]bool)
	for _, email := range onlineClients {
		onlineSet[email] = true
	}
	for email := range onlineFromTraffic {
		onlineSet[email] = true
	}

	onlineList := make([]string, 0, len(onlineSet))
	for email := range onlineSet {
		onlineList = append(onlineList, email)
	}

	return &NodeStats{
		Traffic:       traffics,
		ClientTraffic: clientTraffics,
		OnlineClients: onlineList,
	}, nil
}
