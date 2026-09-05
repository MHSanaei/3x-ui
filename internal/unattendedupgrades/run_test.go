package unattendedupgrades

import (
	"strings"
	"testing"
	"time"
)

func TestTail(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"shorter than max", "hello", 10, "hello"},
		{"exactly max", "hello", 5, "hello"},
		{"longer than max keeps the end", "0123456789", 4, "6789"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tail(tc.in, tc.max); got != tc.want {
				t.Errorf("tail(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
		})
	}
}

func TestStaleRunning(t *testing.T) {
	fresh := RunStatus{State: RunRunning, StartedAt: time.Now().Unix()}
	if staleRunning(fresh) {
		t.Error("staleRunning(fresh) = true, want false")
	}
	old := RunStatus{State: RunRunning, StartedAt: time.Now().Add(-runTimeout - time.Hour).Unix()}
	if !staleRunning(old) {
		t.Error("staleRunning(old) = false, want true")
	}
}

func TestGetRunStatusNeverRun(t *testing.T) {
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	status := GetRunStatus()
	if status.State != RunPending {
		t.Errorf("GetRunStatus() with no prior run = %+v, want State = RunPending", status)
	}
}

func TestGetRunStatusReadsBackTerminalState(t *testing.T) {
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	written := RunStatus{RunID: "123", State: RunSuccess, StartedAt: 100, EndedAt: 200, Tail: "all good"}
	if err := writeRunStatus(written); err != nil {
		t.Fatalf("writeRunStatus: %v", err)
	}
	got := GetRunStatus()
	if got != written {
		t.Errorf("GetRunStatus() = %+v, want %+v", got, written)
	}
}

func TestGetRunStatusStaleRunningReportsAsFailed(t *testing.T) {
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	stale := RunStatus{
		RunID:     "abandoned",
		State:     RunRunning,
		StartedAt: time.Now().Add(-runTimeout - time.Hour).Unix(),
	}
	if err := writeRunStatus(stale); err != nil {
		t.Fatalf("writeRunStatus: %v", err)
	}
	got := GetRunStatus()
	if got.State != RunFailed {
		t.Errorf("GetRunStatus() for a stale running record: State = %q, want %q", got.State, RunFailed)
	}
	if got.Tail == "" {
		t.Error("GetRunStatus() for a stale running record left Tail empty, want an explanation")
	}
}

func TestRunNowRejectsConcurrentRun(t *testing.T) {
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	inProgress := RunStatus{RunID: "already-going", State: RunRunning, StartedAt: time.Now().Unix()}
	if err := writeRunStatus(inProgress); err != nil {
		t.Fatalf("writeRunStatus: %v", err)
	}
	_, err := RunNow()
	if err == nil {
		t.Fatal("RunNow() while a run is already in progress returned nil error, want a rejection")
	}
	if !strings.Contains(err.Error(), "already in progress") {
		t.Errorf("RunNow() error = %q, want it to mention a run already in progress", err.Error())
	}
}
