// Package xray provides XRAY Core management for the node service.
package xray

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

// Manager manages the XRAY Core process lifecycle.
type Manager struct {
	process *xray.Process
	lock    sync.Mutex
	config  *xray.Config
}

// NewManager creates a new XRAY manager instance.
func NewManager() *Manager {
	return &Manager{}
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

// Stop stops the XRAY process.
func (m *Manager) Stop() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.process == nil || !m.process.IsRunning() {
		return nil
	}

	return m.process.Stop()
}
