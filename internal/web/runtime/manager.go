package runtime

import (
	"errors"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

type NodeEgressResolver interface {
	NodeEgressProxyURL(nodeID int) string
}

type Manager struct {
	local Runtime

	mu             sync.RWMutex
	remotes        map[int]*Remote
	overrides      map[int]Runtime // test-only: forces RuntimeFor to return a stub
	localOverride  Runtime         // test-only: forces RuntimeFor(nil) to return a stub
	egressResolver NodeEgressResolver
}

func NewManager(localDeps LocalDeps) *Manager {
	return &Manager{
		local:   NewLocal(localDeps),
		remotes: make(map[int]*Remote),
	}
}

// SetRuntimeOverride makes RuntimeFor(nodeID) return rt instead of building a
// real Remote. Test seam for exercising node-dispatch paths without a network
// node; pass nil rt to clear.
func (m *Manager) SetRuntimeOverride(nodeID int, rt Runtime) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rt == nil {
		delete(m.overrides, nodeID)
		return
	}
	if m.overrides == nil {
		m.overrides = make(map[int]Runtime)
	}
	m.overrides[nodeID] = rt
}

// SetLocalRuntimeOverride makes RuntimeFor(nil) return rt instead of the real
// local runtime. Test seam for exercising the local dispatch path (MTProto
// sidecar, local Xray) without a running child process; pass nil rt to clear.
func (m *Manager) SetLocalRuntimeOverride(rt Runtime) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.localOverride = rt
}

func (m *Manager) SetNodeEgressResolver(r NodeEgressResolver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.egressResolver = r
}

func (m *Manager) NodeEgressProxyURL(nodeID int) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.egressResolver == nil {
		return ""
	}
	return m.egressResolver.NodeEgressProxyURL(nodeID)
}

func (m *Manager) RuntimeFor(nodeID *int) (Runtime, error) {
	if nodeID == nil {
		m.mu.RLock()
		if m.localOverride != nil {
			rt := m.localOverride
			m.mu.RUnlock()
			return rt, nil
		}
		m.mu.RUnlock()
		return m.local, nil
	}
	m.mu.RLock()
	if rt, ok := m.overrides[*nodeID]; ok {
		m.mu.RUnlock()
		return rt, nil
	}
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
	rt := NewRemote(n, m.egressResolver)
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
		if sameRemoteIdentity(rt.node, node) {
			m.mu.RUnlock()
			return rt, nil
		}
		m.mu.RUnlock()
	} else {
		m.mu.RUnlock()
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if rt, ok := m.remotes[node.Id]; ok {
		if sameRemoteIdentity(rt.node, node) {
			return rt, nil
		}
	} else {
		rt := NewRemote(cloneRemoteNode(node), m.egressResolver)
		m.remotes[node.Id] = rt
		return rt, nil
	}
	rt := NewRemote(cloneRemoteNode(node), m.egressResolver)
	m.remotes[node.Id] = rt
	return rt, nil
}

func cloneRemoteNode(n *model.Node) *model.Node {
	if n == nil {
		return nil
	}
	clone := *n
	if n.InboundTags != nil {
		clone.InboundTags = append([]string(nil), n.InboundTags...)
	}
	return &clone
}

func sameRemoteIdentity(a, b *model.Node) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Id == b.Id &&
		a.Scheme == b.Scheme &&
		a.Address == b.Address &&
		a.Port == b.Port &&
		a.BasePath == b.BasePath &&
		a.ApiToken == b.ApiToken &&
		a.AllowPrivateAddress == b.AllowPrivateAddress &&
		a.TlsVerifyMode == b.TlsVerifyMode &&
		a.PinnedCertSha256 == b.PinnedCertSha256 &&
		a.OutboundTag == b.OutboundTag
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
