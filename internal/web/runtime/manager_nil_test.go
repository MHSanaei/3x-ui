package runtime

import "testing"

// A panel that has not wired a runtime manager yet — a CLI path, or a request
// that arrives while the panel is still starting — must get an error instead
// of a nil-pointer panic.
func TestManagerRuntimeForWithoutAManagerReportsAnError(t *testing.T) {
	var m *Manager

	rt, err := m.RuntimeFor(nil)
	if err == nil {
		t.Fatal("RuntimeFor must report an error when no manager is wired")
	}
	if rt != nil {
		t.Fatalf("RuntimeFor returned runtime %q with no manager", rt.Name())
	}
}

func TestManagerRuntimeForWithoutAManagerReportsAnErrorForANode(t *testing.T) {
	var m *Manager
	nodeID := 7

	rt, err := m.RuntimeFor(&nodeID)
	if err == nil {
		t.Fatal("RuntimeFor must report an error when no manager is wired")
	}
	if rt != nil {
		t.Fatalf("RuntimeFor returned runtime %q with no manager", rt.Name())
	}
}
