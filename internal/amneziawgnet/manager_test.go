package amneziawgnet

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// TestManagerLifecycle exercises Ensure/Reconcile's reconfigure-in-place vs.
// rebuild split (see ensureLocked's doc comment) and Reconcile's stop path,
// using a throwaway Manager rather than the process-wide singleton so this
// test doesn't interact with any other test's state.
func TestManagerLifecycle(t *testing.T) {
	priv, pub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}

	m := &Manager{ifaces: map[int]*managed{}}
	inst := amneziawg.Instance{
		Id:            3,
		InterfaceName: "awgtest3",
		ListenPort:    0,
		PrivateKey:    priv,
		PublicKey:     pub,
		Address:       []string{"10.203.0.1/24"},
		MTU:           1420,
		Obfuscation: amneziawg.Obfuscation31{
			Jc: 4, Jmin: 40, Jmax: 70,
			S1: 20, S2: 30, S3: 20, S4: 20,
		},
	}
	defer m.StopAll()

	if err := m.Ensure(Desired{Instance: inst}); err != nil {
		t.Fatalf("Ensure (create): %v", err)
	}
	if !m.HasRunning() {
		t.Fatal("HasRunning() = false after Ensure created an interface")
	}
	dev1, _, ok := m.Lookup(inst.Id)
	if !ok {
		t.Fatal("Lookup after Ensure: not found")
	}

	// Same Instance again: same address fingerprint, so this should
	// reconfigure the existing Device via IpcSet rather than rebuild it --
	// verify by checking the *Device pointer survived unchanged.
	if err := m.Ensure(Desired{Instance: inst}); err != nil {
		t.Fatalf("Ensure (unchanged): %v", err)
	}
	dev2, _, ok := m.Lookup(inst.Id)
	if !ok {
		t.Fatal("Lookup after second Ensure: not found")
	}
	if dev1 != dev2 {
		t.Error("Ensure with an unchanged Instance rebuilt the Device; expected an in-place reconfigure")
	}

	// Changing the interface address is structural (fixed at netstack
	// construction time) and must force a rebuild -- verify by checking the
	// *Device pointer changed.
	changed := inst
	changed.Address = []string{"10.203.1.1/24"}
	if err := m.Ensure(Desired{Instance: changed}); err != nil {
		t.Fatalf("Ensure (address changed): %v", err)
	}
	dev3, _, ok := m.Lookup(inst.Id)
	if !ok {
		t.Fatal("Lookup after address-changing Ensure: not found")
	}
	if dev3 == dev2 {
		t.Error("Ensure with a changed address reconfigured in place; expected a rebuild")
	}

	// Reconcile with nothing desired stops every managed interface.
	m.Reconcile(nil)
	if m.HasRunning() {
		t.Error("HasRunning() = true after Reconcile([]) should have stopped everything")
	}
	if _, _, ok := m.Lookup(inst.Id); ok {
		t.Error("Lookup succeeded after Reconcile([]) removed the interface")
	}
}

// An inbound with no explicit MTU derives it from S4, so an S4-only edit is
// structural: leave it out of the fingerprint and the netstack keeps the old MTU
// while every client emitter already advertises the new one.
func TestEnsureRebuildsWhenS4ChangesTheDerivedMTU(t *testing.T) {
	priv, pub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}

	tests := []struct {
		name        string
		mtu         int
		wantRebuild bool
	}{
		{"derived MTU", 0, true},
		{"explicit MTU", 1420, false},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{ifaces: map[int]*managed{}}
			defer m.StopAll()

			inst := amneziawg.Instance{
				Id:            9 + i,
				InterfaceName: fmt.Sprintf("awgtest%d", 9+i),
				ListenPort:    0,
				PrivateKey:    priv,
				PublicKey:     pub,
				Address:       []string{"10.209.0.1/24"},
				MTU:           tt.mtu,
				Obfuscation: amneziawg.Obfuscation31{
					Jc: 4, Jmin: 40, Jmax: 70,
					S1: 20, S2: 30, S3: 20, S4: 5,
				},
			}
			if err := m.Ensure(Desired{Instance: inst}); err != nil {
				t.Fatalf("Ensure (create): %v", err)
			}
			before, _, ok := m.Lookup(inst.Id)
			if !ok {
				t.Fatal("Lookup after create: not found")
			}

			edited := inst
			edited.Obfuscation.S4 = 27
			if err := m.Ensure(Desired{Instance: edited}); err != nil {
				t.Fatalf("Ensure (S4 changed): %v", err)
			}
			after, _, ok := m.Lookup(inst.Id)
			if !ok {
				t.Fatal("Lookup after S4 edit: not found")
			}

			if rebuilt := before != after; rebuilt != tt.wantRebuild {
				t.Errorf("S4 5->27 rebuilt the Device = %v, want %v (MTU %d -> %d)",
					rebuilt, tt.wantRebuild,
					amneziawg.EffectiveMTU(inst.MTU, inst.Obfuscation.S4),
					amneziawg.EffectiveMTU(edited.MTU, edited.Obfuscation.S4))
			}
		})
	}
}

func TestManagedUDPHandlerDoesNotWaitForManagerLock(t *testing.T) {
	cur := &managed{udpRelay: NewUDPRelay(SocksRelay{Addr: "invalid"}, nil)}
	cur.peers.Store(NewPeerIndex([]amneziawg.Peer{{
		Email:      "peer@test",
		AllowedIPs: []string{"10.210.0.2/32"},
	}}))
	m := &Manager{}

	done := make(chan struct{})
	m.mu.Lock()
	go func() {
		cur.handleUDP(
			netip.MustParseAddrPort("10.210.0.2:1234"),
			netip.MustParseAddrPort("10.210.0.3:53"),
			[]byte("query"),
		)
		close(done)
	}()

	select {
	case <-done:
		m.mu.Unlock()
	case <-time.After(time.Second):
		m.mu.Unlock()
		t.Fatal("UDP handler blocked on the manager lifecycle lock")
	}
}

// TestEnsureUnchangedInstanceDoesNotResetLivePeers is a regression test for a
// real production bug: an unchanged Ensure call (the common case on every
// 10s AmneziaWGJob reconcile tick when no admin edit happened) was calling
// IpcSet unconditionally. amneziawg-go's IpcSet always includes
// replace_peers=true (see buildUAPIConfig), and its own implementation of
// that op is device.RemoveAllPeers() -- unconditionally, even when the new
// peer list is byte-identical to the old one. That tore down every peer's
// live handshake/session state on every single reconcile tick, so no real
// connection could ever survive past ~10 seconds. Caught via a live test
// connection that reset every ~10s with amneziawg-go's own verbose logging
// enabled (AMNEZIAWGNET_DEBUG) showing "UAPI: Removing all peers" +
// peer "Stopping"/"Starting" on every tick.
//
// Verified here by comparing the *device.Peer pointer LookupPeer returns
// before and after a no-op Ensure: identical pointer proves the peer object
// itself survived (no RemoveAllPeers), not just that some higher-level
// abstraction looks unchanged.
func TestEnsureUnchangedInstanceDoesNotResetLivePeers(t *testing.T) {
	priv, pub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate server keypair: %v", err)
	}
	_, peerPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate peer keypair: %v", err)
	}

	m := &Manager{ifaces: map[int]*managed{}}
	inst := amneziawg.Instance{
		Id:            4,
		InterfaceName: "awgtest4",
		ListenPort:    0,
		PrivateKey:    priv,
		PublicKey:     pub,
		Address:       []string{"10.204.0.1/24"},
		MTU:           1420,
		Obfuscation: amneziawg.Obfuscation31{
			Jc: 4, Jmin: 40, Jmax: 70,
			S1: 20, S2: 30, S3: 20, S4: 20,
		},
		Peers: []amneziawg.Peer{
			{Email: "peer@test", PublicKey: peerPub, AllowedIPs: []string{"10.204.0.2/32"}},
		},
	}
	defer m.StopAll()

	if err := m.Ensure(Desired{Instance: inst}); err != nil {
		t.Fatalf("Ensure (create): %v", err)
	}

	peerPubHex, err := wireguard.KeyToHex(peerPub)
	if err != nil {
		t.Fatalf("KeyToHex: %v", err)
	}
	var npk device.NoisePublicKey
	if err := npk.FromHex(peerPubHex); err != nil {
		t.Fatalf("NoisePublicKey.FromHex: %v", err)
	}

	dev, _, ok := m.Lookup(inst.Id)
	if !ok {
		t.Fatal("Lookup after Ensure: not found")
	}
	peerBefore := dev.LookupPeer(npk)
	if peerBefore == nil {
		t.Fatal("LookupPeer returned nil right after Ensure created the peer")
	}

	// Simulate the reconcile job firing again with byte-identical data --
	// this is what AmneziaWGJob does every 10 seconds regardless of whether
	// anything actually changed.
	if err := m.Ensure(Desired{Instance: inst}); err != nil {
		t.Fatalf("Ensure (unchanged, second tick): %v", err)
	}
	peerAfter := dev.LookupPeer(npk)
	if peerAfter == nil {
		t.Fatal("LookupPeer returned nil after the unchanged Ensure -- peer was removed and never re-added")
	}
	if peerBefore != peerAfter {
		t.Error("unchanged Ensure recreated the peer object (RemoveAllPeers + re-add) -- " +
			"any live handshake/session on this peer would have been reset for no reason")
	}
}

// TestForwardedPortsOnlyChangeStillReconcilesPortForwards is a regression
// test for the Phase 3.6 port-forwarding wiring: buildUAPIConfig never reads
// ForwardedPorts (it's a panel-level concept, not a WireGuard UAPI field),
// so a ForwardedPorts-only edit renders a byte-identical UAPI config and
// takes ensureLocked's true no-op branch -- the exact same branch
// TestEnsureUnchangedInstanceDoesNotResetLivePeers exists to guard, just for
// a different subsystem. Without an explicit portForwards.Reconcile call on
// that branch, a ForwardedPorts-only edit would silently never open (or
// close) a listener until some unrelated change also happened to touch this
// inbound. Verified end to end here: a real host-facing listener must exist
// after the second Ensure call, not just an internal state flag.
func TestForwardedPortsOnlyChangeStillReconcilesPortForwards(t *testing.T) {
	priv, pub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate server keypair: %v", err)
	}
	_, peerPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate peer keypair: %v", err)
	}

	forwardedPort := freePort(t)
	m := &Manager{ifaces: map[int]*managed{}}
	inst := amneziawg.Instance{
		Id:            6,
		InterfaceName: "awgtest6",
		ListenPort:    0,
		PrivateKey:    priv,
		PublicKey:     pub,
		Address:       []string{"10.205.0.1/24"},
		MTU:           1420,
		Obfuscation: amneziawg.Obfuscation31{
			Jc: 4, Jmin: 40, Jmax: 70,
			S1: 20, S2: 30, S3: 20, S4: 20,
		},
		Peers: []amneziawg.Peer{
			{Email: "peer@test", PublicKey: peerPub, AllowedIPs: []string{"10.205.0.2/32"}},
		},
	}
	defer m.StopAll()

	if err := m.Ensure(Desired{Instance: inst}); err != nil {
		t.Fatalf("Ensure (create, no ForwardedPorts yet): %v", err)
	}
	if _, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", forwardedPort), 200*time.Millisecond); err == nil {
		t.Fatal("forwarded port already accepting connections before ForwardedPorts was ever set")
	}

	// Only ForwardedPorts changes -- same keys, same AllowedIPs, same
	// address/MTU, so this must take ensureLocked's true no-op UAPI branch.
	changed := inst
	changed.Peers = []amneziawg.Peer{
		{Email: "peer@test", PublicKey: peerPub, AllowedIPs: []string{"10.205.0.2/32"}, ForwardedPorts: fmt.Sprintf("%d", forwardedPort)},
	}
	if err := m.Ensure(Desired{Instance: changed}); err != nil {
		t.Fatalf("Ensure (ForwardedPorts-only change): %v", err)
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", forwardedPort), 2*time.Second)
	if err != nil {
		t.Fatalf("forwarded port not accepting connections after a ForwardedPorts-only Ensure: %v", err)
	}
	conn.Close()
}

// The stack survives an in-place reconfiguration, so a flow pinned to a removed
// peer's address would otherwise reach whichever client gets that address next.
func TestEnsureDropsFlowsOfReassignedTunnelAddress(t *testing.T) {
	priv, pub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate server keypair: %v", err)
	}
	var peerPubs [3]string
	for i := range peerPubs {
		if _, peerPubs[i], err = wireguard.GenerateWireguardKeypair(); err != nil {
			t.Fatalf("generate peer keypair: %v", err)
		}
	}
	reused, kept := netip.MustParseAddr("10.206.0.2"), netip.MustParseAddr("10.206.0.3")

	m := &Manager{ifaces: map[int]*managed{}}
	inst := amneziawg.Instance{
		Id:            9,
		InterfaceName: "awgtest9",
		ListenPort:    0,
		PrivateKey:    priv,
		PublicKey:     pub,
		Address:       []string{"10.206.0.1/24"},
		MTU:           1420,
		Obfuscation: amneziawg.Obfuscation31{
			Jc: 4, Jmin: 40, Jmax: 70,
			S1: 20, S2: 30, S3: 20, S4: 20,
		},
		Peers: []amneziawg.Peer{
			{Email: "old@test", PublicKey: peerPubs[0], AllowedIPs: []string{reused.String() + "/32"}},
			{Email: "other@test", PublicKey: peerPubs[1], AllowedIPs: []string{kept.String() + "/32"}},
		},
	}
	defer m.StopAll()
	if err := m.Ensure(Desired{Instance: inst}); err != nil {
		t.Fatalf("Ensure (create): %v", err)
	}
	cur := m.ifaces[inst.Id]

	endpointsTo := func(addr netip.Addr) int {
		n := 0
		for _, ep := range cur.dev.Stack.RegisteredEndpoints() {
			if remote, ok := endpointRemoteAddr(ep); ok && remote == addr {
				n++
			}
		}
		return n
	}
	waitEndpointsTo := func(addr netip.Addr, want int) {
		t.Helper()
		for deadline := time.Now().Add(2 * time.Second); endpointsTo(addr) != want; time.Sleep(5 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("endpoints connected to %s = %d, want %d", addr, endpointsTo(addr), want)
			}
		}
	}

	for _, addr := range []netip.Addr{reused, kept} {
		raddr := tunnelFullAddress(addr, 8080)
		udpConn, err := gonet.DialUDP(cur.dev.Stack, nil, &raddr, tunnelNetwork(addr))
		if err != nil {
			t.Fatalf("DialUDP %s: %v", addr, err)
		}
		defer udpConn.Close()

		ctrl, _ := net.Pipe()
		relayConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		if err != nil {
			t.Fatalf("ListenUDP: %v", err)
		}
		cur.udpRelay.sessions[netip.AddrPortFrom(addr, 1234)] = &socks5UDPSession{ctrl: ctrl, udpConn: relayConn}
	}
	dialCtx, cancelDial := context.WithCancel(context.Background())
	dialed := make(chan struct{})
	go func() {
		defer close(dialed)
		if conn, err := gonet.DialContextTCP(dialCtx, cur.dev.Stack, tunnelFullAddress(reused, 8080), tunnelNetwork(reused)); err == nil {
			conn.Close()
		}
	}()
	defer func() { cancelDial(); <-dialed }()
	waitEndpointsTo(reused, 2)
	waitEndpointsTo(kept, 1)

	next := inst
	next.Peers = []amneziawg.Peer{
		{Email: "new@test", PublicKey: peerPubs[2], AllowedIPs: []string{reused.String() + "/32"}},
		inst.Peers[1],
	}
	if err := m.Ensure(Desired{Instance: next}); err != nil {
		t.Fatalf("Ensure (address reassigned): %v", err)
	}
	if m.ifaces[inst.Id] != cur {
		t.Fatal("peer change rebuilt the interface instead of reconfiguring it in place")
	}

	waitEndpointsTo(reused, 0)
	waitEndpointsTo(kept, 1)
	cur.udpRelay.mu.Lock()
	_, reusedSession := cur.udpRelay.sessions[netip.AddrPortFrom(reused, 1234)]
	_, keptSession := cur.udpRelay.sessions[netip.AddrPortFrom(kept, 1234)]
	cur.udpRelay.mu.Unlock()
	if reusedSession {
		t.Error("the removed peer's UDP relay session survived its address being reassigned")
	}
	if !keptSession {
		t.Error("an unaffected peer's UDP relay session was torn down")
	}
}

// TestEnsureHeaderProtectionKeyChangeReconfiguresInPlace is a regression
// test for the Phase 3.7 AWG 3.0 wiring: proves that populating
// Desired.Options with a real HeaderProtectionKey/ContentPaddingAddition
// takes ensureLocked's existing reconfigure-in-place branch (same *Device
// survives, no rebuild) rather than silently doing nothing or forcing an
// unnecessary rebuild -- buildUAPIConfig already rendered these fields
// before this phase, so no manager.go changes were needed, but this proves
// the whole chain (Desired -> DeviceOptions -> buildUAPIConfig -> IpcSet)
// actually works together, not just in isolation.
func TestEnsureHeaderProtectionKeyChangeReconfiguresInPlace(t *testing.T) {
	priv, pub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate server keypair: %v", err)
	}
	headerProtectionKey, err := wireguard.GenerateWireguardPSK()
	if err != nil {
		t.Fatalf("generate header protection key: %v", err)
	}

	m := &Manager{ifaces: map[int]*managed{}}
	inst := amneziawg.Instance{
		Id:            7,
		InterfaceName: "awgtest7",
		ListenPort:    0,
		PrivateKey:    priv,
		PublicKey:     pub,
		Address:       []string{"10.207.0.1/24"},
		MTU:           1420,
		Obfuscation: amneziawg.Obfuscation31{
			Jc: 4, Jmin: 40, Jmax: 70,
			S1: 20, S2: 30, S3: 20, S4: 20,
		},
	}
	defer m.StopAll()

	if err := m.Ensure(Desired{Instance: inst}); err != nil {
		t.Fatalf("Ensure (create, no header protection yet): %v", err)
	}
	dev1, _, ok := m.Lookup(inst.Id)
	if !ok {
		t.Fatal("Lookup after Ensure: not found")
	}

	err = m.Ensure(Desired{
		Instance: inst,
		Options: DeviceOptions{
			HeaderProtectionKey:    headerProtectionKey,
			ContentPaddingAddition: "20-40",
		},
	})
	if err != nil {
		t.Fatalf("Ensure (HeaderProtectionKey-only change): %v", err)
	}
	dev2, _, ok := m.Lookup(inst.Id)
	if !ok {
		t.Fatal("Lookup after second Ensure: not found")
	}
	if dev1 != dev2 {
		t.Error("Ensure with a HeaderProtectionKey-only change rebuilt the Device; expected an in-place IpcSet reconfigure")
	}
}

// TestEnsureRejectsHeaderProtectionKeyWithLowS1S4 proves amneziawg-go's own
// IpcSet backstop really exists independent of the save-time
// ValidateHeaderProtection check in
// internal/web/service/inbound_amneziawg.go -- that web-layer check can be
// bypassed (a node-owned inbound, a direct DB edit), so this confirms a
// malformed config still fails loudly here rather than silently applying a
// broken interface.
func TestEnsureRejectsHeaderProtectionKeyWithLowS1S4(t *testing.T) {
	priv, pub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate server keypair: %v", err)
	}
	headerProtectionKey, err := wireguard.GenerateWireguardPSK()
	if err != nil {
		t.Fatalf("generate header protection key: %v", err)
	}

	m := &Manager{ifaces: map[int]*managed{}}
	inst := amneziawg.Instance{
		Id:            8,
		InterfaceName: "awgtest8",
		ListenPort:    0,
		PrivateKey:    priv,
		PublicKey:     pub,
		Address:       []string{"10.208.0.1/24"},
		MTU:           1420,
		Obfuscation: amneziawg.Obfuscation31{
			Jc: 4, Jmin: 40, Jmax: 70,
			S1: 5, S2: 5, S3: 5, S4: 5, // all below amneziawg-go's own 12-byte minimum
		},
	}
	defer m.StopAll()

	err = m.Ensure(Desired{
		Instance: inst,
		Options:  DeviceOptions{HeaderProtectionKey: headerProtectionKey},
	})
	if err == nil {
		t.Fatal("Ensure must fail: amneziawg-go's own IpcSet rejects header protection with S1-S4 below its minimum")
	}
}
