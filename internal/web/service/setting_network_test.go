package service

import (
	stdnet "net"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestNetworkTrafficSettingsPersistAndRejectInvalidChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x-ui.db")
	dbtest.InitDB(t, path)
	s := &SettingService{}
	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if settings.NetworkTrafficMode != "legacy" || settings.NetworkTrafficInterface != "" {
		t.Fatalf("missing settings defaults: mode=%q interface=%q", settings.NetworkTrafficMode, settings.NetworkTrafficInterface)
	}
	interfaces, err := stdnet.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(interfaces) == 0 {
		t.Skip("no network interfaces available")
	}
	settings.NetworkTrafficMode, settings.NetworkTrafficInterface = "manual", interfaces[0].Name
	if err := s.UpdateAllSetting(settings, SecretClears{}); err != nil {
		t.Fatal(err)
	}
	if err := database.CloseDB(); err != nil {
		t.Fatal(err)
	}
	if err := database.InitDB(path); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if got.NetworkTrafficMode != "manual" || got.NetworkTrafficInterface != interfaces[0].Name {
		t.Fatalf("restarted settings: mode=%q interface=%q", got.NetworkTrafficMode, got.NetworkTrafficInterface)
	}
	legacyRequest := *got
	legacyRequest.NetworkTrafficMode, legacyRequest.NetworkTrafficInterface = "", ""
	legacyRequest.SubTitle = "Unrelated legacy API update"
	if err := s.UpdateAllSetting(&legacyRequest, SecretClears{}); err != nil {
		t.Fatal(err)
	}
	legacyReload, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if legacyReload.NetworkTrafficMode != "manual" || legacyReload.NetworkTrafficInterface != interfaces[0].Name || legacyReload.SubTitle != "Unrelated legacy API update" {
		t.Fatalf("older API update changed traffic selection: mode=%q interface=%q title=%q", legacyReload.NetworkTrafficMode, legacyReload.NetworkTrafficInterface, legacyReload.SubTitle)
	}
	for _, tc := range []struct{ mode, iface, message string }{
		{"", interfaces[0].Name, "network traffic mode"},
		{"other", interfaces[0].Name, "network traffic mode"},
		{"manual", "", "network traffic interface"},
		{"manual", "does-not-exist-6906", "network traffic interface"},
		{"manual", "eth0\n", "network traffic interface"},
		{"manual", "../eth0", "network traffic interface"},
	} {
		got.NetworkTrafficMode, got.NetworkTrafficInterface = tc.mode, tc.iface
		if err := s.UpdateAllSetting(got, SecretClears{}); err == nil || !strings.Contains(err.Error(), tc.message) {
			t.Fatalf("invalid settings %+v: %v", tc, err)
		}
	}
	reloaded, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.NetworkTrafficMode != "manual" || reloaded.NetworkTrafficInterface != interfaces[0].Name || reloaded.SubTitle != "Unrelated legacy API update" {
		t.Fatalf("invalid update changed persisted selection: mode=%q interface=%q title=%q", reloaded.NetworkTrafficMode, reloaded.NetworkTrafficInterface, reloaded.SubTitle)
	}
	for _, mode := range []string{"auto", "legacy", ""} {
		reloaded.NetworkTrafficMode = mode
		if mode == "" {
			reloaded.NetworkTrafficInterface = ""
		}
		if err := s.UpdateAllSetting(reloaded, SecretClears{}); err != nil {
			t.Fatal(err)
		}
		saved, err := s.GetAllSetting()
		if err != nil {
			t.Fatal(err)
		}
		want := mode
		if want == "" {
			want = "legacy"
		}
		if saved.NetworkTrafficMode != want {
			t.Fatalf("saved %q mode=%q", mode, saved.NetworkTrafficMode)
		}
	}
}

func TestOmittedNetworkSettingsPreserveConcurrentSelection(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}
	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	interfaces, err := stdnet.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(interfaces) == 0 {
		t.Skip("no network interfaces available")
	}
	settings.NetworkTrafficMode, settings.NetworkTrafficInterface = "manual", interfaces[0].Name
	if err := s.UpdateAllSetting(settings, SecretClears{}); err != nil {
		t.Fatal(err)
	}
	settings.NetworkTrafficMode, settings.NetworkTrafficInterface = "", ""
	settings.SubTitle = "Other settings updated"
	db := database.GetDB()
	var change sync.Once
	const callback = "test:concurrent_traffic_selection"
	if err := db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table != "settings" {
			return
		}
		change.Do(func() {
			if err := db.Transaction(func(write *gorm.DB) error {
				if err := write.Model(&model.Setting{}).Where("key = ?", "networkTrafficMode").Update("value", "auto").Error; err != nil {
					return err
				}
				return write.Model(&model.Setting{}).Where("key = ?", "networkTrafficInterface").Update("value", "").Error
			}); err != nil {
				t.Errorf("concurrent update: %v", err)
			}
		})
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callback) })
	if err := s.UpdateAllSetting(settings, SecretClears{}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.NetworkTrafficMode != "auto" || reloaded.NetworkTrafficInterface != "" || reloaded.SubTitle != "Other settings updated" {
		t.Fatalf("concurrent selection lost: mode=%q interface=%q title=%q", reloaded.NetworkTrafficMode, reloaded.NetworkTrafficInterface, reloaded.SubTitle)
	}
}
