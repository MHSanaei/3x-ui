package runtime

import (
	"errors"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
)

type Manager struct {
	local Runtime

	mu      sync.RWMutex
	remotes map[int]*Remote
}

func NewManager(localDeps LocalDeps) *Manager {
	return &Manager{
		local:   NewLocal(localDeps),
		remotes: make(map[int]*Remote),
	}
}

func (m *Manager) RuntimeFor(nodeID *int) (Runtime, error) {
	if nodeID == nil {
		return m.local, nil
	}
	m.mu.RLock()
	if rt, ok := m.remotes[*nodeID]; ok {
		m.mu.RUnlock()
		return rt, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if rt, ok := m.remotes[*nodeID]; ok {
		return rt, nil
	}
	n, err := loadNode(*nodeID)
	if err != nil {
		return nil, err
	}
	if !n.Enable {
		return nil, errors.New("node " + n.Name + " is disabled")
	}
	rt := NewRemote(n)
	m.remotes[*nodeID] = rt
	return rt, nil
}

func (m *Manager) Local() Runtime { return m.local }

func (m *Manager) RemoteFor(node *model.Node) (*Remote, error) {
	if node == nil {
		return nil, errors.New("node is nil")
	}
	m.mu.RLock()
	if rt, ok := m.remotes[node.Id]; ok {
		m.mu.RUnlock()
		return rt, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if rt, ok := m.remotes[node.Id]; ok {
		return rt, nil
	}
	rt := NewRemote(node)
	m.remotes[node.Id] = rt
	return rt, nil
}

func (m *Manager) InvalidateNode(nodeID int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.remotes, nodeID)
}

func loadNode(id int) (*model.Node, error) {
	db := database.GetDB()
	n := &model.Node{}
	if err := db.Model(model.Node{}).Where("id = ?", id).First(n).Error; err != nil {
		return nil, err
	}
	return n, nil
}

var (
	managerMu sync.RWMutex
	manager   *Manager
)

func SetManager(m *Manager) {
	managerMu.Lock()
	defer managerMu.Unlock()
	manager = m
}

func GetManager() *Manager {
	managerMu.RLock()
	defer managerMu.RUnlock()
	return manager
}
