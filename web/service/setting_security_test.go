package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database"
)

func setupSettingTestDB(t *testing.T) {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.CloseDB(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestGetAllSettingViewRedactsSecrets(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}
	if err := s.saveSetting("tgBotToken", "telegram-secret"); err != nil {
		t.Fatal(err)
	}
	if err := s.saveSetting("twoFactorToken", "totp-secret"); err != nil {
		t.Fatal(err)
	}
	if err := s.saveSetting("ldapPassword", "ldap-secret"); err != nil {
		t.Fatal(err)
	}
	if err := s.saveSetting("apiToken", "api-secret"); err != nil {
		t.Fatal(err)
	}

	view, err := s.GetAllSettingView()
	if err != nil {
		t.Fatal(err)
	}
	if view.TgBotToken != "" || view.TwoFactorToken != "" || view.LdapPassword != "" {
		t.Fatalf("settings view leaked secrets: %#v", view)
	}
	if !view.HasTgBotToken || !view.HasTwoFactorToken || !view.HasLdapPassword || !view.HasApiToken {
		t.Fatalf("settings view did not report configured secret flags: %#v", view)
	}
}

func TestUpdateAllSettingPreservesRedactedSecrets(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}
	if err := s.saveSetting("tgBotToken", "telegram-secret"); err != nil {
		t.Fatal(err)
	}
	if err := s.saveSetting("ldapPassword", "ldap-secret"); err != nil {
		t.Fatal(err)
	}
	if err := s.saveSetting("twoFactorEnable", "true"); err != nil {
		t.Fatal(err)
	}
	if err := s.saveSetting("twoFactorToken", "totp-secret"); err != nil {
		t.Fatal(err)
	}

	view, err := s.GetAllSettingView()
	if err != nil {
		t.Fatal(err)
	}
	settings := &view.AllSetting
	if err := s.UpdateAllSetting(settings); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetTgBotToken(); got != "telegram-secret" {
		t.Fatalf("tg token = %q, want preserved secret", got)
	}
	if got, _ := s.GetLdapPassword(); got != "ldap-secret" {
		t.Fatalf("ldap password = %q, want preserved secret", got)
	}
	if got, _ := s.GetTwoFactorToken(); got != "totp-secret" {
		t.Fatalf("2fa token = %q, want preserved secret", got)
	}
}

func TestSanitizePublicHTTPURLBlocksPrivateAddressUnlessAllowed(t *testing.T) {
	if _, err := SanitizePublicHTTPURL("http://127.0.0.1:8080/hook", false); err == nil {
		t.Fatal("expected localhost URL to be blocked")
	}
	if got, err := SanitizePublicHTTPURL("http://127.0.0.1:8080/hook", true); err != nil || got != "http://127.0.0.1:8080/hook" {
		t.Fatalf("allowPrivate result = %q, %v", got, err)
	}
}
