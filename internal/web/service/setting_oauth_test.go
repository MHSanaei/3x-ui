package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func clearOAuthEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"XUI_OAUTH_ISSUER", "XUI_OAUTH_CLIENT_ID", "XUI_OAUTH_CLIENT_SECRET",
		"XUI_OAUTH_REDIRECT_URL", "XUI_OAUTH_SCOPES", "XUI_OAUTH_GROUPS_CLAIM",
		"XUI_OAUTH_USERNAME_CLAIM", "XUI_OAUTH_ADMIN_GROUP", "XUI_OAUTH_USER_GROUP",
		"XUI_OAUTH_USER_INBOUND_REMARK", "XUI_OAUTH_USER_TOTAL_GB",
		"XUI_OAUTH_USER_EXPIRY_DAYS", "XUI_OAUTH_USER_LIMIT_IP",
	} {
		t.Setenv(k, "")
	}
}

func TestEffectiveOAuthConfig_EnvOverridesDB(t *testing.T) {
	setupBulkDB(t)
	clearOAuthEnv(t)
	s := &SettingService{}
	if err := s.setString("oauthIssuer", "db-issuer"); err != nil {
		t.Fatal(err)
	}
	if err := s.setString("oauthAdminGroup", "db-admins"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XUI_OAUTH_ISSUER", "env-issuer")

	cfg := s.GetEffectiveOAuthConfig()
	if cfg.Issuer != "env-issuer" {
		t.Errorf("Issuer = %q, want env override 'env-issuer'", cfg.Issuer)
	}
	if cfg.AdminGroup != "db-admins" {
		t.Errorf("AdminGroup = %q, want stored 'db-admins'", cfg.AdminGroup)
	}
}

func TestOAuthEnabledEffective(t *testing.T) {
	setupBulkDB(t)
	clearOAuthEnv(t)
	s := &SettingService{}
	if err := s.setString("oauthIssuer", "i"); err != nil {
		t.Fatal(err)
	}
	if err := s.setString("oauthClientId", "c"); err != nil {
		t.Fatal(err)
	}

	if err := s.setBool("oauthEnable", false); err != nil {
		t.Fatal(err)
	}
	if s.OAuthEnabledEffective() {
		t.Fatal("disabled toggle with stored issuer/client should be inactive")
	}
	if err := s.setBool("oauthEnable", true); err != nil {
		t.Fatal(err)
	}
	if !s.OAuthEnabledEffective() {
		t.Fatal("enabled toggle + issuer/client should be active")
	}

	if err := s.setBool("oauthEnable", false); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XUI_OAUTH_ISSUER", "ei")
	t.Setenv("XUI_OAUTH_CLIENT_ID", "ec")
	if !s.OAuthEnabledEffective() {
		t.Fatal("env issuer+client must force-enable regardless of the stored toggle")
	}
	if !s.OAuthEnvLocks()["oauthEnable"] {
		t.Fatal("env issuer+client must lock the enable toggle")
	}
}

func TestEnforceOauthEnvLocks(t *testing.T) {
	setupBulkDB(t)
	clearOAuthEnv(t)
	s := &SettingService{}
	if err := s.setString("oauthIssuer", "stored-issuer"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XUI_OAUTH_ISSUER", "env-issuer")

	as := &entity.AllSetting{OauthIssuer: "attacker-issuer", OauthAdminGroup: "new-admins"}
	s.enforceOauthEnvLocks(as)
	if as.OauthIssuer != "stored-issuer" {
		t.Errorf("env-locked issuer = %q, want reset to stored 'stored-issuer'", as.OauthIssuer)
	}
	if as.OauthAdminGroup != "new-admins" {
		t.Errorf("non-locked adminGroup = %q, want submitted value preserved", as.OauthAdminGroup)
	}
}

func TestOAuthSettingView_EnvLockAndSecret(t *testing.T) {
	setupBulkDB(t)
	clearOAuthEnv(t)
	s := &SettingService{}
	if err := s.setString("oauthClientSecret", "db-secret"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XUI_OAUTH_ISSUER", "env-issuer")
	t.Setenv("XUI_OAUTH_CLIENT_ID", "env-client")

	view, err := s.GetAllSettingView()
	if err != nil {
		t.Fatalf("GetAllSettingView: %v", err)
	}
	if view.OauthIssuer != "env-issuer" {
		t.Errorf("view issuer = %q, want live env value", view.OauthIssuer)
	}
	if view.OauthClientSecret != "" {
		t.Error("client secret must never be echoed to the view")
	}
	if !view.HasOauthClientSecret {
		t.Error("stored secret should surface as HasOauthClientSecret")
	}
	if !view.OauthEnvLocked["oauthIssuer"] || !view.OauthEnvLocked["oauthEnable"] {
		t.Errorf("env locks missing: %+v", view.OauthEnvLocked)
	}
	if !view.OauthEnable {
		t.Error("env issuer+client should present the enable toggle as ON")
	}
}
