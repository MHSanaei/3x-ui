package amneziawgnet

import (
	"errors"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

type failingEngine struct{}

func (f *failingEngine) Name() string { return "failing-kernel" }
func (f *failingEngine) Ensure(d Desired) error {
	return errors.New("simulated kernel module failure")
}
func (f *failingEngine) Remove(inboundID int) {}
func (f *failingEngine) StopAll()             {}
func (f *failingEngine) HasRunning() bool     { return false }
func (f *failingEngine) Diagnose(inboundID int, peers []amneziawg.Peer) Diagnostics {
	return Diagnostics{}
}

func TestManager_EngineFallback(t *testing.T) {
	priv, pub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}

	inst := amneziawg.Instance{
		Id:            99,
		InterfaceName: "awgtest99",
		ListenPort:    58799,
		PrivateKey:    priv,
		PublicKey:     pub,
		Address:       []string{"10.209.0.1/24"},
		MTU:           1420,
	}

	m := &Manager{ifaces: map[int]*managed{}}
	defer m.StopAll()

	// With kernel engine returning error, ensure it falls back cleanly to embedded engine
	m.kernelEngine = &failingEngine{}

	if err := m.Ensure(Desired{Instance: inst}); err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}

	if !m.HasRunning() {
		t.Fatal("expected interface to be running after fallback")
	}

	diag := m.Diagnose(inst.Id, inst.Peers)
	if !diag.Running {
		t.Fatal("expected diagnostics Running=true")
	}
	if diag.Engine != "embedded" {
		t.Fatalf("expected Engine='embedded' after fallback, got %q", diag.Engine)
	}
}
