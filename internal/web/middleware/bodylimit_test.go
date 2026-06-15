package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMaxBodyBytes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const limit = 16

	r := gin.New()
	r.Use(MaxBodyBytes(limit))
	r.POST("/x", func(c *gin.Context) {
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			c.String(http.StatusRequestEntityTooLarge, "too big")
			return
		}
		c.String(http.StatusOK, "ok")
	})
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// Body within the limit is read normally.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("0123456789")))
	if w.Code != http.StatusOK {
		t.Errorf("under-limit POST: got %d, want 200", w.Code)
	}

	// Body over the limit makes the handler's read fail (no unbounded buffer).
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader(make([]byte, limit*4))))
	if w.Code == http.StatusOK {
		t.Errorf("over-limit POST should not succeed, got 200")
	}

	// Bodyless methods pass through untouched.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Code != http.StatusOK {
		t.Errorf("GET should pass through, got %d", w.Code)
	}
}

func TestMaxBodyBytesSkipSuffix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const limit = 16

	r := gin.New()
	r.Use(MaxBodyBytes(limit, "/server/importDB"))
	read := func(c *gin.Context) {
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			c.String(http.StatusRequestEntityTooLarge, "too big")
			return
		}
		c.String(http.StatusOK, "ok")
	}
	r.POST("/server/importDB", read)
	r.POST("/x", read)

	// Exempt route reads an over-limit body without error.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/server/importDB", bytes.NewReader(make([]byte, limit*4))))
	if w.Code != http.StatusOK {
		t.Errorf("exempt route should pass through over-limit body, got %d", w.Code)
	}

	// Non-exempt route is still capped.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader(make([]byte, limit*4))))
	if w.Code == http.StatusOK {
		t.Errorf("non-exempt over-limit POST should not succeed, got 200")
	}
}
