package stats

import (
	"context"
	"sync"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/features/stats"
)

// Manager is an implementation of stats.Manager.
type Manager struct {
	access    sync.RWMutex
	counters  map[string]*Counter
	onlineMap map[string]*OnlineMap
	channels  map[string]*Channel
	running   bool
}

// NewManager creates an instance of Statistics Manager.
func NewManager(ctx context.Context, config *Config) (*Manager, error) {
	m := &Manager{
		counters:  make(map[string]*Counter),
		onlineMap: make(map[string]*OnlineMap),
		channels:  make(map[string]*Channel),
	}

	return m, nil
}

// Type implements common.HasType.
func (*Manager) Type() interface{} {
	return stats.ManagerType()
}

// RegisterCounter implements stats.Manager.
func (m *Manager) RegisterCounter(name string) (stats.Counter, error) {
	m.access.Lock()
	defer m.access.Unlock()

	if _, found := m.counters[name]; found {
		return nil, errors.New("Counter ", name, " already registered.")
	}
	errors.LogDebug(context.Background(), "create new counter ", name)
	c := new(Counter)
	m.counters[name] = c
	return c, nil
}

// UnregisterCounter implements stats.Manager.
func (m *Manager) UnregisterCounter(name string) error {
	m.access.Lock()
	defer m.access.Unlock()

	if _, found := m.counters[name]; found {
		errors.LogDebug(context.Background(), "remove counter ", name)
		delete(m.counters, name)
	}
	return nil
}

// GetCounter implements stats.Manager.
func (m *Manager) GetCounter(name string) stats.Counter {
	m.access.RLock()
	defer m.access.RUnlock()

	if c, found := m.counters[name]; found {
		return c
	}
	return nil
}

// VisitCounters calls visitor function on all managed counters.
func (m *Manager) VisitCounters(visitor func(string, stats.Counter) bool) {
	m.access.RLock()
	defer m.access.RUnlock()

	for name, c := range m.counters {
		if !visitor(name, c) {
			break
		}
	}
}

// RegisterOnlineMap implements stats.Manager.
func (m *Manager) RegisterOnlineMap(name string) (stats.OnlineMap, error) {
	m.access.Lock()
	defer m.access.Unlock()

	if _, found := m.onlineMap[name]; found {
		return nil, errors.New("onlineMap ", name, " already registered.")
	}
	errors.LogDebug(context.Background(), "create new onlineMap ", name)
	om := NewOnlineMap()
	m.onlineMap[name] = om
	return om, nil
}

// UnregisterOnlineMap implements stats.Manager.
func (m *Manager) UnregisterOnlineMap(name string) error {
	m.access.Lock()
	defer m.access.Unlock()

	if _, found := m.onlineMap[name]; found {
		errors.LogDebug(context.Background(), "remove onlineMap ", name)
		delete(m.onlineMap, name)
	}
	return nil
}

// GetOnlineMap implements stats.Manager.
func (m *Manager) GetOnlineMap(name string) stats.OnlineMap {
	m.access.RLock()
	defer m.access.RUnlock()

	if om, found := m.onlineMap[name]; found {
		return om
	}
	return nil
}

// RegisterChannel implements stats.Manager.
func (m *Manager) RegisterChannel(name string) (stats.Channel, error) {
	m.access.Lock()
	defer m.access.Unlock()

	if _, found := m.channels[name]; found {
		return nil, errors.New("Channel ", name, " already registered.")
	}
	errors.LogDebug(context.Background(), "create new channel ", name)
	c := NewChannel(&ChannelConfig{BufferSize: 64, Blocking: false})
	m.channels[name] = c
	if m.running {
		return c, c.Start()
	}
	return c, nil
}

// UnregisterChannel implements stats.Manager.
func (m *Manager) UnregisterChannel(name string) error {
	m.access.Lock()
	defer m.access.Unlock()

	if c, found := m.channels[name]; found {
		errors.LogDebug(context.Background(), "remove channel ", name)
		delete(m.channels, name)
		return c.Close()
	}
	return nil
}

// GetChannel implements stats.Manager.
func (m *Manager) GetChannel(name string) stats.Channel {
	m.access.RLock()
	defer m.access.RUnlock()

	if c, found := m.channels[name]; found {
		return c
	}
	return nil
}

// GetAllOnlineUsers implements stats.Manager.
func (m *Manager) GetAllOnlineUsers() []string {
	m.access.RLock()
	defer m.access.RUnlock()

	usersOnline := make([]string, 0, len(m.onlineMap))
	for user, onlineMap := range m.onlineMap {
		if onlineMap.Count() > 0 {
			usersOnline = append(usersOnline, user)
		}
	}

	return usersOnline
}

// Start implements common.Runnable.
func (m *Manager) Start() error {
	m.access.Lock()
	defer m.access.Unlock()
	m.running = true
	errs := []error{}
	for _, channel := range m.channels {
		if err := channel.Start(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return errors.Combine(errs...)
	}
	return nil
}

// Close implement common.Closable.
func (m *Manager) Close() error {
	m.access.Lock()
	defer m.access.Unlock()
	m.running = false
	errs := []error{}
	for name, channel := range m.channels {
		errors.LogDebug(context.Background(), "remove channel ", name)
		delete(m.channels, name)
		if err := channel.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return errors.Combine(errs...)
	}
	return nil
}

func init() {
	common.Must(common.RegisterConfig((*Config)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return NewManager(ctx, config.(*Config))
	}))
}
