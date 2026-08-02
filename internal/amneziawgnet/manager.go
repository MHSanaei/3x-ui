package amneziawgnet

import (
	"fmt"
	"strings"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// Desired pairs an amneziawg.Instance (the shared, DB-backed shape
// internal/amneziawg's own kernel-module Manager also reconciles toward)
// with this package's own embedded-only DeviceOptions -- the AWG 3.0 fields
// that shared type doesn't carry, see DeviceOptions' doc comment.
type Desired struct {
	Instance amneziawg.Instance
	Options  DeviceOptions
}

// managed is one running embedded interface: the live Device, the peer
// lookup index built from its current peer list, and enough of its own
// configuration to decide whether a later Ensure call can reconfigure it in
// place or needs to rebuild it from scratch.
type managed struct {
	dev      *Device
	peers    *PeerIndex
	inst     amneziawg.Instance
	structFP string
}

// Manager owns the set of running embedded AmneziaWG interfaces, keyed by
// inbound id -- the same shape as internal/amneziawg.Manager (GetManager()
// + sync.Once, mu-guarded map, Ensure/Reconcile/StopAll/HasRunning), so a
// caller already familiar with that Manager needs to learn nothing new here.
// Unlike that Manager, this one doesn't attach any traffic handling by
// itself: Ensure/Reconcile only bring each Instance's Device up to date.
// Attaching a forwarder/UDP handler (see forwarder.go / udp.go) using the
// Device and PeerIndex returned by Lookup is left to the caller -- today a
// test harness, later the Phase 2 SOCKS5 relay wiring -- since this package
// doesn't yet know what that handler should do with a recovered connection.
type Manager struct {
	mu     sync.Mutex
	ifaces map[int]*managed
}

var (
	managerOnce sync.Once
	manager     *Manager
)

// GetManager returns the process-wide embedded-AmneziaWG manager singleton.
func GetManager() *Manager {
	managerOnce.Do(func() {
		manager = &Manager{ifaces: map[int]*managed{}}
	})
	return manager
}

// Ensure brings inbound d.Instance.Id's embedded interface to the state
// d describes, creating it if it doesn't exist yet. A no-op only when
// nothing has changed since the last successful Ensure/Reconcile.
func (m *Manager) Ensure(d Desired) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ensureLocked(d)
}

// ensureLocked decides between three actions: nothing changed since the
// last apply (skip entirely); only peers/obfuscation/keys/listen_port
// changed (reconfigure the existing Device in place via IpcSet, which
// already sends replace_peers=true -- see buildUAPIConfig -- so removed
// peers are dropped correctly without a full rebuild); or the interface's
// own address(es)/MTU changed (these are fixed at netstack-construction
// time, so the only option is closing the old Device and building a fresh
// one). This is a coarser split than internal/amneziawg's own three-tier
// noop/reload/restart fingerprinting (that one also tracks host-side
// TPROXY/NDP rules this embedded path has no equivalent of) -- correct and
// sufficient for Phase 1; revisit only if reconcile frequency at real scale
// makes the address/MTU rebuild path worth avoiding too.
func (m *Manager) ensureLocked(d Desired) error {
	inst, opts := d.Instance, d.Options
	structFP := addressFingerprint(inst)

	cur, exists := m.ifaces[inst.Id]
	if exists && cur.structFP == structFP {
		conf, err := buildUAPIConfig(inst, opts)
		if err != nil {
			return fmt.Errorf("amneziawgnet: %w", err)
		}
		if err := cur.dev.IpcSet(conf); err != nil {
			return fmt.Errorf("amneziawgnet: reconfigure inbound %d: %w", inst.Id, err)
		}
		cur.peers = NewPeerIndex(inst.Peers)
		cur.inst = inst
		return nil
	}

	if exists {
		cur.dev.Close()
		delete(m.ifaces, inst.Id)
	}
	dev, err := NewDevice(inst, opts)
	if err != nil {
		return err
	}
	m.ifaces[inst.Id] = &managed{
		dev:      dev,
		peers:    NewPeerIndex(inst.Peers),
		inst:     inst,
		structFP: structFP,
	}
	logger.Infof("amneziawgnet: started embedded interface %s for inbound %d", inst.InterfaceName, inst.Id)
	return nil
}

// addressFingerprint captures the two Instance fields that can't be changed
// on a running Device via IpcSet alone (they're fixed when the gVisor
// netstack is built) -- everything else (keys, listen port, obfuscation,
// AWG 3.0 options, peers) amneziawg-go's own UAPI can hot-reconfigure.
func addressFingerprint(inst amneziawg.Instance) string {
	return fmt.Sprintf("%d|%s", inst.MTU, strings.Join(inst.Address, ","))
}

// Reconcile brings every desired instance's embedded interface up to date
// and stops any managed interface whose inbound is no longer desired --
// mirroring internal/amneziawg.Manager.Reconcile's per-tick contract.
func (m *Manager) Reconcile(desired []Desired) {
	m.mu.Lock()
	defer m.mu.Unlock()

	want := make(map[int]struct{}, len(desired))
	for _, d := range desired {
		want[d.Instance.Id] = struct{}{}
	}
	for id, cur := range m.ifaces {
		if _, ok := want[id]; ok {
			continue
		}
		cur.dev.Close()
		delete(m.ifaces, id)
		logger.Infof("amneziawgnet: stopped embedded interface for removed inbound %d", id)
	}
	for _, d := range desired {
		if err := m.ensureLocked(d); err != nil {
			logger.Warningf("amneziawgnet: reconcile failed for inbound %d: %v", d.Instance.Id, err)
		}
	}
}

// StopAll tears down every managed interface. Called on panel shutdown.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, cur := range m.ifaces {
		cur.dev.Close()
		delete(m.ifaces, id)
	}
}

// HasRunning reports whether any embedded interface is currently managed.
func (m *Manager) HasRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.ifaces) > 0
}

// Lookup returns the running Device and PeerIndex for inbound id, if any --
// for a caller that wants to attach its own forwarder/handler (a test
// harness today, the Phase 2 SOCKS5 relay wiring later) once the interface
// is up.
func (m *Manager) Lookup(id int) (dev *Device, peers *PeerIndex, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, exists := m.ifaces[id]
	if !exists {
		return nil, nil, false
	}
	return cur.dev, cur.peers, true
}
