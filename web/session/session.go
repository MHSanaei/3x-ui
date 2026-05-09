// Package session provides session management utilities for the 3x-ui web panel.
// It handles user authentication state, login sessions, and session storage using Gin sessions.
package session

import (
	"encoding/gob"
	"net/http"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	loginUserKey = "LOGIN_USER"
	// apiAuthUserKey is the gin-context key under which checkAPIAuth
	// stashes a fallback user for Bearer-token-authenticated callers.
	// Bearer requests don't carry a session cookie, so handlers that
	// scope writes by user.Id (e.g. InboundController.addInbound) would
	// otherwise nil-deref. Keeping the override in the gin context
	// (not the cookie session) means the fallback never leaks into a
	// browser request.
	apiAuthUserKey = "api_auth_user"
)

func init() {
	gob.Register(model.User{})
}

// SetLoginUser stores the authenticated user in the session and persists it.
// gin-contrib/sessions does not auto-save; callers that forget Save() leave
// the cookie out of sync with server state — this helper avoids that pitfall.
func SetLoginUser(c *gin.Context, user *model.User) error {
	if user == nil {
		return nil
	}
	s := sessions.Default(c)
	s.Set(loginUserKey, *user)
	return s.Save()
}

// SetAPIAuthUser stashes a fallback user on the gin context for the
// lifetime of a single bearer-authed request. checkAPIAuth calls this
// after a successful token match so downstream handlers that read
// GetLoginUser don't see nil.
func SetAPIAuthUser(c *gin.Context, user *model.User) {
	if user == nil {
		return
	}
	c.Set(apiAuthUserKey, user)
}

// GetLoginUser retrieves the authenticated user from the session.
// Returns nil if no user is logged in or if the session data is invalid.
func GetLoginUser(c *gin.Context) *model.User {
	if v, ok := c.Get(apiAuthUserKey); ok {
		if u, ok2 := v.(*model.User); ok2 {
			return u
		}
	}
	s := sessions.Default(c)
	obj := s.Get(loginUserKey)
	if obj == nil {
		return nil
	}
	user, ok := obj.(model.User)
	if !ok {
		// Stale or incompatible session payload — wipe and persist immediately
		// so subsequent requests don't keep hitting the same broken cookie.
		s.Delete(loginUserKey)
		if err := s.Save(); err != nil {
			logger.Warning("session: failed to drop stale user payload:", err)
		}
		return nil
	}
	return &user
}

// IsLogin checks if a user is currently authenticated in the session.
func IsLogin(c *gin.Context) bool {
	return GetLoginUser(c) != nil
}

// ClearSession invalidates the session and tells the browser to drop the cookie.
// The cookie attributes (Path/HttpOnly/SameSite) must mirror those used when
// the cookie was created or browsers will keep it.
func ClearSession(c *gin.Context) error {
	s := sessions.Default(c)
	s.Clear()
	cookiePath := c.GetString("base_path")
	if cookiePath == "" {
		cookiePath = "/"
	}
	s.Options(sessions.Options{
		Path:     cookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   c.Request.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
	return s.Save()
}
