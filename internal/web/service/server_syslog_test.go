//go:build !windows

package service

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// A journalctl that never answers must not hang the Syslog request (#6629).
func TestGetLogsSyslogGivesUpOnAStalledJournalctl(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "journalctl"), []byte("#!/bin/sh\nexec sleep 5\n"), 0o755); err != nil {
		t.Fatalf("write fake journalctl: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	saved := syslogTimeout
	syslogTimeout = 200 * time.Millisecond
	t.Cleanup(func() { syslogTimeout = saved })

	start := time.Now()
	got := (&ServerService{}).GetLogs("10", "err", "true")
	want := []string{"journalctl did not answer in time. Try a smaller line count or a less strict level."}
	if !slices.Equal(got, want) {
		t.Fatalf("GetLogs = %q, want %q", got, want)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("GetLogs waited %v for a stalled journalctl, want about %v", elapsed, syslogTimeout)
	}
}
