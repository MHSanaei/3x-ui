package adguard

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// startupTimeout bounds how long Start waits for AdGuard Home to bind its
// listener. Loading filter lists happens after that, so this only covers
// coming up, not becoming useful.
const startupTimeout = 15 * time.Second

// configView is the sliver of AdGuardHome.yaml this package reads back. The
// file belongs to AdGuard Home, which rewrites it in full whenever the admin
// changes anything in its UI, so everything else is deliberately ignored.
type configView struct {
	HTTP struct {
		Address string `yaml:"address"`
	} `yaml:"http"`
	DNS struct {
		Port int `yaml:"port"`
	} `yaml:"dns"`
	Users []struct {
		Name string `yaml:"name"`
	} `yaml:"users"`
}

func readConfig() (configView, error) {
	var cfg configView
	raw, err := os.ReadFile(ConfigPath())
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("cannot parse %s: %w", ConfigPath(), err)
	}
	return cfg, nil
}

// WebAddr returns the loopback address AdGuard Home serves its UI on, read
// from its config rather than from panel settings.
//
// The seeded port is only a starting value: an admin who changes it inside
// AdGuard Home's own UI would otherwise leave the reverse proxy forwarding to
// a port nothing listens on, with no sign of why the site went blank.
func WebAddr() (string, error) {
	cfg, err := readConfig()
	if err != nil {
		return "", err
	}
	if cfg.HTTP.Address == "" {
		return "", fmt.Errorf("%s has no http.address", ConfigPath())
	}
	return cfg.HTTP.Address, nil
}

// WebPort returns just the port half of WebAddr, which is what the reverse
// proxy needs to build its loopback target.
func WebPort() (int, error) {
	addr, err := WebAddr()
	if err != nil {
		return 0, err
	}
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, fmt.Errorf("malformed http.address %q: %w", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("malformed http.address %q: %w", addr, err)
	}
	return port, nil
}

// DNSPort returns the loopback port AdGuard Home answers plain DNS on, for the
// panel to show admins who want to point the host's own resolver at it.
func DNSPort() (int, error) {
	cfg, err := readConfig()
	if err != nil {
		return 0, err
	}
	return cfg.DNS.Port, nil
}

// CurrentUser returns the account name AdGuard Home will accept, read from its
// config so it stays right whether the name was changed from this panel or
// from AdGuard Home's own settings page.
func CurrentUser() (string, error) {
	cfg, err := readConfig()
	if err != nil {
		return "", err
	}
	if len(cfg.Users) == 0 || cfg.Users[0].Name == "" {
		return "", fmt.Errorf("%s has no account", ConfigPath())
	}
	return cfg.Users[0].Name, nil
}

// IsConfigured reports whether a seeded config is present, which together with
// IsInstalled is what the settings UI checks before offering to start.
func IsConfigured() bool {
	_, err := os.Stat(ConfigPath())
	return err == nil
}

// Manager owns the single AdGuard Home process for the whole install.
type Manager struct {
	mu   sync.Mutex
	proc *Process
}

var (
	managerOnce sync.Once
	manager     *Manager
)

// GetManager returns the process-wide AdGuard Home manager singleton.
func GetManager() *Manager {
	managerOnce.Do(func() { manager = &Manager{} })
	return manager
}

// Start launches AdGuard Home if it is not already running, waiting until its
// web listener accepts connections. A no-op, not an error, when already up.
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
		return fmt.Errorf("AdGuard Home is not installed")
	}
	addr, err := WebAddr()
	if err != nil {
		return fmt.Errorf("AdGuard Home config: %w", err)
	}
	proc := newProcess()
	if err := proc.Start(); err != nil {
		return err
	}
	if err := waitForListener(addr, proc); err != nil {
		_ = proc.Stop()
		return err
	}
	m.proc = proc
	logger.Infof("adguard: started on %s", addr)
	return nil
}

// waitForListener blocks until the address accepts a connection, giving up
// early if the process died -- that error is far more useful than a timeout.
func waitForListener(addr string, proc *Process) error {
	deadline := time.Now().Add(startupTimeout)
	for time.Now().Before(deadline) {
		if !proc.IsRunning() {
			return fmt.Errorf("AdGuard Home exited during startup: %s", proc.GetResult())
		}
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("AdGuard Home did not start listening on %s: %s", addr, proc.GetResult())
}

// Stop stops AdGuard Home if it is running. A no-op when already stopped.
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

// SetCredentials replaces the AdGuard Home account, restarting it if it was
// running so the new password actually takes effect.
//
// The stop is not optional. AdGuard Home keeps its whole configuration in
// memory and rewrites the file whenever anything changes, so an edit applied
// underneath a live instance can simply be overwritten again -- leaving an
// admin locked out with a password the panel believes it set. Holding the
// manager lock for the whole sequence keeps a concurrent Start or Stop from
// interleaving with it.
func (m *Manager) SetCredentials(user, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !IsConfigured() {
		return fmt.Errorf("AdGuard Home is not installed")
	}
	wasRunning := m.proc != nil && m.proc.IsRunning()
	if wasRunning {
		if err := m.stopLocked(); err != nil {
			return err
		}
	}
	if err := writeCredentials(user, password); err != nil {
		// Bring it back up on a failed edit: the config is untouched, so the
		// admin is better off with the service running than silently down.
		if wasRunning {
			if startErr := m.startLocked(); startErr != nil {
				logger.Warningf("adguard: could not restart after a failed credential change: %v", startErr)
			}
		}
		return err
	}
	if !wasRunning {
		return nil
	}
	return m.startLocked()
}

// StopAll stops the managed process. Matches the StopAll() shape of the other
// sidecar managers so web.go's shutdown sequence can call it the same way.
func (m *Manager) StopAll() {
	_ = m.Stop()
}

// IsRunning reports whether AdGuard Home is currently running.
func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.proc != nil && m.proc.IsRunning()
}

// LastResult returns the last log line or exit error from the most recent
// AdGuard Home process, or "" if none has run yet this session.
func (m *Manager) LastResult() string {
	m.mu.Lock()
	proc := m.proc
	m.mu.Unlock()
	if proc == nil {
		return ""
	}
	return proc.GetResult()
}
