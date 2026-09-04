package tuic

import (
	"fmt"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

type managed struct {
	proc         *Process
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

	configBytes, err := GenerateConfig(inst)
	if err != nil {
		return fmt.Errorf("tuic: generate config for %d: %w", inst.Id, err)
	}

	configPath, err := WriteConfigFile(inst.Id, configBytes)
	if err != nil {
		return fmt.Errorf("tuic: write config for %d: %w", inst.Id, err)
	}

	structuralFP := inst.StructuralFingerprint()
	usersFP := inst.UsersFingerprint()

	uuidToEmail := make(map[string]string, len(inst.Clients))
	for _, c := range inst.Clients {
		if c.UUID != "" && c.Email != "" {
			uuidToEmail[c.UUID] = c.Email
		}
	}

	existing, ok := m.procs[inst.Id]
	if ok && existing != nil && existing.proc != nil && existing.proc.IsRunning() {
		if existing.structuralFP == structuralFP && existing.usersFP == usersFP {
			existing.proc.UpdateClients(uuidToEmail)
			return nil
		}
		_ = existing.proc.Stop()
	}

	proc := newProcess(configPath, inst.Tag, uuidToEmail)
	if err := proc.Start(); err != nil {
		errStr := err.Error()
		if m.lastStartErr[inst.Id] != errStr {
			m.lastStartErr[inst.Id] = errStr
			logger.Warningf("tuic: failed to start tuic-server for inbound %d (%s): %v", inst.Id, inst.Tag, err)
		}
		return err
	}
	delete(m.lastStartErr, inst.Id)

	m.procs[inst.Id] = &managed{
		proc:         proc,
		tag:          inst.Tag,
		configPath:   configPath,
		structuralFP: structuralFP,
		usersFP:      usersFP,
	}
	return nil
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
		if mg.proc != nil && mg.proc.IsRunning() {
			deltaUp, deltaDown := mg.proc.CollectTraffic()
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
		if existing.proc != nil && existing.proc.IsRunning() {
			_ = existing.proc.Stop()
		}
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
		if mg.proc != nil && mg.proc.IsRunning() {
			_ = mg.proc.Stop()
		}
		_ = RemoveConfigFile(id)
	}
	m.procs = make(map[int]*managed)
}
