package tuic

import (
	"fmt"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

type managed struct {
	server       *Server
	tag          string
	structuralFP string
	usersFP      string
}

type Manager struct {
	mu           sync.Mutex
	servers      map[int]*managed
	lastStartErr map[int]string
}

var (
	managerInstance *Manager
	managerOnce     sync.Once
)

func GetManager() *Manager {
	managerOnce.Do(func() {
		cleanupLegacySidecar()
		managerInstance = &Manager{
			servers:      make(map[int]*managed),
			lastStartErr: make(map[int]string),
		}
	})
	return managerInstance
}

func (m *Manager) HasRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, mg := range m.servers {
		if mg.server != nil && mg.server.IsRunning() {
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

	if existing, ok := m.servers[inst.Id]; ok && existing != nil {
		if existing.server != nil && existing.server.IsRunning() && existing.structuralFP == structuralFP {
			existing.tag = inst.Tag
			if existing.usersFP != usersFP {
				existing.usersFP = usersFP
				existing.server.UpdateUsers(inst.Clients)
			}
			return nil
		}
		stopManaged(existing)
		delete(m.servers, inst.Id)
	}

	server, err := m.startLocked(inst)
	if err != nil {
		if m.lastStartErr[inst.Id] != err.Error() {
			m.lastStartErr[inst.Id] = err.Error()
			logger.Warningf("tuic: failed to start tuic server for inbound %d (%s): %v", inst.Id, inst.Tag, err)
		}
		return err
	}
	delete(m.lastStartErr, inst.Id)

	m.servers[inst.Id] = &managed{
		server:       server,
		tag:          inst.Tag,
		structuralFP: structuralFP,
		usersFP:      usersFP,
	}
	return nil
}

func (m *Manager) startLocked(inst Instance) (*Server, error) {
	relay := &SocksRelay{
		Addr:     fmt.Sprintf("127.0.0.1:%d", SOCKSPortForInbound(inst.Id)),
		Password: SocksPassword(),
	}
	server, err := NewServer(inst, relay)
	if err != nil {
		return nil, fmt.Errorf("tuic: init server for %d: %w", inst.Id, err)
	}
	if err := server.Start(); err != nil {
		return nil, fmt.Errorf("tuic: start server on %s for %d: %w", inst.BindTo(), inst.Id, err)
	}
	return server, nil
}

func stopManaged(mg *managed) {
	if mg.server != nil {
		_ = mg.server.Close()
	}
}

func (m *Manager) GetActiveClients(window time.Duration) ([]string, []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var emails []string
	var tags []string
	for _, mg := range m.servers {
		if mg.server != nil && mg.server.IsRunning() {
			active := mg.server.GetActiveEmails(window)
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
	for _, mg := range m.servers {
		if mg.server != nil && mg.server.IsRunning() {
			up, down := mg.server.CollectTotalTraffic()
			if up > 0 || down > 0 {
				out = append(out, InboundTrafficDelta{
					Tag:  mg.tag,
					Up:   up,
					Down: down,
				})
			}
		}
	}
	return out
}

func (m *Manager) CollectClientTraffic() []ClientTrafficDelta {
	m.mu.Lock()
	defer m.mu.Unlock()
	var allDeltas []ClientTrafficDelta
	for _, mg := range m.servers {
		if mg.server != nil && mg.server.IsRunning() {
			deltas := mg.server.CollectClientTraffic()
			allDeltas = append(allDeltas, deltas...)
		}
	}
	return allDeltas
}

func (m *Manager) Remove(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeLocked(id)
}

func (m *Manager) removeLocked(id int) {
	if existing, ok := m.servers[id]; ok && existing != nil {
		stopManaged(existing)
		delete(m.servers, id)
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

	for id := range m.servers {
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
	for _, mg := range m.servers {
		stopManaged(mg)
	}
	m.servers = make(map[int]*managed)
}
