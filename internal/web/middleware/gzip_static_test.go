package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

// readCountingFS counts ReadFile calls, i.e. how often a file gets compressed.
type readCountingFS struct {
	fstest.MapFS
	reads atomic.Int32
}

func (f *readCountingFS) ReadFile(name string) ([]byte, error) {
	f.reads.Add(1)
	return f.MapFS.ReadFile(name)
}

func TestGzipStatic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	raw := strings.Repeat("console.log('bundle');", 4096)
	files := &readCountingFS{MapFS: fstest.MapFS{
		"app.js": {Data: []byte(raw)},
	}}
	r := gin.New()
	r.Group("/assets", GzipStatic(files)).StaticFS("/", http.FS(files))

	do := func(method, target, acceptEncoding string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, target, nil)
		req.Header.Set("Accept-Encoding", acceptEncoding)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	t.Run("gzip downloads share one compression", func(t *testing.T) {
		for range 3 {
			w := do(http.MethodGet, "/assets/app.js", "gzip, deflate")
			if w.Code != http.StatusOK || w.Header().Get("Content-Encoding") != "gzip" || w.Header().Get("Vary") != "Accept-Encoding" {
				t.Fatalf("got %d, headers %v", w.Code, w.Header())
			}
			if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
				t.Errorf("Content-Type = %q, want the bundle's own type", ct)
			}
			zr, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			if plain, _ := io.ReadAll(zr); string(plain) != raw {
				t.Fatal("decompressed body differs from the bundle")
			}
		}
		if n := files.reads.Load(); n != 1 {
			t.Errorf("bundle compressed %d times, want once", n)
		}
	})

	t.Run("HEAD sends headers only", func(t *testing.T) {
		w := do(http.MethodHead, "/assets/app.js", "gzip")
		if w.Code != http.StatusOK || w.Body.Len() != 0 || w.Header().Get("Content-Encoding") != "gzip" {
			t.Errorf("got %d, %d body bytes, headers %v", w.Code, w.Body.Len(), w.Header())
		}
	})

	t.Run("without Accept-Encoding the static route serves the plain file", func(t *testing.T) {
		w := do(http.MethodGet, "/assets/app.js", "")
		if w.Code != http.StatusOK || w.Header().Get("Content-Encoding") != "" || w.Body.String() != raw {
			t.Errorf("got %d, Content-Encoding %q, %d body bytes", w.Code, w.Header().Get("Content-Encoding"), w.Body.Len())
		}
	})

	t.Run("unknown name is left to the static route", func(t *testing.T) {
		w := do(http.MethodGet, "/assets/none.js", "gzip")
		if w.Code != http.StatusNotFound || w.Header().Get("Content-Encoding") != "" {
			t.Errorf("got %d, Content-Encoding %q, want 404 plain", w.Code, w.Header().Get("Content-Encoding"))
		}
	})
}
