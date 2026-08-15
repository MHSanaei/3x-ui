package database

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
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
	if runtime.GOOS == "windows" {
		t.Skip("fail2ban shell fixtures are Unix-only")
	}
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

func TestResetIpLimitsKeepsConfiguredLimitsWhenProbeFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fail2ban shell fixtures are Unix-only")
	}
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	if err := InitDB(config.GetDBPath()); err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })
	if err := db.Where("seeder_name = ?", "ResetIpLimitNoFail2ban").Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatalf("clear seeder history: %v", err)
	}
	settings, err := json.Marshal(map[string]any{"clients": []any{map[string]any{"email": "kept@example.test", "limitIp": 6}}})
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	inbound := model.Inbound{Remark: "kept", Settings: string(settings)}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	record := model.ClientRecord{Email: "kept@example.test", LimitIP: 2}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("create client record: %v", err)
	}
	stubFail2banClient(t, 1)
	if err := resetIpLimitsWithoutFail2ban(); err != nil {
		t.Fatalf("reset: %v", err)
	}
	var gotInbound model.Inbound
	if err := db.First(&gotInbound, inbound.Id).Error; err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(gotInbound.Settings), &got); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	clients := got["clients"].([]any)
	if limit := clients[0].(map[string]any)["limitIp"]; limit != float64(6) {
		t.Fatalf("inbound limitIp = %v, want 6", limit)
	}
	if err := db.First(&record, record.Id).Error; err != nil {
		t.Fatalf("reload client record: %v", err)
	}
	if record.LimitIP != 2 {
		t.Fatalf("client record limitIp = %d, want 2", record.LimitIP)
	}
	var count int64
	if err := db.Model(&model.HistoryOfSeeders{}).Where("seeder_name = ?", "ResetIpLimitNoFail2ban").Count(&count).Error; err != nil {
		t.Fatalf("count seeder history: %v", err)
	}
	if count != 0 {
		t.Fatalf("seeder history rows = %d, want 0", count)
	}
}
