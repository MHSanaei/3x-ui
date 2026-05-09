package runtime

import (
	"errors"
	"sync"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
)

// Manager is the entry point for service code that needs a Runtime.
// One singleton lives in the package-level `manager` var, set at
// server bootstrap (web.go calls SetManager once). InboundService and
// friends read it via GetManager().
//
// Local runs forever; Remotes are built lazily per nodeID and cached.
// Cache invalidation runs on node Update/Delete (NodeService hooks
// InvalidateNode) so a token rotation surfaces the next call.
type Manager struct {
	local Runtime

	mu      sync.RWMutex
	remotes map[int]*Remote
}

// NewManager wires the singleton with the deps Local needs. The runtime
// package can't import service so the caller (web.go) supplies the
// callbacks that bridge into XrayService.
func NewManager(localDeps LocalDeps) *Manager {
	return &Manager{
		local:   NewLocal(localDeps),
		remotes: make(map[int]*Remote),
	}
}

// RuntimeFor picks the right adapter for an inbound based on NodeID.
// Returns local when nodeID is nil; otherwise looks up the node row
// (or returns the cached Remote for it). The caller does not need to
// know which kind they got — that's the point of the abstraction.
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

	// Cache miss — load the node row and build a Remote. We re-check
	// under the write lock to avoid duplicate construction under load.
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

// Local returns the singleton local runtime. Used by code that needs
// to operate on the panel's own xray regardless of which inbound it
// came from (e.g. on-demand restart from the UI).
func (m *Manager) Local() Runtime { return m.local }

// RemoteFor returns the Remote adapter for an already-loaded node row.
// Differs from RuntimeFor in two ways: it skips the DB lookup (caller
// hands in the node), and it returns the concrete *Remote so callers
// like NodeTrafficSyncJob can reach FetchTrafficSnapshot, which the
// Runtime interface doesn't expose.
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

// InvalidateNode drops the cached Remote for nodeID so the next
// RuntimeFor call rebuilds it from the (possibly updated) node row.
// Called from NodeService.Update / Delete.
func (m *Manager) InvalidateNode(nodeID int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.remotes, nodeID)
}

// loadNode reads a node row directly from the DB. Kept package-local
// to avoid pulling NodeService into the runtime — service depends on
// runtime, not the other way around.
func loadNode(id int) (*model.Node, error) {
	db := database.GetDB()
	n := &model.Node{}
	if err := db.Model(model.Node{}).Where("id = ?", id).First(n).Error; err != nil {
		return nil, err
	}
	return n, nil
}

// Singleton wiring -------------------------------------------------------

var (
	managerMu sync.RWMutex
	manager   *Manager
)

// SetManager installs the process-wide Manager. web.go calls this once
// during NewServer. Tests can call it again with a stub.
func SetManager(m *Manager) {
	managerMu.Lock()
	defer managerMu.Unlock()
	manager = m
}

// GetManager returns the installed Manager, or nil before SetManager
// has run. Callers should treat nil as "still booting" — the existing
// behaviour for code paths that only run on the local engine continues
// to work via a pre-wired fallback set up in init() below.
func GetManager() *Manager {
	managerMu.RLock()
	defer managerMu.RUnlock()
	return manager
}
