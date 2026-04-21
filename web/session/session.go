// Package session provides session management utilities for the 3x-ui web panel.
// It handles user authentication state, login sessions, and session storage using Gin sessions.
package session

import (
	"encoding/gob"
	"net/http"

	"github.com/mhsanaei/3x-ui/v2/database/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	loginUserKey = "LOGIN_USER"
)

func init() {
	gob.Register(model.User{})
}

// SetLoginUser stores the authenticated user in the session.
// The user object is serialized and stored for subsequent requests.
func SetLoginUser(c *gin.Context, user *model.User) {
	if user == nil {
		return
	}
	s := sessions.Default(c)
	s.Set(loginUserKey, *user)
}

// GetLoginUser retrieves the authenticated user from the session.
// Returns nil if no user is logged in or if the session data is invalid.
func GetLoginUser(c *gin.Context) *model.User {
	s := sessions.Default(c)
	obj := s.Get(loginUserKey)
	if obj == nil {
		return nil
	}
	user, ok := obj.(model.User)
	if !ok {

		s.Delete(loginUserKey)
		return nil
	}
	return &user
}

// IsLogin checks if a user is currently authenticated in the session.
// Returns true if a valid user session exists, false otherwise.
func IsLogin(c *gin.Context) bool {
	return GetLoginUser(c) != nil
}

// ClearSession removes all session data and invalidates the session.
// This effectively logs out the user and clears any stored session information.
func ClearSession(c *gin.Context) {
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
		SameSite: http.SameSiteLaxMode,
	})
}
