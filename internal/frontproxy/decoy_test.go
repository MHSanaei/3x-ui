package frontproxy

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func serveDecoy(t *testing.T, cfg DecoyConfig, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	newDecoyHandler(cfg).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestTemplateDecoyServesEmbeddedPage(t *testing.T) {
	rec := serveDecoy(t, DecoyConfig{Mode: DecoyTemplate, Template: "maintenance"}, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Scheduled maintenance") {
		t.Errorf("body does not look like the maintenance page: %q", rec.Body.String())
	}
}

// Every embedded template must actually load, or the settings UI would offer
// a choice that silently degrades to the default.
func TestEveryAdvertisedTemplateLoads(t *testing.T) {
	names := DecoyTemplateNames()
	if len(names) == 0 {
		t.Fatal("no decoy templates advertised")
	}
	for _, name := range names {
		body, err := readDecoyTemplate(name)
		if err != nil {
			t.Errorf("template %q failed to load: %v", name, err)
			continue
		}
		if !strings.Contains(string(body), "<html") {
			t.Errorf("template %q does not look like HTML", name)
		}
	}
}

// A template name is admin-supplied; it must never be able to reach outside
// the embedded set and read arbitrary files.
func TestReadDecoyTemplateRejectsTraversal(t *testing.T) {
	for _, name := range []string{"../go.mod", "a/b", `a\b`, "maintenance.html", "./x"} {
		if _, err := readDecoyTemplate(name); err == nil {
			t.Errorf("readDecoyTemplate(%q) succeeded, want rejection", name)
		}
	}
}

func TestUnknownTemplateFallsBackToDefault(t *testing.T) {
	rec := serveDecoy(t, DecoyConfig{Mode: DecoyTemplate, Template: "does-not-exist"}, "/")
	if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
		t.Fatalf("status = %d, body len = %d, want a served fallback page", rec.Code, rec.Body.Len())
	}
}

func TestUploadDecoyServesIndex(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>my own site</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := serveDecoy(t, DecoyConfig{Mode: DecoyUpload, Dir: dir}, "/")
	if !strings.Contains(rec.Body.String(), "my own site") {
		t.Errorf("uploaded site not served, got %q", rec.Body.String())
	}
}

// An upload directory that was never populated must not turn the reverse proxy
// into a 404/500 -- it falls back to a template so the decoy still looks real.
func TestUploadDecoyWithoutIndexFallsBackToTemplate(t *testing.T) {
	rec := serveDecoy(t, DecoyConfig{Mode: DecoyUpload, Dir: t.TempDir()}, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 from the template fallback", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<html") {
		t.Errorf("expected a fallback HTML page, got %q", rec.Body.String())
	}
}

func TestProxyDecoyForwardsToUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("upstream page"))
	}))
	defer upstream.Close()

	rec := serveDecoy(t, DecoyConfig{Mode: DecoyProxy, ProxyURL: upstream.URL}, "/")
	if !strings.Contains(rec.Body.String(), "upstream page") {
		t.Errorf("proxy decoy did not forward, got %q", rec.Body.String())
	}
}

// A malformed or non-HTTP proxy URL must degrade to a template rather than
// leaving the reverse proxy serving nothing.
func TestProxyDecoyRejectsBadURLs(t *testing.T) {
	for _, raw := range []string{"", "://nope", "ftp://example.com", "file:///etc/passwd", "https://"} {
		if _, err := newProxyDecoy(raw); err == nil {
			t.Errorf("newProxyDecoy(%q) succeeded, want rejection", raw)
		}
		rec := serveDecoy(t, DecoyConfig{Mode: DecoyProxy, ProxyURL: raw}, "/")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<html") {
			t.Errorf("proxy URL %q: expected template fallback, got status %d", raw, rec.Code)
		}
	}
}
