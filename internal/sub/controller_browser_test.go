package sub

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
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

			if got := isBrowserSubscriptionRequest(c); got != tt.want {
				t.Fatalf("isBrowserSubscriptionRequest() = %v, want %v", got, tt.want)
			}
		})
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
