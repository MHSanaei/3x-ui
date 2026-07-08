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
	const limit = 10 << 20

	r := gin.New()
	r.Use(MaxBodyBytes(limit, "/server/importDB"))
	read := func(c *gin.Context) {
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			c.String(http.StatusRequestEntityTooLarge, "too big")
			return
		}
		c.String(http.StatusOK, "ok")
	}
	r.POST("/prefix/panel/api/server/importDB", read)
	r.POST("/prefix/panel/api/server/importDB/other", read)
	r.POST("/x", read)

	large := bytes.Repeat([]byte("x"), limit+1)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/prefix/panel/api/server/importDB", bytes.NewReader(large)))
	if w.Code != http.StatusOK {
		t.Fatalf("restore route should accept an over-limit body, got %d", w.Code)
	}

	for _, path := range []string{"/x", "/prefix/panel/api/server/importDB/other"} {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(large)))
		if w.Code == http.StatusOK {
			t.Fatalf("non-exempt path %q accepted an over-limit body", path)
		}
	}
}
