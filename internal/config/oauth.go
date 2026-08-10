package config

import (
	"os"
	"strconv"
	"strings"
)

// OAuthConfig holds the OIDC login configuration, sourced entirely from the
// environment so a deployment stays declarative (no DB/UI state).
type OAuthConfig struct {
	Issuer        string
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	Scopes        []string
	GroupsClaim   string
	UsernameClaim string

	AdminGroup string
	UserGroups []string

	UserInboundRemarks []string
	UserFlow           string
	UserTotalGB        int64
	UserExpiryDays     int
	UserLimitIP        int
}

// OAuthEnabled reports whether OIDC login is configured. The issuer and client
// id are the minimum required to run the flow.
func OAuthEnabled() bool {
	return envStr("XUI_OAUTH_ISSUER") != "" && envStr("XUI_OAUTH_CLIENT_ID") != ""
}

// oauthEnvKeys maps a settings json key to the XUI_OAUTH_* env var that overrides it.
var oauthEnvKeys = map[string]string{
	"oauthIssuer":            "XUI_OAUTH_ISSUER",
	"oauthClientId":          "XUI_OAUTH_CLIENT_ID",
	"oauthClientSecret":      "XUI_OAUTH_CLIENT_SECRET",
	"oauthRedirectUrl":       "XUI_OAUTH_REDIRECT_URL",
	"oauthScopes":            "XUI_OAUTH_SCOPES",
	"oauthGroupsClaim":       "XUI_OAUTH_GROUPS_CLAIM",
	"oauthUsernameClaim":     "XUI_OAUTH_USERNAME_CLAIM",
	"oauthAdminGroup":        "XUI_OAUTH_ADMIN_GROUP",
	"oauthUserGroup":         "XUI_OAUTH_USER_GROUP",
	"oauthUserInboundRemark": "XUI_OAUTH_USER_INBOUND_REMARK",
	"oauthUserFlow":          "XUI_OAUTH_USER_FLOW",
	"oauthUserTotalGB":       "XUI_OAUTH_USER_TOTAL_GB",
	"oauthUserExpiryDays":    "XUI_OAUTH_USER_EXPIRY_DAYS",
	"oauthUserLimitIP":       "XUI_OAUTH_USER_LIMIT_IP",
}

// OAuthEnvValues returns the XUI_OAUTH_* values that are set, keyed by their
// settings json key. The settings layer gives these precedence over stored
// values and locks the matching UI fields read-only.
func OAuthEnvValues() map[string]string {
	m := make(map[string]string, len(oauthEnvKeys))
	for key, env := range oauthEnvKeys {
		if v := envStr(env); v != "" {
			m[key] = v
		}
	}
	return m
}

// GetOAuthConfig reads the XUI_OAUTH_* environment into an OAuthConfig,
// applying defaults for the OIDC scopes and claim names.
func GetOAuthConfig() OAuthConfig {
	return OAuthConfig{
		Issuer:        envStr("XUI_OAUTH_ISSUER"),
		ClientID:      envStr("XUI_OAUTH_CLIENT_ID"),
		ClientSecret:  envStr("XUI_OAUTH_CLIENT_SECRET"),
		RedirectURL:   envStr("XUI_OAUTH_REDIRECT_URL"),
		Scopes:        envList("XUI_OAUTH_SCOPES", []string{"openid", "profile", "email", "groups"}),
		GroupsClaim:   envStrOr("XUI_OAUTH_GROUPS_CLAIM", "groups"),
		UsernameClaim: envStrOr("XUI_OAUTH_USERNAME_CLAIM", "email"),

		AdminGroup: envStr("XUI_OAUTH_ADMIN_GROUP"),
		UserGroups: envList("XUI_OAUTH_USER_GROUP", nil),

		UserInboundRemarks: envList("XUI_OAUTH_USER_INBOUND_REMARK", nil),
		UserFlow:           envStr("XUI_OAUTH_USER_FLOW"),
		UserTotalGB:        int64(envInt("XUI_OAUTH_USER_TOTAL_GB", 0)),
		UserExpiryDays:     envInt("XUI_OAUTH_USER_EXPIRY_DAYS", 0),
		UserLimitIP:        envInt("XUI_OAUTH_USER_LIMIT_IP", 0),
	}
}

func envStr(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func envStrOr(key, fallback string) string {
	if v := envStr(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := envStr(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// envList splits a comma-separated env value, trimming blanks. An unset value
// yields the fallback so callers can distinguish "use defaults" from "empty".
func envList(key string, fallback []string) []string {
	v := envStr(key)
	if v == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}
