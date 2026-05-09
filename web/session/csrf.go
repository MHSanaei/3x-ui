package session

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"io"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const csrfTokenKey = "CSRF_TOKEN"

// CSRFHeaderName is the request header used by browser clients for unsafe methods.
const CSRFHeaderName = "X-CSRF-Token"

// EnsureCSRFToken returns the current session CSRF token or creates one.
func EnsureCSRFToken(c *gin.Context) (string, error) {
	s := sessions.Default(c)
	if token, ok := s.Get(csrfTokenKey).(string); ok && token != "" {
		return token, nil
	}
	token, err := newCSRFToken()
	if err != nil {
		return "", err
	}
	s.Set(csrfTokenKey, token)
	return token, s.Save()
}

// ValidateCSRFToken checks the submitted CSRF token against the session token.
func ValidateCSRFToken(c *gin.Context) bool {
	s := sessions.Default(c)
	expected, ok := s.Get(csrfTokenKey).(string)
	if !ok || expected == "" {
		return false
	}
	actual := c.GetHeader(CSRFHeaderName)
	if actual == "" {
		actual = c.PostForm("_csrf")
	}
	if len(actual) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

func newCSRFToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
