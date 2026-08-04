package amneziawgnet

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/amnezia-vpn/amneziawg-go/v3/device"

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
		ListenPort:    58714,
		PrivateKey:    priv,
		PublicKey:     pub,
		Address:       []string{"10.203.0.1/24"},
		MTU:           1420,
		Obfuscation: amneziawg.Obfuscation20{
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
		ListenPort:    58715,
		PrivateKey:    priv,
		PublicKey:     pub,
		Address:       []string{"10.204.0.1/24"},
		MTU:           1420,
		Obfuscation: amneziawg.Obfuscation20{
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

	const forwardedPort = 58930
	m := &Manager{ifaces: map[int]*managed{}}
	inst := amneziawg.Instance{
		Id:            6,
		InterfaceName: "awgtest6",
		ListenPort:    58716,
		PrivateKey:    priv,
		PublicKey:     pub,
		Address:       []string{"10.205.0.1/24"},
		MTU:           1420,
		Obfuscation: amneziawg.Obfuscation20{
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
