package panel

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestStableSnapshot_SaveAndGetInfo(t *testing.T) {
	dbFolder := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbFolder)

	mainFolder := t.TempDir()
	srcBinary := filepath.Join(mainFolder, "x-ui")
	dummyContent := []byte("#!/bin/sh\necho stable-v2.4.9\n")
	if err := os.WriteFile(srcBinary, dummyContent, 0o755); err != nil {
		t.Fatalf("failed to create source binary: %v", err)
	}

	// Before saving snapshot, info should be absent.
	if info, ok := GetStableSnapshotInfo(); ok || info != nil {
		t.Fatalf("GetStableSnapshotInfo() before save = (%v, %v), want (nil, false)", info, ok)
	}

	// Save snapshot of current stable installation.
	if err := SaveStableSnapshot(mainFolder, "v2.4.9"); err != nil {
		t.Fatalf("SaveStableSnapshot failed: %v", err)
	}

	info, ok := GetStableSnapshotInfo()
	if !ok || info == nil {
		t.Fatalf("GetStableSnapshotInfo() after save = (%v, %v), want valid info and true", info, ok)
	}
	if info.Version != "v2.4.9" {
		t.Errorf("info.Version = %q, want %q", info.Version, "v2.4.9")
	}
	if info.Arch != runtime.GOARCH {
		t.Errorf("info.Arch = %q, want %q", info.Arch, runtime.GOARCH)
	}
	if info.CreatedAt <= 0 {
		t.Errorf("info.CreatedAt = %d, want > 0", info.CreatedAt)
	}

	snapshotBin := filepath.Join(dbFolder, "snapshots", "stable", "x-ui")
	data, err := os.ReadFile(snapshotBin)
	if err != nil {
		t.Fatalf("failed to read snapshot binary: %v", err)
	}
	if string(data) != string(dummyContent) {
		t.Fatalf("snapshot binary content = %q, want %q", string(data), string(dummyContent))
	}
}

func TestStableSnapshot_Restore(t *testing.T) {
	dbFolder := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbFolder)

	mainFolder := t.TempDir()
	srcBinary := filepath.Join(mainFolder, "x-ui")
	stableContent := []byte("#!/bin/sh\necho stable-v2.4.9\n")
	if err := os.WriteFile(srcBinary, stableContent, 0o755); err != nil {
		t.Fatalf("failed to create source binary: %v", err)
	}

	if err := SaveStableSnapshot(mainFolder, "v2.4.9"); err != nil {
		t.Fatalf("SaveStableSnapshot failed: %v", err)
	}

	// Simulate upgrading to dev binary.
	devContent := []byte("#!/bin/sh\necho dev-latest\n")
	if err := os.WriteFile(srcBinary, devContent, 0o755); err != nil {
		t.Fatalf("failed to overwrite with dev binary: %v", err)
	}

	// Restore snapshot back to mainFolder.
	if err := RestoreStableSnapshot(mainFolder); err != nil {
		t.Fatalf("RestoreStableSnapshot failed: %v", err)
	}

	restoredData, err := os.ReadFile(srcBinary)
	if err != nil {
		t.Fatalf("failed to read restored binary: %v", err)
	}
	if string(restoredData) != string(stableContent) {
		t.Fatalf("restored content = %q, want %q", string(restoredData), string(stableContent))
	}

	fi, err := os.Stat(srcBinary)
	if err != nil {
		t.Fatalf("failed to stat restored binary: %v", err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm()&0o111 == 0 {
		t.Fatalf("restored binary is not executable: mode = %v", fi.Mode().Perm())
	}
}

func TestStableSnapshot_CorruptOrMissing(t *testing.T) {
	dbFolder := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbFolder)
	mainFolder := t.TempDir()

	// Restore without snapshot should fail.
	if err := RestoreStableSnapshot(mainFolder); err == nil {
		t.Fatal("RestoreStableSnapshot() with missing snapshot succeeded, want error")
	}

	snapshotDir := filepath.Join(dbFolder, "snapshots", "stable")
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}

	// Invalid version.json.
	if err := os.WriteFile(filepath.Join(snapshotDir, "version.json"), []byte("invalid json"), 0o644); err != nil {
		t.Fatalf("failed to write corrupt version.json: %v", err)
	}
	if _, ok := GetStableSnapshotInfo(); ok {
		t.Fatal("GetStableSnapshotInfo() with corrupt JSON returned ok=true, want false")
	}

	// Valid JSON but missing binary.
	validJSON := []byte(`{"version":"v2.4.9","createdAt":1700000000,"arch":"amd64"}`)
	if err := os.WriteFile(filepath.Join(snapshotDir, "version.json"), validJSON, 0o644); err != nil {
		t.Fatalf("failed to write valid version.json: %v", err)
	}
	if _, ok := GetStableSnapshotInfo(); ok {
		t.Fatal("GetStableSnapshotInfo() with missing binary returned ok=true, want false")
	}
}

func TestStableSnapshot_SaveMissingSource(t *testing.T) {
	dbFolder := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbFolder)
	mainFolder := t.TempDir()

	if err := SaveStableSnapshot(mainFolder, "v2.4.9"); err == nil {
		t.Fatal("SaveStableSnapshot() with missing source binary succeeded, want error")
	}
}
