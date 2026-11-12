package web

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// Any per-request compressor here is pinned for as long as an anonymous client
// stalls its download, so the bundles must come out as one shared gzip copy.
func TestAssetsServedFromSharedGzipCopy(t *testing.T) {
	entries, err := fs.ReadDir(distFS, "dist/assets")
	if err != nil || len(entries) == 0 {
		t.Skip("no built frontend bundles embedded (dist stub)")
	}
	name := ""
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".js") {
			name = e.Name()
			break
		}
	}
	if name == "" {
		t.Skip("no .js bundle embedded")
	}
	raw, err := fs.ReadFile(distFS, "dist/assets/"+name)
	if err != nil {
		t.Fatal(err)
	}
	engine := newTestEngine(t)

	get := func(acceptEncoding string) *http.Response {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/assets/"+name, nil)
		req.Header.Set("Accept-Encoding", acceptEncoding)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		return w.Result()
	}

	for range 2 {
		res := get("gzip")
		body, _ := io.ReadAll(res.Body)
		if res.StatusCode != http.StatusOK || res.Header.Get("Content-Encoding") != "gzip" {
			t.Fatalf("gzip download: got %d, Content-Encoding %q", res.StatusCode, res.Header.Get("Content-Encoding"))
		}
		// A precompressed copy is sent with its length; a streamed compressor can't know it.
		if got := res.Header.Get("Content-Length"); got != strconv.Itoa(len(body)) {
			t.Errorf("Content-Length = %q, want %d: bundle was compressed per request", got, len(body))
		}
		if res.Header.Get("Last-Modified") == "" {
			t.Error("Last-Modified missing: assets FS no longer stamps startTime")
		}
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if plain, _ := io.ReadAll(zr); !bytes.Equal(plain, raw) {
			t.Fatal("gzip download decompresses to something other than the bundle")
		}
	}

	res := get("")
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Encoding") != "" || !bytes.Equal(body, raw) {
		t.Fatalf("plain download: got %d, Content-Encoding %q, %d bytes", res.StatusCode, res.Header.Get("Content-Encoding"), len(body))
	}
}

// The assets route never listed its directory; serving the bundles from a shared copy
// must not turn that 404 into a 500.
func TestAssetsDirectoryIsNotFound(t *testing.T) {
	engine := newTestEngine(t)
	for _, encoding := range []string{"", "gzip"} {
		req := httptest.NewRequest(http.MethodGet, "/assets/", nil)
		req.Header.Set("Accept-Encoding", encoding)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("GET /assets/ with Accept-Encoding %q = %d, want 404", encoding, w.Code)
		}
	}
}
