package controller

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Cookies do not separate ports or schemes, so another service on the panel's host name
// must not be able to open the admin's live-update socket from the admin's browser.
func TestCheckSameOriginComparesSchemeAndPort(t *testing.T) {
	for _, tc := range []struct {
		name, host, origin string
		tls                bool
		want               bool
	}{
		{"no origin header", "panel.example.com:2053", "", false, true},
		{"same host and port", "panel.example.com:2053", "http://panel.example.com:2053", false, true},
		{"default port omitted by the browser", "panel.example.com:443", "https://panel.example.com", true, true},
		{"behind a proxy without a port", "panel.example.com", "https://panel.example.com", false, true},
		{"bare IPv6 host", "[::1]", "http://[::1]", false, true},
		{"another port on the same host", "panel.example.com:2053", "http://panel.example.com:8080", false, false},
		{"plain http origin on a TLS panel", "panel.example.com:2053", "http://panel.example.com:2053", true, false},
		{"non-web scheme", "panel.example.com:2053", "chrome-extension://panel.example.com:2053", false, false},
		{"another host", "panel.example.com:2053", "http://evil.example.com:2053", false, false},
	} {
		req := httptest.NewRequest(http.MethodGet, "/ws", nil)
		req.Host = tc.host
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		if tc.tls {
			req.TLS = &tls.ConnectionState{}
		} else {
			req.TLS = nil
		}
		if got := checkSameOrigin(req); got != tc.want {
			t.Errorf("%s: checkSameOrigin(Host %q, Origin %q) = %v, want %v", tc.name, tc.host, tc.origin, got, tc.want)
		}
	}
}
