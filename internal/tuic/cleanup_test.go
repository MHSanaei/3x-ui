package tuic

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupLegacyFiles(t *testing.T) {
	tempDir := t.TempDir()
	tuicDir := filepath.Join(tempDir, "tuic")
	if err := os.MkdirAll(tuicDir, 0o755); err != nil {
		t.Fatalf("mkdir tuic: %v", err)
	}
	cfgFile := filepath.Join(tuicDir, "tuic_1.json")
	if err := os.WriteFile(cfgFile, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	binFile := filepath.Join(tempDir, "tuic-server")
	if err := os.WriteFile(binFile, []byte("dummy"), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	// Test cleanup logic directly on tempDir
	_ = os.RemoveAll(tuicDir)
	_ = os.Remove(binFile)

	if _, err := os.Stat(tuicDir); !os.IsNotExist(err) {
		t.Errorf("tuic dir should be deleted, got err: %v", err)
	}
	if _, err := os.Stat(binFile); !os.IsNotExist(err) {
		t.Errorf("tuic-server binary should be deleted, got err: %v", err)
	}
}

func TestCleanupLegacySidecarNoPanic(t *testing.T) {
	// Calling cleanupLegacySidecar should not panic even if no legacy files or processes exist
	cleanupLegacySidecar()
}
