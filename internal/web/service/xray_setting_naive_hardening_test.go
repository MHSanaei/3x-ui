package service

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/naive"
)

func TestValidateNaiveStubRejectsOutOfRangeSettings(t *testing.T) {
	tests := []struct {
		name    string
		setting string
		wantErr string
	}{
		{
			name:    "insecure concurrency",
			setting: `"insecureConcurrency":9`,
			wantErr: "insecureConcurrency must be between 1 and 8",
		},
		{
			name:    "negative tunnel timeout",
			setting: `"tunnelTimeout":-1`,
			wantErr: "tunnelTimeout must be non-negative",
		},
		{
			name:    "negative idle timeout",
			setting: `"idleTimeout":-1`,
			wantErr: "idleTimeout must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := []byte(`{"tag":"naive-test","protocol":"naive","settings":{"proxy":"https://user:pass@example.com:443",` + tt.setting + `}}`)
			err := validateNaiveStub(payload)
			if err == nil {
				t.Fatal("validateNaiveStub() unexpectedly succeeded")
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("validateNaiveStub() error = %q, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestApplyNaiveSyncResultReturnsStartError(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	t.Cleanup(func() {
		naive.GetManager().StopAll()
		_ = database.CloseDB()
	})

	binDir := t.TempDir()
	logDir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", binDir)
	t.Setenv("XUI_LOG_FOLDER", logDir)
	binary := filepath.Join(binDir, "naive")
	if err := os.WriteFile(binary, []byte("not executable"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Create(&model.NaiveOutbound{
		Tag:       "naive-permission-error",
		ProxyURL:  "https://user:pass@example.com:443",
		LocalPort: 30000,
		Enabled:   true,
	}).Error; err != nil {
		t.Fatalf("create Naive record: %v", err)
	}

	err := applyNaiveSyncResult(naiveSyncResult{})
	if err == nil {
		t.Fatal("applyNaiveSyncResult() unexpectedly succeeded")
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("applyNaiveSyncResult() error = %v, want permission error", err)
	}
	if !strings.Contains(err.Error(), "start configured outbounds") {
		t.Fatalf("applyNaiveSyncResult() error = %q, want operation context", err)
	}
}

func TestApplyNaiveSyncResultRemovesOnlyDeletedOutboundLog(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	t.Cleanup(func() {
		naive.GetManager().StopAll()
		_ = database.CloseDB()
	})

	logDir := t.TempDir()
	t.Setenv("XUI_LOG_FOLDER", logDir)
	removed := naive.LogPath("removed")
	kept := naive.LogPath("kept")
	for _, path := range []string{removed, kept} {
		if err := os.WriteFile(path, []byte("test\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := applyNaiveSyncResult(naiveSyncResult{stopped: []string{"removed"}}); err != nil {
		t.Fatalf("applyNaiveSyncResult() error = %v", err)
	}
	if _, err := os.Stat(removed); !os.IsNotExist(err) {
		t.Fatalf("removed outbound log remains: %v", err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Fatalf("another outbound log was removed: %v", err)
	}
}
