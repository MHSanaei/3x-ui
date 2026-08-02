package amneziawgnet

import (
	"testing"

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
