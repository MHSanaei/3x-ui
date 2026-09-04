// Package psiphon manages a single Psiphon ConsoleClient sidecar process for
// the whole panel install (not one per inbound, unlike mtproto/amneziawgnet --
// there is only ever one local SOCKS proxy to offer as an outbound), the same
// shape as internal/tor.
//
// Unlike internal/tor, which expects a system-installed binary, Psiphon is not
// packaged by any distro -- this package downloads an official release itself,
// the same shape as internal/adguard. Unlike both, this package never
// generates working credentials: Psiphon's PropagationChannelId/SponsorId/
// server-list access are Psiphon Inc.'s to issue, not ours to invent, so the
// admin supplies their own psiphon.config and this package only patches the
// handful of fields it must control (see SaveConfig).
package psiphon

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// SocksPort is the fixed loopback port the managed Psiphon process listens
// on. Deliberately not 1080 -- an admin may already have something bound there.
const SocksPort = 19060

// startupTimeout bounds how long Start waits for the SOCKS listener, which
// Psiphon binds before any network activity -- well before a tunnel connects.
const startupTimeout = 10 * time.Second

func configDir() string   { return config.GetBinFolderPath() + "/psiphon" }
func dataDir() string     { return configDir() + "/data" }
func ConfigPath() string  { return configDir() + "/psiphon.config" }
func NoticesPath() string { return configDir() + "/notices.log" }

// IsConfigured reports whether the admin has supplied a psiphon.config yet,
// mirroring adguard.IsConfigured.
func IsConfigured() bool {
	info, err := os.Stat(ConfigPath())
	return err == nil && info.Mode().IsRegular()
}

// Manager owns the single Psiphon process for the whole install.
type Manager struct {
	mu   sync.Mutex
	proc *Process
}

var (
	managerOnce sync.Once
	manager     *Manager
)

// GetManager returns the process-wide Psiphon manager singleton.
func GetManager() *Manager {
	managerOnce.Do(func() { manager = &Manager{} })
	return manager
}

// Start launches Psiphon if it is not already running, waiting until its
// SOCKS listener accepts connections. A no-op, not an error, when already up.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startLocked()
}

func (m *Manager) startLocked() error {
	if m.proc != nil && m.proc.IsRunning() {
		return nil
	}
	if !IsInstalled() {
		return fmt.Errorf("Psiphon is not installed")
	}
	if !IsConfigured() {
		return fmt.Errorf("Psiphon has no config -- upload a psiphon.config first")
	}
	if err := os.MkdirAll(dataDir(), 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", dataDir(), err)
	}
	// Each run starts its notice log fresh -- -notices only ever appends, and
	// CurrentTunnel should reflect this process, not a stale prior run.
	_ = os.Remove(NoticesPath())
	proc := newProcess()
	if err := proc.Start(); err != nil {
		return err
	}
	if err := waitForListener(fmt.Sprintf("127.0.0.1:%d", SocksPort), proc); err != nil {
		_ = proc.Stop()
		return err
	}
	m.proc = proc
	logger.Infof("psiphon: started on 127.0.0.1:%d", SocksPort)
	return nil
}

// Stop stops Psiphon if it is running. A no-op, not an error, when already
// stopped, for the same symmetry reason as Start.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopLocked()
}

func (m *Manager) stopLocked() error {
	if m.proc == nil || !m.proc.IsRunning() {
		m.proc = nil
		return nil
	}
	err := m.proc.Stop()
	m.proc = nil
	return err
}

// StopAll stops the managed Psiphon process, matching every other sidecar
// manager's StopAll() shape so web.go's shutdown sequence can call it the same way.
func (m *Manager) StopAll() {
	_ = m.Stop()
}

// IsRunning reports whether the process is alive, not whether a tunnel is
// connected -- the same distinction internal/tor draws. See CurrentTunnel.
func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.proc != nil && m.proc.IsRunning()
}

// LastResult returns the last log line or exit error from the most recent
// Psiphon process, or "" if none has run yet this session.
func (m *Manager) LastResult() string {
	m.mu.Lock()
	proc := m.proc
	m.mu.Unlock()
	if proc == nil {
		return ""
	}
	return proc.GetResult()
}

// Restart stops and starts Psiphon, used after SaveConfig changes EgressRegion
// or the config itself -- Psiphon does not hot-reload either.
func (m *Manager) Restart() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.stopLocked(); err != nil {
		return err
	}
	return m.startLocked()
}
