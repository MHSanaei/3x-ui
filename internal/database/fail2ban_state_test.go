package database

import (
	"os"
	"path/filepath"
	"testing"
)

// stubFail2banClient puts a fail2ban-client on PATH whose exit code the test picks.
func stubFail2banClient(t *testing.T, exitCode int) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "fail2ban-client")
	body := "#!/bin/sh\nexit " + string(rune('0'+exitCode)) + "\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	t.Setenv("PATH", dir)
}

func TestFail2banEnforcementStateSeparatesAbsentFromUnrunnable(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if got, _ := fail2banEnforcementState(); got != fail2banAbsent {
			t.Fatalf("state = %v, want fail2banAbsent", got)
		}
	})

	t.Run("present and runnable", func(t *testing.T) {
		stubFail2banClient(t, 0)
		if got, _ := fail2banEnforcementState(); got != fail2banEnforcing {
			t.Fatalf("state = %v, want fail2banEnforcing", got)
		}
	})

	t.Run("present but failing", func(t *testing.T) {
		stubFail2banClient(t, 1)
		got, err := fail2banEnforcementState()
		if got != fail2banUnknown {
			t.Fatalf("state = %v, want fail2banUnknown", got)
		}
		if err == nil {
			t.Fatal("want the probe error, got nil")
		}
	})
}
