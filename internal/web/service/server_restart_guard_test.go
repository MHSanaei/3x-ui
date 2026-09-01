package service

import (
	"errors"
	"strings"
	"testing"
	"time"
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

// A successful claim must release the flag when the restart finishes (here,
// fails on a seeded invalid template), or a later restart stays locked out.
func TestRestartXrayServiceFromPanelReleasesTheFlag(t *testing.T) {
	setupSettingTestDB(t)
	if err := (&SettingService{}).saveSetting("xrayTemplateConfig", "{ not valid json"); err != nil {
		t.Fatalf("seed template: %v", err)
	}

	s := &ServerService{}
	if err := s.RestartXrayServiceFromPanel(); err == nil {
		t.Fatal("RestartXrayServiceFromPanel() with an invalid template = nil, want an error")
	}
	if s.restartInFlight.Load() {
		t.Fatal("RestartXrayServiceFromPanel left restartInFlight true after finishing, want it released")
	}
}

// StartRestartFromPanel must return a runID before its background goroutine
// touches anything -- notably the database, which this test never sets up.
func TestStartRestartFromPanelReturnsBeforeTheRestartRuns(t *testing.T) {
	s := &ServerService{}
	runID, err := s.StartRestartFromPanel()
	if err != nil {
		t.Fatalf("StartRestartFromPanel() = %v, want nil", err)
	}
	if runID == "" {
		t.Fatal("StartRestartFromPanel() returned an empty runID")
	}
	if !s.restartInFlight.Load() {
		t.Fatal("StartRestartFromPanel did not mark a restart in flight")
	}

	// Drain the background goroutine before returning, so its panic-recovery
	// log (it has no database either) doesn't land during a later test.
	deadline := time.Now().Add(2 * time.Second)
	for s.restartInFlight.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
}

// The two restart paths must share one slot: a dashboard click and a
// concurrent synchronous restart (master-node, API token) must not both proceed.
func TestStartRestartFromPanelSharesTheSlotWithTheSyncPath(t *testing.T) {
	s := &ServerService{}
	s.restartInFlight.Store(true)
	t.Cleanup(func() { s.restartInFlight.Store(false) })

	if _, err := s.StartRestartFromPanel(); !errors.Is(err, ErrRestartInFlight) {
		t.Fatalf("StartRestartFromPanel() while a sync restart is in flight = %v, want ErrRestartInFlight", err)
	}
}

// A panic in the background restart (here, from touching a nil database)
// must be recovered and recorded as a failure, not crash the panel process.
func TestStartRestartFromPanelRecoversFromPanicAndRecordsFailure(t *testing.T) {
	s := &ServerService{}
	runID, err := s.StartRestartFromPanel()
	if err != nil {
		t.Fatalf("StartRestartFromPanel() = %v, want nil", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status := s.GetRestartStatus()
		if status.RunID == runID {
			if status.State != "failed" {
				t.Fatalf("recorded state = %q, want %q", status.State, "failed")
			}
			if status.ErrMsg == "" {
				t.Error("recorded a failure with no ErrMsg")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no restart result recorded within the deadline")
}
