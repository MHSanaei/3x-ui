package sub

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

func TestIsBrowserSubscriptionRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name   string
		accept string
		ua     string
		dest   string
		mode   string
		query  string
		want   bool
	}{
		{name: "explicit html query is not implicit navigation", query: "?html=1", want: false},
		{name: "html accept", accept: "text/html,application/xhtml+xml", want: true},
		{name: "browser navigation with wildcard accept", accept: "*/*", ua: "Mozilla/5.0 Safari/605.1.15", dest: "document", mode: "navigate", want: true},
		{name: "browser ua fallback", accept: "*/*", ua: "Mozilla/5.0 Chrome/126.0.0.0", want: true},
		{name: "vpn client wildcard", accept: "*/*", ua: "Incy/3.3.0", want: false},
		{name: "vpn client with mozilla token", accept: "*/*", ua: "Mozilla/5.0 Incy/3.3.0", want: false},
		{name: "plain client", accept: "*/*", ua: "Go-http-client/2.0", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest(http.MethodGet, "/sub/abc"+tt.query, nil)
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			if tt.ua != "" {
				req.Header.Set("User-Agent", tt.ua)
			}
			if tt.dest != "" {
				req.Header.Set("Sec-Fetch-Dest", tt.dest)
			}
			if tt.mode != "" {
				req.Header.Set("Sec-Fetch-Mode", tt.mode)
			}
			c.Request = req

			if got := (&SUBController{}).isBrowserSubscriptionRequest(c); got != tt.want {
				t.Fatalf("isBrowserSubscriptionRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBrowserClassificationHonorsConfiguredFormatMatchers(t *testing.T) {
	cases := []struct {
		name string
		new  func() *SUBController
	}{
		{"clash", func() *SUBController {
			return &SUBController{subClashAutoDetect: true, clashEnabled: true, clashUserAgent: regexp.MustCompile(`Custom-Client`)}
		}},
		{"json", func() *SUBController {
			return &SUBController{jsonAutoDetect: true, jsonEnabled: true, jsonUserAgent: regexp.MustCompile(`Custom-Client`)}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/sub/abc", nil)
			c.Request.Header.Set("User-Agent", "Mozilla/5.0 Custom-Client/1.0")
			if tc.new().isBrowserSubscriptionRequest(c) {
				t.Fatal("configured subscription client was classified as a browser")
			}
		})
	}
}

func TestSubscriptionCopyPageUsesRequestLocale(t *testing.T) {
	bundle := i18n.NewBundle(language.English)
	for id, text := range map[string]string{
		"subCopyPageTitle":        "Titre localisé",
		"subCopyPageHeading":      "En-tête localisé",
		"subCopyPageInstructions": "Instructions localisées",
	} {
		bundle.AddMessages(language.French, &i18n.Message{ID: id, Other: text})
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/sub/abc", nil)
	c.Request.Header.Set("Accept-Language", "fr-FR")
	c.Set("localizer", i18n.NewLocalizer(bundle, "fr-FR"))

	(&SUBController{}).serveSubscriptionCopyPage(c)
	if body := w.Body.String(); !strings.Contains(body, `<html lang="fr-FR">`) ||
		!strings.Contains(body, "Titre localisé") || !strings.Contains(body, "Instructions localisées") {
		t.Fatalf("copy page was not localized from the request: %s", body)
	}
}

func TestExplicitSubPageRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{name: "html=1", query: "?html=1", want: true},
		{name: "view=html", query: "?view=HTML", want: true},
		{name: "no query", query: "", want: false},
		{name: "unrelated query", query: "?format=info", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/sub/abc"+tt.query, nil)

			if got := explicitSubPageRequest(c); got != tt.want {
				t.Fatalf("explicitSubPageRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}
