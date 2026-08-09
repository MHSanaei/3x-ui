package session

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// Session keys for OIDC login. Role distinguishes the full-access admin tier
// from the self-service user tier; the flow keys hold per-login CSRF/PKCE state.
const (
	loginRoleKey        = "LOGIN_ROLE"
	loginClientSubIDKey = "LOGIN_CLIENT_SUBID"
	oauthStateKey       = "OAUTH_STATE"
	oauthNonceKey       = "OAUTH_NONCE"
	oauthVerifierKey    = "OAUTH_VERIFIER"
)

// Login roles. An absent role is treated as admin so password logins and
// pre-existing sessions keep full access without a migration.
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// SetLoginRole records the caller's tier on the session.
func SetLoginRole(c *gin.Context, role string) error {
	s := sessions.Default(c)
	s.Set(loginRoleKey, role)
	return s.Save()
}

// GetLoginRole returns the session role, defaulting to admin when unset.
func GetLoginRole(c *gin.Context) string {
	s := sessions.Default(c)
	if role, ok := s.Get(loginRoleKey).(string); ok && role != "" {
		return role
	}
	return RoleAdmin
}

// SetLoginClientSubID binds a self-service (user-tier) session to the subId of
// its provisioned client, which the cabinet reads to show the connection.
func SetLoginClientSubID(c *gin.Context, subID string) error {
	s := sessions.Default(c)
	s.Set(loginClientSubIDKey, subID)
	return s.Save()
}

// GetLoginClientSubID returns the user-tier client subId, or "" for an admin or
// anonymous session. Its presence is what authorizes the cabinet.
func GetLoginClientSubID(c *gin.Context) string {
	s := sessions.Default(c)
	subID, _ := s.Get(loginClientSubIDKey).(string)
	return subID
}

// SetOAuthFlow persists the state, nonce, and PKCE verifier for the in-flight
// login so the callback can validate them.
func SetOAuthFlow(c *gin.Context, state, nonce, verifier string) error {
	s := sessions.Default(c)
	s.Set(oauthStateKey, state)
	s.Set(oauthNonceKey, nonce)
	s.Set(oauthVerifierKey, verifier)
	return s.Save()
}

// GetOAuthFlow returns the stored state, nonce, and PKCE verifier.
func GetOAuthFlow(c *gin.Context) (state, nonce, verifier string) {
	s := sessions.Default(c)
	state, _ = s.Get(oauthStateKey).(string)
	nonce, _ = s.Get(oauthNonceKey).(string)
	verifier, _ = s.Get(oauthVerifierKey).(string)
	return state, nonce, verifier
}

// ClearOAuthFlow drops the in-flight login state after the callback consumes it.
func ClearOAuthFlow(c *gin.Context) error {
	s := sessions.Default(c)
	s.Delete(oauthStateKey)
	s.Delete(oauthNonceKey)
	s.Delete(oauthVerifierKey)
	return s.Save()
}
