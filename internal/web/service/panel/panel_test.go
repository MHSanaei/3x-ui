package panel

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

func writeStatusFile(t *testing.T, path string, runID int64, state string) {
	t.Helper()
	body := fmt.Sprintf(`{"runId":"%d","state":"%s","exitCode":0,"finishedAt":%d}`, runID, state, time.Now().Unix())
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGetUpdateStatus(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	path := config.GetUpdateStatusFilePath()
	svc := &PanelService{}

	if got := svc.GetUpdateStatus(); got.State != updateStatePending {
		t.Fatalf("missing status file: State = %q, want %q", got.State, updateStatePending)
	}

	writeStatusFile(t, path, 1735689600123456789, updateStateSuccess)
	got := svc.GetUpdateStatus()
	if got.RunID != "1735689600123456789" {
		t.Fatalf("RunID = %q, want %q (must round-trip as a decimal string, not a JSON number, or it loses precision past 2^53 in JS)", got.RunID, "1735689600123456789")
	}
	if got.State != updateStateSuccess {
		t.Fatalf("State = %q, want %q", got.State, updateStateSuccess)
	}

	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := svc.GetUpdateStatus(); got.State != updateStatePending {
		t.Fatalf("corrupt status file: State = %q, want %q", got.State, updateStatePending)
	}

	writeStatusFile(t, path, 1, "some-unrecognized-state")
	if got := svc.GetUpdateStatus(); got.State != updateStatePending {
		t.Fatalf("unrecognized state normalizes to pending: State = %q, want %q", got.State, updateStatePending)
	}
}
