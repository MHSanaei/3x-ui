package sub

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// newTestSUBController builds a controller with just the bits loadSubTemplate
// needs, so the template tests don't require a database.
func newTestSUBController() *SUBController {
	return &SUBController{subTemplateCache: map[string]*cachedSubTemplate{}}
}

func TestShouldAutoServeClash(t *testing.T) {
	tests := []struct {
		name         string
		autoDetect   bool
		clashEnabled bool
		wantsHTML    bool
		userAgent    string
		pattern      string
		want         bool
	}{
		{name: "clash verge", autoDetect: true, clashEnabled: true, userAgent: "Clash-Verge/v2.4.2", want: true},
		{name: "mihomo", autoDetect: true, clashEnabled: true, userAgent: "mihomo/1.19.12", want: true},
		{name: "stash case insensitive", autoDetect: true, clashEnabled: true, userAgent: "STASH/2.6.0", want: true},
		{name: "flclash covered by clash", autoDetect: true, clashEnabled: true, userAgent: "FlClash/0.8.91", want: true},
		{name: "xray raw fallback", autoDetect: true, clashEnabled: true, userAgent: "v2rayNG/1.10.0"},
		{name: "v2raya raw fallback", autoDetect: true, clashEnabled: true, userAgent: "v2rayA/2.2"},
		{name: "unknown raw fallback", autoDetect: true, clashEnabled: true, userAgent: "CustomClient/1.0"},
		{name: "empty raw fallback", autoDetect: true, clashEnabled: true},
		{name: "browser HTML wins", autoDetect: true, clashEnabled: true, wantsHTML: true, userAgent: "Clash-Verge/v2.4.2"},
		{name: "disabled by default", clashEnabled: true, userAgent: "mihomo/1.19.12"},
		{name: "clash endpoint disabled", autoDetect: true, userAgent: "mihomo/1.19.12"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldAutoServeClash(tt.autoDetect, tt.clashEnabled, tt.wantsHTML, tt.userAgent, compileUserAgentRegex("Clash/Mihomo", tt.pattern, service.DefaultSubClashUserAgentRegex))
			if got != tt.want {
				t.Fatalf("shouldAutoServeClash() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldAutoServeClashUsesConfiguredRegex(t *testing.T) {
	configured := compileUserAgentRegex("Clash/Mihomo", `(?i)^custom-client/`, service.DefaultSubClashUserAgentRegex)
	if !shouldAutoServeClash(true, true, false, "Custom-Client/1.0", configured) {
		t.Fatal("configured User-Agent regex did not match")
	}
	if shouldAutoServeClash(true, true, false, "Mihomo/1.19", configured) {
		t.Fatal("built-in User-Agent matched after a custom regex replaced it")
	}
}

func TestCompileUserAgentRegexFallsBackForInvalidPattern(t *testing.T) {
	compiled := compileUserAgentRegex("Clash/Mihomo", "[", service.DefaultSubClashUserAgentRegex)
	if !compiled.MatchString("Mihomo/1.19") {
		t.Fatal("invalid regex did not fall back to the default pattern")
	}
}

func TestSanitizeUserAgentForLog(t *testing.T) {
	if got := sanitizeUserAgentForLog("client/1.0\r\nforged\tline"); got != "client/1.0  forged line" {
		t.Fatalf("sanitizeUserAgentForLog() = %q", got)
	}
	long := strings.Repeat("界", 513)
	if got := sanitizeUserAgentForLog(long); len([]rune(got)) != 512 {
		t.Fatalf("sanitized User-Agent length = %d runes, want 512", len([]rune(got)))
	}
}

func TestStandardSubscriptionAutoDetectsClash(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "auto", 4480, 1, `{"network":"tcp","security":"none"}`)
	gin.SetMode(gin.TestMode)

	newRouter := func(autoDetect bool, clashUserAgentRegex string) *gin.Engine {
		router := gin.New()
		NewSUBController(
			router.Group("/"),
			"/sub/", "/json/", "/clash/",
			autoDetect, clashUserAgentRegex, true, true, true,
			"", "12", "", "", "", false, "",
			"", "", "", "", false, "", false, false, "",
		)
		return router
	}

	t.Run("recognized client receives YAML", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://sub.example.com/sub/s1", nil)
		req.Header.Set("User-Agent", "Clash-Verge/v2.4.2")
		resp := httptest.NewRecorder()

		newRouter(true, "").ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
		}
		if got := resp.Header().Get("Content-Type"); got != "application/yaml; charset=utf-8" {
			t.Fatalf("Content-Type = %q, want YAML", got)
		}
		if body := resp.Body.String(); !strings.Contains(body, "proxies:") || !strings.Contains(body, "type: vless") {
			t.Fatalf("auto-detected body is not Clash YAML:\n%s", body)
		}
	})

	t.Run("disabled setting preserves raw base64", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://sub.example.com/sub/s1", nil)
		req.Header.Set("User-Agent", "Clash-Verge/v2.4.2")
		resp := httptest.NewRecorder()

		newRouter(false, "").ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
		}
		decoded, err := base64.StdEncoding.DecodeString(resp.Body.String())
		if err != nil {
			t.Fatalf("raw response is not base64: %v", err)
		}
		if !strings.Contains(string(decoded), "vless://") {
			t.Fatalf("decoded raw response lacks VLESS link: %s", decoded)
		}
	})

	t.Run("configured regex controls detection", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://sub.example.com/sub/s1", nil)
		req.Header.Set("User-Agent", "Mihomo/1.19")
		resp := httptest.NewRecorder()

		newRouter(true, `(?i)^custom-client/`).ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
		}
		if got := resp.Header().Get("Content-Type"); got == "application/yaml; charset=utf-8" {
			t.Fatalf("Content-Type = %q, custom regex should preserve raw response", got)
		}
	})

	t.Run("Xray client preserves raw base64", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://sub.example.com/sub/s1", nil)
		req.Header.Set("User-Agent", "v2rayA/2.2")
		resp := httptest.NewRecorder()

		newRouter(true, "").ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
		}
		decoded, err := base64.StdEncoding.DecodeString(resp.Body.String())
		if err != nil {
			t.Fatalf("raw response is not base64: %v", err)
		}
		if !strings.Contains(string(decoded), "vless://") {
			t.Fatalf("decoded raw response lacks VLESS link: %s", decoded)
		}
	})
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func renderTemplate(t *testing.T, a *SUBController, dir string, data map[string]any) string {
	t.Helper()
	tmpl, err := a.loadSubTemplate(dir)
	if err != nil {
		t.Fatalf("loadSubTemplate: unexpected error: %v", err)
	}
	if tmpl == nil {
		t.Fatal("loadSubTemplate: expected a template, got nil")
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("execute: %v", err)
	}
	return buf.String()
}

func TestLoadSubTemplate_RendersIndex(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), `<h1>{{ .sId }}</h1>`)

	got := renderTemplate(t, newTestSUBController(), dir, map[string]any{"sId": "abc-123"})
	if want := `<h1>abc-123</h1>`; got != want {
		t.Fatalf("rendered = %q, want %q", got, want)
	}
}

func TestLoadSubTemplate_PrefersSubHTML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), `from-index`)
	writeFile(t, filepath.Join(dir, "sub.html"), `from-sub`)

	got := renderTemplate(t, newTestSUBController(), dir, nil)
	if got != "from-sub" {
		t.Fatalf("rendered = %q, want %q (sub.html should take precedence)", got, "from-sub")
	}
}

func TestLoadSubTemplate_FallbackCases(t *testing.T) {
	a := newTestSUBController()

	t.Run("missing dir", func(t *testing.T) {
		tmpl, err := a.loadSubTemplate(filepath.Join(t.TempDir(), "does-not-exist"))
		if tmpl != nil || err != nil {
			t.Fatalf("got (%v, %v), want (nil, nil)", tmpl, err)
		}
	})

	t.Run("path is a file not a dir", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "index.html")
		writeFile(t, file, `whatever`)
		tmpl, err := a.loadSubTemplate(file)
		if tmpl != nil || err != nil {
			t.Fatalf("got (%v, %v), want (nil, nil)", tmpl, err)
		}
	})

	t.Run("dir without template file", func(t *testing.T) {
		tmpl, err := a.loadSubTemplate(t.TempDir())
		if tmpl != nil || err != nil {
			t.Fatalf("got (%v, %v), want (nil, nil)", tmpl, err)
		}
	})
}

func TestLoadSubTemplate_MalformedTemplate(t *testing.T) {
	dir := t.TempDir()
	// Unterminated action — html/template fails to parse this.
	writeFile(t, filepath.Join(dir, "index.html"), `<h1>{{ .sId </h1>`)

	tmpl, err := newTestSUBController().loadSubTemplate(dir)
	if err == nil {
		t.Fatal("expected a parse error for a malformed template, got nil")
	}
	if tmpl != nil {
		t.Fatalf("expected nil template on parse error, got %v", tmpl)
	}
}

func TestLoadSubTemplate_CacheHitAndInvalidation(t *testing.T) {
	a := newTestSUBController()
	dir := t.TempDir()
	path := filepath.Join(dir, "index.html")

	// v1 with a fixed mtime.
	writeFile(t, path, `v1`)
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, t1, t1); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	first, err := a.loadSubTemplate(dir)
	if err != nil || first == nil {
		t.Fatalf("first load: (%v, %v)", first, err)
	}

	// Same mtime → cache hit returns the identical parsed template.
	second, err := a.loadSubTemplate(dir)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if second != first {
		t.Fatal("expected cache hit to return the same *template.Template pointer")
	}

	// New content + newer mtime → cache invalidated, fresh content served.
	writeFile(t, path, `v2`)
	t2 := t1.Add(time.Hour)
	if err := os.Chtimes(path, t2, t2); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	third, err := a.loadSubTemplate(dir)
	if err != nil || third == nil {
		t.Fatalf("third load: (%v, %v)", third, err)
	}
	if third == first {
		t.Fatal("expected cache invalidation to re-parse the template after mtime change")
	}
	var buf bytes.Buffer
	if err := third.Execute(&buf, nil); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if buf.String() != "v2" {
		t.Fatalf("rendered = %q, want %q after edit", buf.String(), "v2")
	}
}
