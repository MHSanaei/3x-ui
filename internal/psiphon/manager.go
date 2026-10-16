package psiphon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

var (
	globalManager     *Manager
	globalManagerOnce sync.Once
)

// GetManager returns the singleton Psiphon Manager.
func GetManager() *Manager {
	globalManagerOnce.Do(func() {
		globalManager = NewManager()
	})
	return globalManager
}

// Manager supervises Psiphon sidecar instances, port allocation, and country discovery.
type Manager struct {
	mu               sync.RWMutex
	instances        map[string]*Instance
	processes        map[string]*Process
	allocatedPorts   map[int]string
	availableRegions map[string]struct{}
	baseConfigJSON   string
	dataBaseDir      string
}

// NewManager creates an initialized Psiphon Manager.
func NewManager() *Manager {
	dataDir := filepath.Join(config.GetBinFolderPath(), "psiphon_data")
	mgr := &Manager{
		instances:        make(map[string]*Instance),
		processes:        make(map[string]*Process),
		allocatedPorts:   make(map[int]string),
		availableRegions: make(map[string]struct{}),
		dataBaseDir:      dataDir,
	}
	killStrayPsiphonProcesses(GetBinaryPath())
	return mgr
}

// SetBaseConfig updates the master Psiphon deployment JSON.
func (m *Manager) SetBaseConfig(cfgJSON string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.baseConfigJSON = cfgJSON
}

// GetBaseConfig returns the current master deployment JSON.
func (m *Manager) GetBaseConfig() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.baseConfigJSON
}

// SetDataBaseDir overrides the root directory for instance data directories (used in tests).
func (m *Manager) SetDataBaseDir(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dataBaseDir = dir
}

// AllocatePort finds the lowest available loopback TCP port in the 40001-40099 range.
func (m *Manager) AllocatePort(preferred int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if preferred >= DefaultBasePort && preferred <= DefaultMaxPort {
		if _, used := m.allocatedPorts[preferred]; !used && isPortAvailable(preferred) {
			return preferred, nil
		}
	}

	for port := DefaultBasePort; port <= DefaultMaxPort; port++ {
		if _, used := m.allocatedPorts[port]; !used && isPortAvailable(port) {
			return port, nil
		}
	}
	return 0, errors.New("no available ports in psiphon port range (40001-40099)")
}

func isPortAvailable(port int) bool {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// RegisterInstance registers an instance into memory and tracks its port.
func (m *Manager) RegisterInstance(inst *Instance) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if inst == nil || inst.ID == "" {
		return errors.New("invalid instance")
	}
	if otherID, used := m.allocatedPorts[inst.SocksPort]; used && otherID != inst.ID {
		return fmt.Errorf("port %d already allocated to instance %s", inst.SocksPort, otherID)
	}
	m.instances[inst.ID] = inst
	m.allocatedPorts[inst.SocksPort] = inst.ID
	return nil
}

// UnregisterInstance stops any running process and frees the instance and its port.
func (m *Manager) UnregisterInstance(id string) {
	_ = m.Stop(id)

	m.mu.Lock()
	defer m.mu.Unlock()
	if inst, ok := m.instances[id]; ok {
		delete(m.allocatedPorts, inst.SocksPort)
		delete(m.instances, id)
	}
	delete(m.processes, id)
}

// GetInstance returns a snapshot copy of an instance by ID.
func (m *Manager) GetInstance(id string) (*Instance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	inst, ok := m.instances[id]
	if !ok {
		return nil, false
	}
	cpy := *inst
	return &cpy, true
}

// GetActiveInstances returns all currently tracked instances.
func (m *Manager) GetActiveInstances() []*Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]*Instance, 0, len(m.instances))
	for _, inst := range m.instances {
		cpy := *inst
		res = append(res, &cpy)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].CountryCode < res[j].CountryCode
	})
	return res
}

// Start launches the ConsoleClient process for an instance.
func (m *Manager) Start(id string) error {
	m.mu.Lock()
	inst, ok := m.instances[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("instance %s not found", id)
	}
	if proc, running := m.processes[id]; running && proc.IsRunning() {
		inst.Status = StatusConnected
		m.mu.Unlock()
		return nil
	}

	countryCode := inst.CountryCode
	socksPort := inst.SocksPort
	baseDir := m.dataBaseDir
	baseJSON := m.baseConfigJSON
	m.mu.Unlock()

	dirName := strings.ToLower(countryCode)
	if dirName == "" {
		dirName = "auto"
	}
	dataDir := filepath.Join(baseDir, dirName)
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	configPath := filepath.Join(dataDir, "config.json")
	if err := writeInstanceConfigFile(configPath, dataDir, countryCode, socksPort, baseJSON); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	handler := func(n Notice) {
		m.handleNotice(id, n)
	}

	label := fmt.Sprintf("psiphon-%s", countryCode)
	proc := NewProcess(configPath, dataDir, label, handler)

	m.mu.Lock()
	inst.Status = StatusStarting
	m.processes[id] = proc
	m.mu.Unlock()

	if err := proc.Start(); err != nil {
		m.mu.Lock()
		inst.Status = StatusError
		m.mu.Unlock()
		return fmt.Errorf("failed to start psiphon process: %w", err)
	}
	return nil
}

func writeInstanceConfigFile(configPath, dataDir, countryCode string, socksPort int, baseJSON string) error {
	cfg := make(map[string]any)
	if strings.TrimSpace(baseJSON) != "" {
		_ = json.Unmarshal([]byte(baseJSON), &cfg)
	}

	// Sane defaults
	if _, ok := cfg["DisableLocalHTTPProxy"]; !ok {
		cfg["DisableLocalHTTPProxy"] = true
	}
	if _, ok := cfg["EmitDiagnosticNotices"]; !ok {
		cfg["EmitDiagnosticNotices"] = true
	}
	if _, ok := cfg["UseIndistinguishableTLS"]; !ok {
		cfg["UseIndistinguishableTLS"] = true
	}

	// Protected overrides
	cfg["EgressRegion"] = countryCode
	cfg["LocalSocksProxyPort"] = socksPort
	cfg["ListenInterface"] = "lo"
	cfg["DisableLocalSocksProxy"] = false
	cfg["DataRootDirectory"] = dataDir

	body, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, body, 0o600)
}

func (m *Manager) handleNotice(id string, n Notice) {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[id]
	if !ok {
		return
	}

	switch n.NoticeType {
	case "ListeningSocksProxyPort":
		if inst.Status == StatusStarting || inst.Status == StatusStopped {
			inst.Status = StatusStarting
		}
	case "ActiveTunnel":
		inst.Status = StatusConnected
	case "ConnectedServerRegion":
		if reg, ok := n.Data["region"].(string); ok && reg != "" {
			inst.CountryCode = reg
			inst.CountryName = CountryName(reg)
		}
	case "AvailableEgressRegions":
		if regions, ok := n.Data["regions"].([]any); ok {
			for _, r := range regions {
				if rStr, ok := r.(string); ok && rStr != "" {
					m.availableRegions[rStr] = struct{}{}
				}
			}
		}
	case "Error":
		if msg, ok := n.Data["message"].(string); ok && strings.Contains(strings.ToLower(msg), "fatal") {
			inst.Status = StatusError
		}
	}
}

// Stop terminates the running process for an instance.
func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	proc, ok := m.processes[id]
	inst, hasInst := m.instances[id]
	if hasInst {
		inst.Status = StatusStopped
	}
	m.mu.Unlock()

	if ok && proc != nil {
		return proc.Stop()
	}
	return nil
}

// StartAll launches all registered enabled instances.
func (m *Manager) StartAll() error {
	m.mu.RLock()
	ids := make([]string, 0, len(m.instances))
	for id, inst := range m.instances {
		if inst.Enabled {
			ids = append(ids, id)
		}
	}
	m.mu.RUnlock()

	for _, id := range ids {
		if err := m.Start(id); err != nil {
			logger.Errorf("psiphon: start instance %s failed: %v", id, err)
		}
	}
	return nil
}

// StopAll stops all active instances gracefully.
func (m *Manager) StopAll() {
	m.mu.RLock()
	ids := make([]string, 0, len(m.processes))
	for id := range m.processes {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	for _, id := range ids {
		_ = m.Stop(id)
	}
}

// ProbeInstance runs latency and exit IP probes through the instance's SOCKS proxy.
func (m *Manager) ProbeInstance(ctx context.Context, id string) (*ProbeResult, error) {
	m.mu.RLock()
	inst, ok := m.instances[id]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("instance %s not found", id)
	}
	port := inst.SocksPort
	m.mu.RUnlock()

	res := Probe(ctx, port)
	if res.OK {
		m.mu.Lock()
		if cur, ok := m.instances[id]; ok {
			cur.LatencyMs = res.LatencyMs
			if res.ExitIP != "" {
				cur.ExitIP = res.ExitIP
			}
			cur.LastTested = time.Now().UnixMilli()
			if cur.Status == StatusStarting {
				cur.Status = StatusConnected
			}
		}
		m.mu.Unlock()
	}
	return &res, nil
}

// GetAvailableCountries returns static plus dynamically discovered regions.
func (m *Manager) GetAvailableCountries() []Country {
	m.mu.RLock()
	defer m.mu.RUnlock()

	addedMap := make(map[string]bool)
	for _, inst := range m.instances {
		addedMap[inst.CountryCode] = true
	}

	merged := make(map[string]string)
	for k, v := range StaticCountries {
		merged[k] = v
	}
	for r := range m.availableRegions {
		if _, ok := merged[r]; !ok {
			merged[r] = CountryName(r)
		}
	}

	var countries []Country
	for code, name := range merged {
		countries = append(countries, Country{
			Code:      code,
			Name:      name,
			Flag:      CountryFlag(code),
			Added:     addedMap[code],
			Available: true,
		})
	}
	sort.Slice(countries, func(i, j int) bool {
		return countries[i].Name < countries[j].Name
	})
	return countries
}
