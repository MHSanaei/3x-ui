package ha

import (
	"context"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// RecoveryChecker asks the local PostgreSQL for pg_is_in_recovery().
type RecoveryChecker interface {
	IsInRecovery(ctx context.Context) (bool, error)
}

// PatroniChecker asks the local Patroni whether it leads.
type PatroniChecker interface {
	IsLeader(ctx context.Context) (bool, error)
}

// PeerProber pings the peer panels (the HTTP quorum channel).
type PeerProber interface {
	Probe(ctx context.Context) []PeerStatus
}

// StandbyLister lists live streaming standbys of the local primary (the
// replication quorum channel).
type StandbyLister interface {
	StreamingStandbys(ctx context.Context, maxAge time.Duration) ([]string, error)
}

// Probes are the observation sources; only PG is mandatory.
type Probes struct {
	PG       RecoveryChecker
	Patroni  PatroniChecker
	Peers    PeerProber
	Standbys StandbyLister
}

// Self holds this machine's public URLs, handed out while it is master.
type Self struct {
	PanelURL string
	SubURL   string
}

// Snapshot is the role decision plus where clients should go.
type Snapshot struct {
	State
	MasterPanelURL string `json:"master_panel_url,omitempty"`
	MasterSubURL   string `json:"master_sub_url,omitempty"`
}

// Monitor observes the node periodically and runs the role automaton.
type Monitor struct {
	cfg      Config
	self     Self
	probes   Probes
	interval time.Duration
	now      func() time.Time

	mu   sync.RWMutex
	snap Snapshot
	prev State
}

// NewMonitor builds a monitor; interval <= 0 means 2 s.
func NewMonitor(cfg Config, self Self, p Probes, interval time.Duration) *Monitor {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	init := Initial()
	return &Monitor{
		cfg: cfg, self: self, probes: p, interval: interval, now: time.Now,
		prev: init, snap: Snapshot{State: init},
	}
}

// Current returns the latest decision.
func (m *Monitor) Current() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snap
}

// Run ticks until ctx ends, handing every decision to onTick.
func (m *Monitor) Run(ctx context.Context, onTick func(Snapshot)) {
	t := time.NewTicker(m.interval)
	defer t.Stop()
	for {
		snap := m.Tick(ctx)
		if onTick != nil {
			onTick(snap)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Tick runs one observation round and the automaton transition.
func (m *Monitor) Tick(ctx context.Context) Snapshot {
	rctx, cancel := context.WithTimeout(ctx, m.interval)
	defer cancel()
	s, peers := m.observe(rctx)

	m.mu.Lock()
	prev := m.prev
	next := Next(m.cfg, prev, s)
	snap := Snapshot{State: next}
	switch {
	case next.IsMaster():
		snap.MasterPanelURL, snap.MasterSubURL = m.self.PanelURL, m.self.SubURL
	case next.Master != "":
		for _, p := range peers {
			if p.Code == next.Master {
				snap.MasterPanelURL, snap.MasterSubURL = p.PanelURL, p.SubURL
			}
		}
	}
	m.prev, m.snap = next, snap
	m.mu.Unlock()

	if next.Role != prev.Role {
		logger.Warningf("ha: role %s -> %s (reason %q, seen %v, quorum %v, pg_err %v, patroni_err %v)",
			prev.Role, next.Role, next.Reason, next.Seen, next.Quorum, s.PGErr, s.PatroniErr)
	}
	return snap
}

func (m *Monitor) observe(ctx context.Context) (Sample, []PeerStatus) {
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		s     Sample
		peers []PeerStatus
	)
	wg.Go(func() {
		rec, err := m.probes.PG.IsInRecovery(ctx)
		mu.Lock()
		s.InRecovery, s.PGErr = rec, err
		mu.Unlock()
	})
	if m.probes.Patroni != nil {
		wg.Go(func() {
			lead, err := m.probes.Patroni.IsLeader(ctx)
			mu.Lock()
			s.PatroniLeader, s.PatroniErr = lead, err
			mu.Unlock()
		})
	}
	if m.probes.Peers != nil {
		wg.Go(func() {
			got := m.probes.Peers.Probe(ctx)
			mu.Lock()
			defer mu.Unlock()
			peers = got
			for _, p := range got {
				if !p.OK {
					continue
				}
				s.Seen = append(s.Seen, p.Code)
				if p.Role == Master {
					s.MasterHint = p.Code
				}
			}
		})
	}
	if m.probes.Standbys != nil {
		wg.Go(func() {
			// A standby reports every wal_receiver_status_interval (10 s by default).
			names, err := m.probes.Standbys.StreamingStandbys(ctx, max(m.cfg.FenceAfter, 15*time.Second))
			if err != nil {
				logger.Debug("ha: pg_stat_replication:", err)
				return
			}
			mu.Lock()
			s.Seen = append(s.Seen, names...)
			mu.Unlock()
		})
	}
	wg.Wait()
	s.At = m.now()
	return s, peers
}

// Gate turns role decisions into the two process transitions: become master
// once, and give up after having been master.
type Gate struct {
	mu        sync.Mutex
	promoted  chan struct{}
	demoted   chan struct{}
	wasMaster bool
	gaveUp    bool
}

// NewGate builds a gate with both transitions pending.
func NewGate() *Gate {
	return &Gate{promoted: make(chan struct{}), demoted: make(chan struct{})}
}

// Promoted is closed when the node first becomes master.
func (g *Gate) Promoted() <-chan struct{} { return g.promoted }

// Demoted is closed when a promoted node stops being master.
func (g *Gate) Demoted() <-chan struct{} { return g.demoted }

// Observe feeds one decision.
func (g *Gate) Observe(st State) {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch {
	case st.IsMaster() && !g.wasMaster:
		g.wasMaster = true
		close(g.promoted)
	case !st.IsMaster() && g.wasMaster && !g.gaveUp:
		g.gaveUp = true
		close(g.demoted)
	}
}
