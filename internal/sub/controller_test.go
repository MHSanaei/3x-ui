package sub

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

var testDistFS = fstest.MapFS{
	"dist/subpage.html": {Data: []byte(`<!doctype html><html><head></head><body><div id="root"></div></body></html>`)},
}

// newTestSUBController builds a controller with just the bits loadSubTemplate
// needs, so the template tests don't require a database.
func newTestSUBController() *SUBController {
	return &SUBController{subTemplateCache: map[string]*cachedSubTemplate{}}
}

type subscriptionTestRouterConfig struct {
	clashAutoDetect     bool
	clashUserAgentRegex string
	jsonAutoDetect      bool
	jsonUserAgentRegex  string
	jsonAlwaysArray     bool
}

func newSubscriptionTestRouter(config subscriptionTestRouterConfig) *gin.Engine {
	router := gin.New()
	options := []SUBControllerOption{
		WithSUBJsonEnabled(true),
		WithSUBClashEnabled(true),
	}
	if config.clashAutoDetect {
		options = append(options, WithSUBClashAutoDetect(true))
	}
	if config.clashUserAgentRegex != "" {
		options = append(options, WithSUBClashUserAgentRegex(config.clashUserAgentRegex))
	}
	if config.jsonAutoDetect {
		options = append(options, WithSUBJsonAutoDetect(true))
	}
	if config.jsonUserAgentRegex != "" {
		options = append(options, WithSUBJsonUserAgentRegex(config.jsonUserAgentRegex))
	}
	if config.jsonAlwaysArray {
		options = append(options, WithSUBJsonAlwaysArray(true))
	}
	NewSUBController(router.Group("/"), options...)
	return router
}

func TestNewSUBControllerOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defaults := NewSUBController(gin.New().Group("/"))
	if defaults.subPath != "/sub/" || defaults.subJsonPath != "/json/" || defaults.subClashPath != "/clash/" {
		t.Fatalf("default paths = %q, %q, %q", defaults.subPath, defaults.subJsonPath, defaults.subClashPath)
	}
	if !defaults.subEncrypt || defaults.updateInterval != "12" {
		t.Fatalf("default encryption/update = %v, %q", defaults.subEncrypt, defaults.updateInterval)
	}
	if defaults.subService.remarkTemplate != service.DefaultRemarkTemplate {
		t.Fatalf("default remark template = %q", defaults.subService.remarkTemplate)
	}
	if defaults.jsonEnabled || defaults.clashEnabled {
		t.Fatalf("format endpoints enabled by default: json=%v clash=%v", defaults.jsonEnabled, defaults.clashEnabled)
	}

	configured := NewSUBController(
		gin.New().Group("/"),
		WithSUBPath("/custom/"),
		WithSUBJsonEnabled(true),
		WithSUBEncryption(false),
		WithSUBUpdateInterval("24"),
	)
	if configured.subPath != "/custom/" || !configured.jsonEnabled || configured.subEncrypt || configured.updateInterval != "24" {
		t.Fatalf("configured values were not applied: path=%q json=%v encrypt=%v update=%q",
			configured.subPath, configured.jsonEnabled, configured.subEncrypt, configured.updateInterval)
	}
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
		{name: "clash case insensitive", autoDetect: true, clashEnabled: true, userAgent: "CLASH-META/1.0", want: true},
		{name: "flclash covered by clash", autoDetect: true, clashEnabled: true, userAgent: "FlClash/0.8.91", want: true},
		{name: "generic client raw fallback", autoDetect: true, clashEnabled: true, userAgent: "GenericClient/1.10.0"},
		{name: "other client raw fallback", autoDetect: true, clashEnabled: true, userAgent: "OtherClient/2.2"},
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

func TestShouldAutoServeJson(t *testing.T) {
	configured := compileUserAgentRegex("Xray JSON", `(?i)^jsonclient([ /]|$)`, service.DefaultSubJsonUserAgentRegex)
	for _, userAgent := range []string{"JsonClient/1.6.32", "jsonclient 1.6.32"} {
		if !shouldAutoServeJson(true, true, false, userAgent, configured) {
			t.Errorf("configured Xray JSON regex did not match %q", userAgent)
		}
	}
	for _, userAgent := range []string{"GenericClient/1.10.0", "OtherClient/2.2", "ThirdClient/7.0", "CustomClient/1.0"} {
		if shouldAutoServeJson(true, true, false, userAgent, configured) {
			t.Errorf("configured Xray JSON regex unexpectedly matched %q", userAgent)
		}
	}
	if shouldAutoServeJson(false, true, false, "JsonClient/1.6.32", configured) {
		t.Fatal("disabled Xray JSON auto-detection matched")
	}
	if shouldAutoServeJson(true, false, false, "JsonClient/1.6.32", configured) {
		t.Fatal("disabled JSON endpoint matched")
	}
	if shouldAutoServeJson(true, true, true, "JsonClient/1.6.32", configured) {
		t.Fatal("browser HTML request matched Xray JSON")
	}

	empty := compileUserAgentRegex("Xray JSON", "", service.DefaultSubJsonUserAgentRegex)
	if empty != nil {
		t.Fatal("empty Xray JSON default should not compile to a matcher")
	}
	if shouldAutoServeJson(true, true, false, "JsonClient/1.6.32", empty) {
		t.Fatal("empty Xray JSON default should not auto-serve")
	}
}

func TestShouldAutoServeJsonUsesConfiguredRegex(t *testing.T) {
	configured := compileUserAgentRegex("Xray JSON", `(?i)^custom-json/`, service.DefaultSubJsonUserAgentRegex)
	if !shouldAutoServeJson(true, true, false, "Custom-JSON/1.0", configured) {
		t.Fatal("configured Xray JSON User-Agent regex did not match")
	}
	if shouldAutoServeJson(true, true, false, "OtherClient/1.10.0", configured) {
		t.Fatal("unrelated User-Agent matched after a custom regex was configured")
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

func seedSubMtprotoInbound(t *testing.T, subId, tag string, port int) {
	t.Helper()
	db := database.GetDB()
	secret := "ee1234567890abcdef1234567890abcd7777772e636c6f7564666c6172652e636f6d"
	email := tag + "@e"
	settings := fmt.Sprintf(`{"clients":[{"email":%q,"subId":%q,"enable":true,"secret":%q}]}`, email, subId, secret)
	ib := &model.Inbound{
		UserId: 1, Tag: tag, Enable: true, Listen: "203.0.113.5", Port: port,
		Protocol: model.MTProto, Remark: tag, Settings: settings, StreamSettings: "{}",
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("seed mtproto inbound %s: %v", tag, err)
	}
	client := &model.ClientRecord{Email: email, SubID: subId, Secret: secret, Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client %s: %v", email, err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("seed client_inbound %s: %v", email, err)
	}
}

func TestAutoDetectFallsBackToRawWhenFormatHasNoContent(t *testing.T) {
	seedSubDB(t)
	seedSubMtprotoInbound(t, "s1", "tg", 4490)
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodGet, "http://sub.example.com/sub/s1", nil)
	req.Header.Set("User-Agent", "Clash-Verge/v2.4.2")
	resp := httptest.NewRecorder()

	newSubscriptionTestRouter(subscriptionTestRouterConfig{clashAutoDetect: true, jsonAutoDetect: true}).ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	decoded, err := base64.StdEncoding.DecodeString(resp.Body.String())
	if err != nil {
		t.Fatalf("fallback response is not base64: %v", err)
	}
	if !strings.Contains(string(decoded), "tg://proxy") {
		t.Fatalf("decoded fallback lacks the Telegram proxy link: %s", decoded)
	}
}

func TestStandardSubscriptionAutoDetectsFormats(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "auto", 4480, 1, `{"network":"tcp","security":"none"}`)
	gin.SetMode(gin.TestMode)

	t.Run("recognized client receives YAML", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://sub.example.com/sub/s1", nil)
		req.Header.Set("User-Agent", "Clash-Verge/v2.4.2")
		resp := httptest.NewRecorder()

		newSubscriptionTestRouter(subscriptionTestRouterConfig{clashAutoDetect: true}).ServeHTTP(resp, req)

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

	t.Run("Clash wins when both format regexes match", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://sub.example.com/sub/s1", nil)
		req.Header.Set("User-Agent", "Hybrid/1.0")
		resp := httptest.NewRecorder()

		newSubscriptionTestRouter(subscriptionTestRouterConfig{
			clashAutoDetect:     true,
			clashUserAgentRegex: `(?i)^hybrid/`,
			jsonAutoDetect:      true,
			jsonUserAgentRegex:  `(?i)^hybrid/`,
		}).ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
		}
		if got := resp.Header().Get("Content-Type"); got != "application/yaml; charset=utf-8" {
			t.Fatalf("Content-Type = %q, want Clash YAML", got)
		}
	})

	t.Run("disabled setting preserves raw base64", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://sub.example.com/sub/s1", nil)
		req.Header.Set("User-Agent", "Clash-Verge/v2.4.2")
		resp := httptest.NewRecorder()

		newSubscriptionTestRouter(subscriptionTestRouterConfig{}).ServeHTTP(resp, req)

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

		newSubscriptionTestRouter(subscriptionTestRouterConfig{
			clashAutoDetect:     true,
			clashUserAgentRegex: `(?i)^custom-client/`,
		}).ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
		}
		if got := resp.Header().Get("Content-Type"); got == "application/yaml; charset=utf-8" {
			t.Fatalf("Content-Type = %q, custom regex should preserve raw response", got)
		}
	})

	t.Run("unrecognized client preserves raw base64", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://sub.example.com/sub/s1", nil)
		req.Header.Set("User-Agent", "GenericClient/1.10.0")
		resp := httptest.NewRecorder()

		newSubscriptionTestRouter(subscriptionTestRouterConfig{
			clashAutoDetect: true,
			jsonAutoDetect:  true,
		}).ServeHTTP(resp, req)

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

	t.Run("recognized Xray JSON client receives configuration array", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://sub.example.com/sub/s1", nil)
		req.Header.Set("User-Agent", "JsonClient/1.6.32")
		resp := httptest.NewRecorder()

		newSubscriptionTestRouter(subscriptionTestRouterConfig{
			jsonAutoDetect:     true,
			jsonUserAgentRegex: `(?i)^jsonclient([ /]|$)`,
		}).ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
		}
		if got := resp.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Fatalf("Content-Type = %q, want JSON", got)
		}
		if body := strings.TrimSpace(resp.Body.String()); !strings.HasPrefix(body, "[") || !strings.Contains(body, `"outbounds"`) {
			t.Fatalf("auto-detected body is not an Xray JSON configuration array:\n%s", body)
		}
	})

	t.Run("explicit JSON endpoint preserves legacy single object by default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://sub.example.com/json/s1", nil)
		resp := httptest.NewRecorder()

		newSubscriptionTestRouter(subscriptionTestRouterConfig{}).ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
		}
		if got := resp.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
			t.Fatalf("Content-Type = %q, want legacy text/plain", got)
		}
		if body := strings.TrimSpace(resp.Body.String()); !strings.HasPrefix(body, "{") {
			t.Fatalf("legacy explicit JSON body is not an object: %s", body)
		}
	})

	t.Run("explicit JSON endpoint can follow XTLS array standard", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://sub.example.com/json/s1", nil)
		resp := httptest.NewRecorder()

		newSubscriptionTestRouter(subscriptionTestRouterConfig{jsonAlwaysArray: true}).ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
		}
		if got := resp.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
			t.Fatalf("Content-Type = %q, want legacy text/plain", got)
		}
		if body := strings.TrimSpace(resp.Body.String()); !strings.HasPrefix(body, "[") {
			t.Fatalf("standards-compliant explicit JSON body is not an array: %s", body)
		}
	})
}

func TestFormatEndpointsRawViewBypassesBrowserPage(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "raw", 4481, 1, `{"network":"tcp","security":"none"}`)
	gin.SetMode(gin.TestMode)
	oldDistFS := distFS
	distFS = testDistFS
	t.Cleanup(func() { distFS = oldDistFS })
	router := newSubscriptionTestRouter(subscriptionTestRouterConfig{})

	tests := []struct {
		name         string
		path         string
		contentType  string
		disposition  string
		bodyContains string
	}{
		{name: "JSON", path: "/json/s1?view=raw", contentType: "application/json; charset=utf-8", disposition: `attachment; filename="subscription.json"`, bodyContains: "outbounds"},
		{name: "Clash", path: "/clash/s1?view=raw", contentType: "application/yaml; charset=utf-8", disposition: `attachment; filename="subscription.yaml"`, bodyContains: "proxies:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://sub.example.com"+tt.path, nil)
			req.Header.Set("Accept", "text/html")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
			}
			if got := resp.Header().Get("Content-Type"); got != tt.contentType {
				t.Fatalf("Content-Type = %q, want %q", got, tt.contentType)
			}
			if got := resp.Header().Get("Content-Disposition"); got != tt.disposition {
				t.Fatalf("Content-Disposition = %q, want %q", got, tt.disposition)
			}
			if !strings.Contains(resp.Body.String(), tt.bodyContains) {
				t.Fatalf("raw body does not contain %q: %s", tt.bodyContains, resp.Body.String())
			}
		})
	}

	for _, path := range []string{"/json/s1", "/clash/s1"} {
		t.Run(path+" browser page", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://sub.example.com"+path, nil)
			req.Header.Set("Accept", "text/html")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
			}
			if got := resp.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Fatalf("Content-Type = %q, want HTML", got)
			}
		})
	}
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
