package tor

import (
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// SocksPort is the fixed loopback port the managed tor daemon listens on.
// Deliberately not 9050 (the common system-tor default) to avoid colliding
// with an unrelated tor instance an admin may already be running.
const SocksPort = 19050

func configDir() string { return config.GetBinFolderPath() + "/tor" }
func dataDir() string   { return configDir() + "/data" }
func torrcPath() string { return configDir() + "/torrc" }

// IsAvailable reports whether a `tor` binary is on PATH. This package never
// bundles or downloads one -- the admin installs it via their own package
// manager (apt/dnf/pacman install tor).
func IsAvailable() bool {
	_, err := exec.LookPath("tor")
	return err == nil
}

// renderTorrc builds the minimal config for a loopback-only Tor client.
// ClientOnly is set explicitly, in addition to simply never setting an
// ORPort, so this can never turn into a relay/exit node even by a future
// config mistake -- that has real bandwidth/abuse exposure for whoever owns
// the host, and is never what a one-click "Tor outbound" button should do.
func renderTorrc() string {
	return fmt.Sprintf(
		"SocksPort 127.0.0.1:%d\nDataDirectory %s\nClientOnly 1\nRunAsDaemon 0\nLog notice stdout\n",
		SocksPort, dataDir(),
	)
}

// writeTorrc lays out the config directory and torrc. DataDirectory must be
// 0700 -- Tor refuses to start against a group/world-readable one.
func writeTorrc() error {
	if err := os.MkdirAll(dataDir(), 0o700); err != nil {
		return err
	}
	return os.WriteFile(torrcPath(), []byte(renderTorrc()), 0o640)
}

// Manager owns the single tor process for the whole install.
type Manager struct {
	mu   sync.Mutex
	proc *Process
}

var (
	managerOnce sync.Once
	manager     *Manager
)

// GetManager returns the process-wide tor manager singleton.
func GetManager() *Manager {
	managerOnce.Do(func() { manager = &Manager{} })
	return manager
}

// Start launches tor if it is not already running. A no-op, not an error,
// when it's already up -- callers (the API handler, boot-time auto-start)
// both want "make sure it's running," not "fail if it already is."
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.proc != nil && m.proc.IsRunning() {
		return nil
	}
	if !IsAvailable() {
		return fmt.Errorf("tor binary not found on PATH")
	}
	if err := writeTorrc(); err != nil {
		return fmt.Errorf("writing torrc: %w", err)
	}
	proc := newProcess(torrcPath())
	if err := proc.Start(); err != nil {
		return err
	}
	m.proc = proc
	logger.Infof("tor: started on 127.0.0.1:%d", SocksPort)
	return nil
}

// Stop stops tor if it is running. A no-op, not an error, when it's already
// stopped, for the same symmetry reason as Start.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.proc == nil || !m.proc.IsRunning() {
		return nil
	}
	err := m.proc.Stop()
	m.proc = nil
	return err
}

// StopAll stops the managed tor process. Called on panel shutdown; matches
// the StopAll() shape of every other sidecar manager (mtproto, amneziawgnet)
// so web.go's shutdown sequence can call it the same way.
func (m *Manager) StopAll() {
	_ = m.Stop()
}

// IsRunning reports whether tor is currently running.
func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.proc != nil && m.proc.IsRunning()
}

// LastResult returns the last log line or exit error from the most recent
// tor process, or "" if none has run yet this session.
func (m *Manager) LastResult() string {
	m.mu.Lock()
	proc := m.proc
	m.mu.Unlock()
	if proc == nil {
		return ""
	}
	return proc.GetResult()
}
