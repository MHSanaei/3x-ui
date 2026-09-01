package service

import (
	"strings"
	"testing"
)

// A double-click on the panel's Restart button used to queue behind
// XrayService's lock and then run a second, redundant stop-then-start.
func TestRestartXrayServiceFromPanelRejectsConcurrentCall(t *testing.T) {
	s := &ServerService{}
	s.restartInFlight.Store(true)
	t.Cleanup(func() { s.restartInFlight.Store(false) })

	t.Run("rejects with the expected message", func(t *testing.T) {
		err := s.RestartXrayServiceFromPanel()
		if err == nil || strings.TrimSpace(err.Error()) != "a restart is already in progress" {
			t.Fatalf("RestartXrayServiceFromPanel() = %v, want %q", err, "a restart is already in progress")
		}
	})

	t.Run("does not clear the in-flight restart's flag", func(t *testing.T) {
		_ = s.RestartXrayServiceFromPanel()
		if !s.restartInFlight.Load() {
			t.Fatal("a rejected call cleared restartInFlight, want the running restart's flag untouched")
		}
	})
}
