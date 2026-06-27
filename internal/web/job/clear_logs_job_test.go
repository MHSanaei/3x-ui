package job

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeAccessLogConfig points bin/config.json at the given access log path (use
// "none" to disable), so GetAccessLogPath resolves it the way the job does.
func writeAccessLogConfig(t *testing.T, accessPath string) {
	t.Helper()
	binDir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", binDir)
	configData, err := json.Marshal(map[string]any{
		"log": map[string]any{"access": accessPath},
	})
	if err != nil {
		t.Fatalf("marshal xray config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "config.json"), configData, 0o644); err != nil {
		t.Fatalf("write xray config: %v", err)
	}
}

func TestWipeAccessLog_TruncatesEnabledLog(t *testing.T) {
	accessLog := filepath.Join(t.TempDir(), "access.log")
	if err := os.WriteFile(accessLog, []byte("2026/06/23 12:00:00 from tcp:203.0.113.10:443 accepted\n"), 0o644); err != nil {
		t.Fatalf("seed access log: %v", err)
	}
	writeAccessLogConfig(t, accessLog)

	wipeAccessLog()

	info, err := os.Stat(accessLog)
	if err != nil {
		t.Fatalf("access log should still exist: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("access log should be truncated to 0, got %d bytes", info.Size())
	}
}

func TestWipeAccessLog_LeavesDisabledLogAlone(t *testing.T) {
	writeAccessLogConfig(t, "none")

	// Must not panic or create a file literally named "none".
	wipeAccessLog()

	if _, err := os.Stat("none"); err == nil {
		os.Remove("none")
		t.Fatal(`wipeAccessLog must not create a file named "none"`)
	}
}
