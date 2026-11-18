package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUpdateScriptRequiresLocalPackage(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux maintenance script")
	}
	base := t.TempDir()
	panel := filepath.Join(base, "x-ui")
	if err := os.WriteFile(panel, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\""), 0o700); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(base, "with spaces.zip")
	if err := os.WriteFile(pkg, []byte("test fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XUI_MAIN_FOLDER", base)
	out, err := exec.Command("bash", "./update.sh", pkg).CombinedOutput()
	if want := strings.Join([]string{"offline-update", pkg, panel, ""}, "\n"); err != nil || string(out) != want {
		t.Fatalf("offline update command: %s (%v), want %q", out, err, want)
	}
	if out, err := exec.Command("bash", "./update.sh", "https://example.invalid/update.zip").CombinedOutput(); err == nil {
		t.Fatalf("URL was accepted by offline updater: %s", out)
	}
}
