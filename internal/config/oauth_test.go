package config

import (
	"reflect"
	"testing"
)

func TestOAuthEnabled(t *testing.T) {
	tests := []struct {
		name   string
		issuer string
		client string
		want   bool
	}{
		{"both set", "https://idp.example", "cid", true},
		{"missing client", "https://idp.example", "", false},
		{"missing issuer", "", "cid", false},
		{"neither", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XUI_OAUTH_ISSUER", tt.issuer)
			t.Setenv("XUI_OAUTH_CLIENT_ID", tt.client)
			if got := OAuthEnabled(); got != tt.want {
				t.Fatalf("OAuthEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetOAuthConfigDefaults(t *testing.T) {
	for _, k := range []string{
		"XUI_OAUTH_SCOPES", "XUI_OAUTH_GROUPS_CLAIM", "XUI_OAUTH_USERNAME_CLAIM",
		"XUI_OAUTH_USER_GROUP", "XUI_OAUTH_USER_TOTAL_GB", "XUI_OAUTH_USER_EXPIRY_DAYS",
		"XUI_OAUTH_USER_LIMIT_IP",
	} {
		t.Setenv(k, "")
	}
	cfg := GetOAuthConfig()
	if want := []string{"openid", "profile", "email", "groups"}; !reflect.DeepEqual(cfg.Scopes, want) {
		t.Errorf("Scopes = %v, want %v", cfg.Scopes, want)
	}
	if cfg.GroupsClaim != "groups" {
		t.Errorf("GroupsClaim = %q, want %q", cfg.GroupsClaim, "groups")
	}
	if cfg.UsernameClaim != "email" {
		t.Errorf("UsernameClaim = %q, want %q", cfg.UsernameClaim, "email")
	}
	if cfg.UserGroups != nil {
		t.Errorf("UserGroups = %v, want nil", cfg.UserGroups)
	}
}

func TestGetOAuthConfigParsing(t *testing.T) {
	t.Setenv("XUI_OAUTH_ISSUER", "https://idp.example ")
	t.Setenv("XUI_OAUTH_SCOPES", "openid, groups ,, email")
	t.Setenv("XUI_OAUTH_USER_GROUP", "vpn-users, staff")
	t.Setenv("XUI_OAUTH_ADMIN_GROUP", "admins")
	t.Setenv("XUI_OAUTH_USER_INBOUND_REMARK", "VLESS-443, Reality ")
	t.Setenv("XUI_OAUTH_USER_TOTAL_GB", "50")
	t.Setenv("XUI_OAUTH_USER_EXPIRY_DAYS", "30")
	t.Setenv("XUI_OAUTH_USER_LIMIT_IP", "3")

	cfg := GetOAuthConfig()
	if cfg.Issuer != "https://idp.example" {
		t.Errorf("Issuer = %q, want trimmed", cfg.Issuer)
	}
	if want := []string{"openid", "groups", "email"}; !reflect.DeepEqual(cfg.Scopes, want) {
		t.Errorf("Scopes = %v, want %v", cfg.Scopes, want)
	}
	if want := []string{"vpn-users", "staff"}; !reflect.DeepEqual(cfg.UserGroups, want) {
		t.Errorf("UserGroups = %v, want %v", cfg.UserGroups, want)
	}
	if want := []string{"VLESS-443", "Reality"}; !reflect.DeepEqual(cfg.UserInboundRemarks, want) {
		t.Errorf("UserInboundRemarks = %v, want %v", cfg.UserInboundRemarks, want)
	}
	if cfg.AdminGroup != "admins" {
		t.Errorf("AdminGroup = %q, want %q", cfg.AdminGroup, "admins")
	}
	if cfg.UserTotalGB != 50 || cfg.UserExpiryDays != 30 || cfg.UserLimitIP != 3 {
		t.Errorf("limits = %d/%d/%d, want 50/30/3", cfg.UserTotalGB, cfg.UserExpiryDays, cfg.UserLimitIP)
	}
}

func TestEnvIntFallbackOnGarbage(t *testing.T) {
	t.Setenv("XUI_OAUTH_USER_LIMIT_IP", "not-a-number")
	if got := GetOAuthConfig().UserLimitIP; got != 0 {
		t.Fatalf("UserLimitIP = %d, want fallback 0", got)
	}
}
