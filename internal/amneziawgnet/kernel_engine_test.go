package amneziawgnet

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
)

func TestKernelEngine_Interface(t *testing.T) {
	var engine Engine = NewKernelEngine()
	if engine.Name() != "kernel" {
		t.Fatalf("expected engine Name()='kernel', got %q", engine.Name())
	}
	if engine.HasRunning() {
		t.Fatal("expected HasRunning()=false on fresh engine")
	}

	diag := engine.Diagnose(1, []amneziawg.Peer{})
	if diag.Running {
		t.Fatal("expected Running=false for unmanaged inbound")
	}
}
