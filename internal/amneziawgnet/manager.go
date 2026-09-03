package amneziawgnet

import (
	"fmt"
	"net/netip"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// verboseLoggerIfEnabled returns a real amneziawg-go verbose logger (real
// handshake/keepalive/decrypt-error diagnostics -- the device is otherwise
// completely silent by design, see DeviceOptions' own doc comment) when the
// AMNEZIAWGNET_DEBUG environment variable is set to any non-empty value,
// nil otherwise (NewDevice's own default -- LogLevelSilent -- applies).
// Deliberately opt-in and env-var-gated rather than a permanent log-level
// setting: this device's own protocol-level logging has no per-peer
// filtering, so enabling it on a busy real inbound would be noisy; it's
// meant for exactly this kind of "why did this one handshake go quiet"
// investigation on a low-traffic box.
func verboseLoggerIfEnabled(inboundID int) *device.Logger {
	if os.Getenv("AMNEZIAWGNET_DEBUG") == "" {
		return nil
	}
	return device.NewLogger(device.LogLevelVerbose, fmt.Sprintf("(awg#%d) ", inboundID))
}

// Desired pairs an amneziawg.Instance (the shared, DB-backed shape
// internal/amneziawg's own kernel-module Manager also reconciles toward)
// with this package's own embedded-only DeviceOptions -- the AWG 3.0 fields
// that shared type doesn't carry, see DeviceOptions' doc comment.
type Desired struct {
	Instance amneziawg.Instance
	Options  DeviceOptions
}

// managed is one running embedded interface: the live Device, its UDP relay
// sessions, its open per-client port-forward listeners, the peer lookup
// index built from its current peer list, and enough of its own
// configuration to decide whether a later Ensure call can reconfigure it in
// place or needs to rebuild it from scratch.
type managed struct {
	dev          *Device
	udpRelay     *UDPRelay
	portForwards *PortForwardSet
	// peers is the live snapshot the attached TCP/UDP handlers read; an
	// in-place reconfigure must Store into this same struct, not a new one.
	peers      atomic.Pointer[PeerIndex]
	inst       amneziawg.Instance
	structFP   string
	uapiConfig string
}

func (m *managed) lookupPeer(addr netip.Addr) (amneziawg.Peer, bool) {
	peers := m.peers.Load()
	if peers == nil {
		return amneziawg.Peer{}, false
	}
	return peers.Lookup(addr)
}

func (m *managed) handleUDP(src, dst netip.AddrPort, payload []byte) {
	peer, ok := m.lookupPeer(src.Addr())
	if !ok {
		return
	}
	m.udpRelay.Handle(src, dst, peer.Email, payload)
}

func (m *managed) close() {
	m.portForwards.Close()
	// Cleared before Close: a TCP handler already past lookupPeer (its own
	// goroutine, see AttachTCPForwarder) must fail closed, not resolve a peer.
	m.peers.Store(nil)
	// Stop packet delivery before closing the relay so an in-flight handler
	// cannot publish a new session after the relay has already been swept.
	m.dev.Close()
	m.udpRelay.Close()
}

// Manager owns the set of running embedded AmneziaWG interfaces, keyed by
// inbound id -- the same shape as internal/amneziawg.Manager (GetManager()
// + sync.Once, mu-guarded map, Ensure/Reconcile/StopAll/HasRunning), so a
// caller already familiar with that Manager needs to learn nothing new here.
// Every Device this Manager builds gets its TCP forwarder and UDP handler
// attached automatically (see ensureLocked), relaying into that instance's
// own loopback SOCKS5 inbound (SOCKSPortForInbound/SocksPassword) -- a
// caller only needs to keep calling Ensure/Reconcile with fresh Instance
// data; it doesn't need to know relay.go exists at all.
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
// last apply (skip entirely -- this is the common case on every 10s
// reconcile tick when no admin edit happened, and it MUST actually skip the
// IpcSet call, not just look like it should: amneziawg-go's IpcSet always
// includes replace_peers=true -- see buildUAPIConfig -- and its own
// implementation of that op is device.RemoveAllPeers(), unconditionally,
// even when the new peer list is byte-identical to the old one. A real
// production bug, found via a live test connection that reset every ~10s:
// calling IpcSet on every tick regardless of whether anything changed was
// tearing down every peer's live handshake/session state on every single
// reconcile, so no connection could ever survive past one tick); only
// peers/obfuscation/keys/listen_port changed (reconfigure the existing
// Device in place via IpcSet); or the interface's own address(es)/MTU
// changed (these are fixed at netstack-construction time, so the only
// option is closing the old Device and building a fresh one). This is a
// coarser split than internal/amneziawg's own three-tier noop/reload/
// restart fingerprinting (that one also tracks host-side TPROXY/NDP rules
// this embedded path has no equivalent of) -- correct and sufficient here.
func (m *Manager) ensureLocked(d Desired) error {
	inst, opts := d.Instance, d.Options
	if opts.Logger == nil {
		opts.Logger = verboseLoggerIfEnabled(inst.Id)
	}
	structFP := addressFingerprint(inst)

	cur, exists := m.ifaces[inst.Id]
	// Captured before either branch below: peers/AllowedIPs can change
	// (and so can each peer's IPv6 alias) without the address/MTU
	// fingerprint changing at all, so both the reconfigure-in-place branch
	// and the rebuild branch need to diff IPv6 aliases against whatever
	// this id had before, not just on a rebuild.
	var oldInst amneziawg.Instance
	if exists {
		oldInst = cur.inst
	}

	if exists && cur.structFP == structFP {
		conf, err := buildUAPIConfig(inst, opts)
		if err != nil {
			return fmt.Errorf("amneziawgnet: %w", err)
		}
		// True no-op: the rendered UAPI config -- which already covers every
		// field IpcSet can act on (keys, listen port, obfuscation, AWG 3.0
		// options, the full peer list) -- is byte-identical to what's
		// already live. Comparing the rendered string instead of inst
		// directly means this can never drift out of sync with whatever
		// buildUAPIConfig actually reads, the way a hand-maintained field
		// list could.
		if conf == cur.uapiConfig {
			cur.peers.Store(NewPeerIndex(inst.Peers))
			cur.inst = inst
			applyV6Aliases(diffV6Aliases(oldInst, inst))
			// buildUAPIConfig never reads ForwardedPorts (it's a panel-level
			// concept, not a WireGuard UAPI field), so a ForwardedPorts-only
			// edit renders byte-identical here and takes this exact no-op
			// branch -- without this call, that edit would silently never
			// open/close a listener until some unrelated change also
			// happened to touch this inbound. See
			// TestForwardedPortsOnlyChangeStillReconcilesPortForwards.
			cur.portForwards.Reconcile(inst)
			return nil
		}
		if err := cur.dev.IpcSet(conf); err != nil {
			return fmt.Errorf("amneziawgnet: reconfigure inbound %d: %w", inst.Id, err)
		}
		cur.peers.Store(NewPeerIndex(inst.Peers))
		cur.inst = inst
		cur.uapiConfig = conf
		applyV6Aliases(diffV6Aliases(oldInst, inst))
		cur.portForwards.Reconcile(inst)
		return nil
	}

	if exists {
		cur.close()
		delete(m.ifaces, inst.Id)
	}
	dev, err := newUnconfiguredDevice(inst, opts)
	if err != nil {
		return err
	}

	relay := socksRelayForInstance(inst)
	udpRelay := NewUDPRelay(relay, dev.Stack)
	portForwards := NewPortForwardSet(dev.Stack, inst.Id)
	next := &managed{
		dev:          dev,
		udpRelay:     udpRelay,
		portForwards: portForwards,
		inst:         inst,
		structFP:     structFP,
	}
	next.peers.Store(NewPeerIndex(inst.Peers))
	AttachTCPForwarder(dev.Stack, func(conn *gonet.TCPConn, dest netip.AddrPort) {
		srcAddrPort, err := netip.ParseAddrPort(conn.RemoteAddr().String())
		if err != nil {
			conn.Close()
			return
		}
		// Load the latest immutable peer snapshot without entering the
		// lifecycle lock held while a device reconfigures or closes.
		peer, ok := next.lookupPeer(srcAddrPort.Addr().Unmap())
		if !ok {
			conn.Close()
			return
		}
		relay.RelayTCP(conn, peer.Email, dest)
	})
	AttachUDPHandler(dev.Stack, next.handleUDP)

	// Handlers are registered on dev.Stack above, BEFORE Configure's IpcSet
	// can start any peer's receive goroutine -- see newUnconfiguredDevice's
	// doc comment for why this order (not convenience) is what makes this
	// race-free.
	if err := dev.Configure(inst, opts); err != nil {
		udpRelay.Close()
		portForwards.Close()
		return err
	}
	// dev.Configure already rendered and applied this exact config
	// internally; recomputing it here (cheap, pure, guaranteed to succeed
	// since Configure just proved these inputs are valid) is simpler than
	// threading the string back out of Configure's own signature, and gives
	// the no-op check above a correct baseline to compare the next tick
	// against instead of an empty string.
	conf, _ := buildUAPIConfig(inst, opts)
	next.uapiConfig = conf
	m.ifaces[inst.Id] = next
	applyV6Aliases(diffV6Aliases(oldInst, inst))
	portForwards.Reconcile(inst)
	logger.Infof("amneziawgnet: started embedded interface %s for inbound %d", inst.InterfaceName, inst.Id)
	return nil
}

// socksRelayForInstance derives the loopback SOCKS5 relay address/password
// for inst -- both fully determined by its id and the process-wide
// password (SOCKSPortForInbound/SocksPassword), so no per-instance state
// needs threading through Desired/DeviceOptions for this.
func socksRelayForInstance(inst amneziawg.Instance) SocksRelay {
	return SocksRelay{
		Addr:     fmt.Sprintf("127.0.0.1:%d", SOCKSPortForInbound(inst.Id)),
		Password: SocksPassword(),
	}
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
		applyV6Aliases(diffV6Aliases(cur.inst, amneziawg.Instance{}))
		cur.close()
		delete(m.ifaces, id)
		logger.Infof("amneziawgnet: stopped embedded interface for removed inbound %d", id)
	}
	for _, d := range desired {
		if err := m.ensureLocked(d); err != nil {
			logger.Warningf("amneziawgnet: reconcile failed for inbound %d: %v", d.Instance.Id, err)
		}
	}
}

// Remove tears down inbound id's embedded interface, if any -- mirrors
// internal/amneziawg.Manager.Remove, for a caller that needs to drop a
// single inbound outside a full Reconcile pass (e.g. the immediate-apply
// CRUD path in internal/web/runtime/local.go).
func (m *Manager) Remove(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, exists := m.ifaces[id]
	if !exists {
		return
	}
	applyV6Aliases(diffV6Aliases(cur.inst, amneziawg.Instance{}))
	cur.close()
	delete(m.ifaces, id)
	logger.Infof("amneziawgnet: stopped embedded interface for removed inbound %d", id)
}

// StopAll tears down every managed interface. Called on panel shutdown.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, cur := range m.ifaces {
		applyV6Aliases(diffV6Aliases(cur.inst, amneziawg.Instance{}))
		cur.close()
		delete(m.ifaces, id)
	}
}

// HasRunning reports whether any embedded interface is currently managed.
func (m *Manager) HasRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.ifaces) > 0
}

// Lookup returns the running device and current peer snapshot for diagnostics,
// tests, and other callers outside the packet-delivery path.
func (m *Manager) Lookup(id int) (dev *Device, peers *PeerIndex, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, exists := m.ifaces[id]
	if !exists {
		return nil, nil, false
	}
	return cur.dev, cur.peers.Load(), true
}
