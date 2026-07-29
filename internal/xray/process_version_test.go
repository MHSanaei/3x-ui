package xray

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestRefreshVersionTimesOut(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is Unix-only")
	}

	dir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", dir)
	binaryPath := filepath.Join(dir, GetBinaryName())
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\nexec sleep 1\n"), 0o700); err != nil {
		t.Fatalf("write xray fixture: %v", err)
	}

	previousTimeout := xrayVersionTimeout
	xrayVersionTimeout = 20 * time.Millisecond
	t.Cleanup(func() { xrayVersionTimeout = previousTimeout })

	p := newProcess(&Config{})
	started := time.Now()
	p.refreshVersion()
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("refreshVersion duration = %s, want under 500ms", elapsed)
	}
	if got := p.GetXrayVersion(); got != "Unknown" {
		t.Fatalf("version = %q, want Unknown", got)
	}
}
