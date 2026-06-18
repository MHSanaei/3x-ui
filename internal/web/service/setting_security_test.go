package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
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
	if err := s.saveSetting("smtpPassword", "smtp-secret"); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Create(&model.ApiToken{Name: "test", Token: "api-secret", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}

	view, err := s.GetAllSettingView()
	if err != nil {
		t.Fatal(err)
	}
	if view.TgBotToken != "" || view.TwoFactorToken != "" || view.LdapPassword != "" || view.SmtpPassword != "" {
		t.Fatalf("settings view leaked secrets: %#v", view)
	}
	if !view.HasTgBotToken || !view.HasTwoFactorToken || !view.HasLdapPassword || !view.HasApiToken || !view.HasSmtpPassword {
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
	if err := s.saveSetting("smtpPassword", "smtp-secret"); err != nil {
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
	if got, _ := s.GetSmtpPassword(); got != "smtp-secret" {
		t.Fatalf("smtp password = %q, want preserved secret", got)
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

func TestSeedSubJsonTemplateIfEmpty(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}
	const builtin = `{"remarks":"","inbounds":[]}`
	const custom = `{"remarks":"mine","inbounds":[]}`

	if err := s.SeedSubJsonTemplateIfEmpty(builtin); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSubJsonTemplate()
	if err != nil || got != builtin {
		t.Fatalf("first seed = %q, %v; want builtin", got, err)
	}

	if err := s.SeedSubJsonTemplateIfEmpty(`{"ignored":true}`); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetSubJsonTemplate()
	if err != nil || got != builtin {
		t.Fatalf("second seed must not overwrite, got %q", got)
	}

	if err := s.saveSetting("subJsonTemplate", custom); err != nil {
		t.Fatal(err)
	}
	if err := s.SeedSubJsonTemplateIfEmpty(builtin); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetSubJsonTemplate()
	if err != nil || got != custom {
		t.Fatalf("custom template must be preserved, got %q", got)
	}
}

func TestAllSettingCheckValidSubJsonTemplate(t *testing.T) {
	base := entity.AllSetting{WebPort: 2053, SubPort: 2096, TimeLocation: "Local"}
	valid := base
	valid.SubJsonTemplate = `{"dns":{}}`
	if err := valid.CheckValid(); err != nil {
		t.Fatalf("valid template: %v", err)
	}
	invalid := base
	invalid.SubJsonTemplate = `{not json}`
	if err := invalid.CheckValid(); err == nil {
		t.Fatal("expected invalid subJsonTemplate to fail CheckValid")
	}
	for _, raw := range []string{`null`, `[]`, `"x"`} {
		bad := base
		bad.SubJsonTemplate = raw
		if err := bad.CheckValid(); err == nil {
			t.Fatalf("expected non-object subJsonTemplate %q to fail CheckValid", raw)
		}
	}
	empty := base
	if err := empty.CheckValid(); err != nil {
		t.Fatalf("empty template should be allowed: %v", err)
	}
}
