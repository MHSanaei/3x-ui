package tuic

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

type managed struct {
	proc         *Process
	relay        *udpRelay
	tag          string
	configPath   string
	structuralFP string
	usersFP      string
}

type Manager struct {
	mu           sync.Mutex
	procs        map[int]*managed
	lastStartErr map[int]string
}

var (
	managerInstance *Manager
	managerOnce     sync.Once
)

func GetManager() *Manager {
	managerOnce.Do(func() {
		managerInstance = &Manager{
			procs:        make(map[int]*managed),
			lastStartErr: make(map[int]string),
		}
		if n := killStrayTuicProcesses(GetBinaryPath()); n > 0 {
			logger.Warningf("tuic: terminated %d orphaned tuic-server process(es) from a previous run", n)
		}
	})
	return managerInstance
}

func (m *Manager) HasRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, mg := range m.procs {
		if mg.proc != nil && mg.proc.IsRunning() {
			return true
		}
	}
	return false
}

func (m *Manager) Ensure(inst Instance) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ensureLocked(inst)
}

func (m *Manager) ensureLocked(inst Instance) error {
	if len(inst.Clients) == 0 {
		m.removeLocked(inst.Id)
		return nil
	}

	structuralFP := inst.StructuralFingerprint()
	usersFP := inst.UsersFingerprint()

	uuidToEmail := make(map[string]string, len(inst.Clients))
	for _, c := range inst.Clients {
		if c.UUID != "" && c.Email != "" {
			uuidToEmail[c.UUID] = c.Email
		}
	}

	if existing, ok := m.procs[inst.Id]; ok && existing != nil {
		if existing.proc != nil && existing.proc.IsRunning() &&
			existing.structuralFP == structuralFP && existing.usersFP == usersFP {
			existing.tag = inst.Tag
			existing.proc.UpdateClients(uuidToEmail)
			return nil
		}
		stopManaged(existing)
		delete(m.procs, inst.Id)
	}

	proc, relay, configPath, err := m.startLocked(inst, uuidToEmail)
	if err != nil {
		if m.lastStartErr[inst.Id] != err.Error() {
			m.lastStartErr[inst.Id] = err.Error()
			logger.Warningf("tuic: failed to start tuic-server for inbound %d (%s): %v", inst.Id, inst.Tag, err)
		}
		return err
	}
	delete(m.lastStartErr, inst.Id)

	m.procs[inst.Id] = &managed{
		proc:         proc,
		relay:        relay,
		tag:          inst.Tag,
		configPath:   configPath,
		structuralFP: structuralFP,
		usersFP:      usersFP,
	}
	return nil
}

func (m *Manager) startLocked(inst Instance, uuidToEmail map[string]string) (*Process, *udpRelay, string, error) {
	port, err := freeLoopbackUDPPort()
	if err != nil {
		return nil, nil, "", fmt.Errorf("tuic: pick sidecar port for %d: %w", inst.Id, err)
	}
	upstream := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port}
	configBytes, err := GenerateConfig(inst, net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, nil, "", fmt.Errorf("tuic: generate config for %d: %w", inst.Id, err)
	}
	configPath, err := WriteConfigFile(inst.Id, configBytes)
	if err != nil {
		return nil, nil, "", fmt.Errorf("tuic: write config for %d: %w", inst.Id, err)
	}
	relay, err := startUDPRelay(inst.BindTo(), upstream, relayFlowIdle)
	if err != nil {
		_ = RemoveConfigFile(inst.Id)
		return nil, nil, "", fmt.Errorf("tuic: listen on %s for %d: %w", inst.BindTo(), inst.Id, err)
	}
	proc := newProcess(configPath, inst.Tag, uuidToEmail)
	if err := proc.Start(); err != nil {
		relay.Close()
		_ = RemoveConfigFile(inst.Id)
		return nil, nil, "", err
	}
	return proc, relay, configPath, nil
}

func stopManaged(mg *managed) {
	if mg.proc != nil && mg.proc.IsRunning() {
		_ = mg.proc.Stop()
	}
	mg.relay.Close()
}

func (m *Manager) GetActiveClients(window time.Duration) ([]string, []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var emails []string
	var tags []string
	for _, mg := range m.procs {
		if mg.proc != nil && mg.proc.IsRunning() {
			active := mg.proc.GetActiveEmails(window)
			if len(active) > 0 {
				emails = append(emails, active...)
				tags = append(tags, mg.tag)
			}
		}
	}
	return emails, tags
}

type InboundTrafficDelta struct {
	Tag  string
	Up   int64
	Down int64
}

func (m *Manager) CollectTraffic() []InboundTrafficDelta {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []InboundTrafficDelta
	for _, mg := range m.procs {
		if mg.relay != nil && mg.proc != nil && mg.proc.IsRunning() {
			deltaUp, deltaDown := mg.relay.CollectTraffic()
			if deltaUp > 0 || deltaDown > 0 {
				out = append(out, InboundTrafficDelta{
					Tag:  mg.tag,
					Up:   deltaUp,
					Down: deltaDown,
				})
			}
		}
	}
	return out
}

func (m *Manager) Remove(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeLocked(id)
}

func (m *Manager) removeLocked(id int) {
	if existing, ok := m.procs[id]; ok && existing != nil {
		stopManaged(existing)
		_ = RemoveConfigFile(id)
		delete(m.procs, id)
		delete(m.lastStartErr, id)
	}
}

func (m *Manager) Reconcile(desired []Instance) {
	m.mu.Lock()
	defer m.mu.Unlock()

	desiredMap := make(map[int]Instance, len(desired))
	for _, inst := range desired {
		desiredMap[inst.Id] = inst
	}

	for id := range m.procs {
		if _, ok := desiredMap[id]; !ok {
			m.removeLocked(id)
		}
	}

	for _, inst := range desired {
		_ = m.ensureLocked(inst)
	}
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, mg := range m.procs {
		stopManaged(mg)
		_ = RemoveConfigFile(id)
	}
	m.procs = make(map[int]*managed)
}
