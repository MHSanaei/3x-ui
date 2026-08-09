package service

import (
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func oauthSplitCsv(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// effectiveOauthString returns the env value when the matching XUI_OAUTH_* var is
// set, otherwise the stored setting.
func (s *SettingService) effectiveOauthString(env map[string]string, key string, get func() (string, error)) string {
	if v, ok := env[key]; ok {
		return v
	}
	v, _ := get()
	return v
}

func (s *SettingService) effectiveOauthInt(env map[string]string, key string, get func() (int, error)) int {
	if v, ok := env[key]; ok {
		n, _ := strconv.Atoi(strings.TrimSpace(v))
		return n
	}
	v, _ := get()
	return v
}

// GetEffectiveOAuthConfig merges the XUI_OAUTH_* environment (priority) over the
// stored settings, applying built-in defaults for the OIDC scopes and claims.
func (s *SettingService) GetEffectiveOAuthConfig() config.OAuthConfig {
	env := config.OAuthEnvValues()

	scopes := s.effectiveOauthString(env, "oauthScopes", s.GetOauthScopes)
	if scopes == "" {
		scopes = "openid,profile,email,groups"
	}
	groupsClaim := s.effectiveOauthString(env, "oauthGroupsClaim", s.GetOauthGroupsClaim)
	if groupsClaim == "" {
		groupsClaim = "groups"
	}
	usernameClaim := s.effectiveOauthString(env, "oauthUsernameClaim", s.GetOauthUsernameClaim)
	if usernameClaim == "" {
		usernameClaim = "email"
	}

	return config.OAuthConfig{
		Issuer:             s.effectiveOauthString(env, "oauthIssuer", s.GetOauthIssuer),
		ClientID:           s.effectiveOauthString(env, "oauthClientId", s.GetOauthClientId),
		ClientSecret:       s.effectiveOauthString(env, "oauthClientSecret", s.GetOauthClientSecret),
		RedirectURL:        s.effectiveOauthString(env, "oauthRedirectUrl", s.GetOauthRedirectUrl),
		Scopes:             oauthSplitCsv(scopes),
		GroupsClaim:        groupsClaim,
		UsernameClaim:      usernameClaim,
		AdminGroup:         s.effectiveOauthString(env, "oauthAdminGroup", s.GetOauthAdminGroup),
		UserGroups:         oauthSplitCsv(s.effectiveOauthString(env, "oauthUserGroup", s.GetOauthUserGroup)),
		UserInboundRemarks: oauthSplitCsv(s.effectiveOauthString(env, "oauthUserInboundRemark", s.GetOauthUserInboundRemark)),
		UserTotalGB:        int64(s.effectiveOauthInt(env, "oauthUserTotalGB", s.GetOauthUserTotalGB)),
		UserExpiryDays:     s.effectiveOauthInt(env, "oauthUserExpiryDays", s.GetOauthUserExpiryDays),
		UserLimitIP:        s.effectiveOauthInt(env, "oauthUserLimitIP", s.GetOauthUserLimitIP),
	}
}

// OAuthEnvLocks reports which OAuth settings are pinned by a XUI_OAUTH_* env var
// (read-only in the UI). The enable toggle is locked ON when both the issuer and
// client id come from the environment, preserving env-only deployments.
func (s *SettingService) OAuthEnvLocks() map[string]bool {
	env := config.OAuthEnvValues()
	locks := make(map[string]bool, len(env)+1)
	for k := range env {
		locks[k] = true
	}
	if env["oauthIssuer"] != "" && env["oauthClientId"] != "" {
		locks["oauthEnable"] = true
	}
	return locks
}

// enforceOauthEnvLocks resets env-pinned OAuth fields to their stored values
// before a save, so a field the UI presents read-only cannot be changed through
// the API (the env value overrides at read time regardless).
func (s *SettingService) enforceOauthEnvLocks(allSetting *entity.AllSetting) {
	locks := s.OAuthEnvLocks()
	if locks["oauthEnable"] {
		allSetting.OauthEnable, _ = s.GetOauthEnable()
	}
	if locks["oauthIssuer"] {
		allSetting.OauthIssuer, _ = s.GetOauthIssuer()
	}
	if locks["oauthClientId"] {
		allSetting.OauthClientId, _ = s.GetOauthClientId()
	}
	if locks["oauthClientSecret"] {
		allSetting.OauthClientSecret, _ = s.GetOauthClientSecret()
	}
	if locks["oauthRedirectUrl"] {
		allSetting.OauthRedirectUrl, _ = s.GetOauthRedirectUrl()
	}
	if locks["oauthScopes"] {
		allSetting.OauthScopes, _ = s.GetOauthScopes()
	}
	if locks["oauthGroupsClaim"] {
		allSetting.OauthGroupsClaim, _ = s.GetOauthGroupsClaim()
	}
	if locks["oauthUsernameClaim"] {
		allSetting.OauthUsernameClaim, _ = s.GetOauthUsernameClaim()
	}
	if locks["oauthAdminGroup"] {
		allSetting.OauthAdminGroup, _ = s.GetOauthAdminGroup()
	}
	if locks["oauthUserGroup"] {
		allSetting.OauthUserGroup, _ = s.GetOauthUserGroup()
	}
	if locks["oauthUserInboundRemark"] {
		allSetting.OauthUserInboundRemark, _ = s.GetOauthUserInboundRemark()
	}
	if locks["oauthUserTotalGB"] {
		allSetting.OauthUserTotalGB, _ = s.GetOauthUserTotalGB()
	}
	if locks["oauthUserExpiryDays"] {
		allSetting.OauthUserExpiryDays, _ = s.GetOauthUserExpiryDays()
	}
	if locks["oauthUserLimitIP"] {
		allSetting.OauthUserLimitIP, _ = s.GetOauthUserLimitIP()
	}
}

// OAuthEnabledEffective reports whether OIDC login is active: issuer and client
// id must resolve, and the flow must be enabled (env-pinned or the stored toggle).
func (s *SettingService) OAuthEnabledEffective() bool {
	cfg := s.GetEffectiveOAuthConfig()
	if cfg.Issuer == "" || cfg.ClientID == "" {
		return false
	}
	if s.OAuthEnvLocks()["oauthEnable"] {
		return true
	}
	enable, _ := s.GetOauthEnable()
	return enable
}
