package frontproxy

import (
	"bytes"
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
	// The wording varies per install, so assert on what every render shares
	// rather than on one phrasing this seed happened to pick.
	body := rec.Body.String()
	if !strings.Contains(body, `<html lang="ru"`) || !strings.Contains(body, "</html>") {
		t.Errorf("body does not look like a rendered decoy page: %q", body)
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
		body, err := renderDecoyTemplate(name, "seed")
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
func TestRenderDecoyTemplateRejectsTraversal(t *testing.T) {
	for _, name := range []string{"../go.mod", "a/b", `a\b`, "maintenance.html", "./x"} {
		if _, err := renderDecoyTemplate(name, "seed"); err == nil {
			t.Errorf("renderDecoyTemplate(%q) succeeded, want rejection", name)
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

// A site the admin uploads doesn't know its own domain in advance, and
// shouldn't have to be re-uploaded just because the reverse proxy's domain
// changed -- {{DECOY_ORIGIN}} in any of its text files resolves to whatever
// host the visitor actually used.
func TestUploadDecoySubstitutesOriginToken(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"index.html": `<html><a href="{{DECOY_ORIGIN}}/articles/a.html">a</a></html>`,
		"sitemap.xml": `<urlset><url><loc>{{DECOY_ORIGIN}}/</loc></url></urlset>`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	h, err := newUploadDecoy(dir)
	if err != nil {
		t.Fatal(err)
	}

	check := func(path, wantSubstring string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Host = "ccl.852654.xyz"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if strings.Contains(rec.Body.String(), "{{DECOY_ORIGIN}}") {
			t.Errorf("%s: token was not substituted: %q", path, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), wantSubstring) {
			t.Errorf("%s: want %q in body, got %q", path, wantSubstring, rec.Body.String())
		}
	}
	check("/", "https://ccl.852654.xyz/articles/a.html")
	check("/sitemap.xml", "https://ccl.852654.xyz/")
}

// Binary/non-text files must never be read into memory and scanned for the
// token -- only the small text-format allowlist is, so an uploaded image or
// font is still streamed as-is by http.FileServer.
func TestUploadDecoyLeavesNonTextFilesAlone(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>ok</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw := []byte("\x89PNG\r\n\x1a\n" + "{{DECOY_ORIGIN}} looks like a token but is inside binary data")
	if err := os.WriteFile(filepath.Join(dir, "logo.png"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	rec := serveDecoy(t, DecoyConfig{Mode: DecoyUpload, Dir: dir}, "/logo.png")
	if !bytes.Equal(rec.Body.Bytes(), raw) {
		t.Errorf("binary file was altered: got %q, want %q", rec.Body.Bytes(), raw)
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

// The whole point of the variant layer: two installs must not serve the same
// bytes, or the page hash identifies this panel instead of hiding it.
func TestTemplatesDifferBetweenInstalls(t *testing.T) {
	for _, name := range DecoyTemplateNames() {
		if _, ok := loginMocks[name]; ok {
			// Exempt: a login-mock renders one real product's fixed branding, not per-install colors.
			// Matching countless real non-panel installs of that product isn't a fingerprint; looking unlike it would be.
			continue
		}
		a, err := renderDecoyTemplate(name, "install-one")
		if err != nil {
			t.Fatalf("template %q: %v", name, err)
		}
		b, err := renderDecoyTemplate(name, "install-two")
		if err != nil {
			t.Fatalf("template %q: %v", name, err)
		}
		if bytes.Equal(a, b) {
			t.Errorf("template %q renders identically for two seeds", name)
		}
	}
}

// It must still be stable for one install, though: markup that changes on
// every reload is its own kind of tell.
func TestTemplateIsStableForOneInstall(t *testing.T) {
	a, err := renderDecoyTemplate("maintenance", "same-seed")
	if err != nil {
		t.Fatal(err)
	}
	b, err := renderDecoyTemplate("maintenance", "same-seed")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Error("the same seed rendered two different pages")
	}
}

// A browser given no validator caches the page heuristically, which is how a
// decoy keeps showing the previous theme after the admin picks a new one.
func TestTemplateDecoySendsCacheValidators(t *testing.T) {
	rec := serveDecoy(t, DecoyConfig{Mode: DecoyTemplate, Template: "clock"}, "/")
	if rec.Header().Get("ETag") == "" {
		t.Error("no ETag, so the browser has nothing to revalidate against")
	}
	if rec.Header().Get("Cache-Control") == "" {
		t.Error("no Cache-Control, so freshness is left to browser heuristics")
	}
}

// Picking a different theme must change the validator, or a cached copy of
// the old page stays valid forever.
func TestSwitchingTemplateChangesTheETag(t *testing.T) {
	seen := map[string]string{}
	for _, name := range []string{"clock", "tetris", "parked"} {
		rec := serveDecoy(t, DecoyConfig{Mode: DecoyTemplate, Template: name}, "/")
		tag := rec.Header().Get("ETag")
		if prev, dup := seen[tag]; dup {
			t.Errorf("themes %q and %q share ETag %s", prev, name, tag)
		}
		seen[tag] = name
	}
}

// With a matching validator the answer is 304 and no body, so an unchanged
// decoy costs a browser nothing to recheck.
func TestUnchangedTemplateRevalidatesTo304(t *testing.T) {
	first := serveDecoy(t, DecoyConfig{Mode: DecoyTemplate, Template: "clock"}, "/")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("If-None-Match", first.Header().Get("ETag"))
	rec := httptest.NewRecorder()
	newDecoyHandler(DecoyConfig{Mode: DecoyTemplate, Template: "clock"}).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Errorf("status = %d, want 304 for an unchanged page", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("304 carried a %d-byte body", rec.Body.Len())
	}
}
