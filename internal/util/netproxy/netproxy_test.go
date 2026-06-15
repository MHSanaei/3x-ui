package netproxy

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestNewHTTPClient(t *testing.T) {
	tests := []struct {
		name      string
		proxyURL  string
		wantErr   bool
		wantProxy bool
		wantDial  bool
	}{
		{name: "empty returns direct client", proxyURL: ""},
		{name: "socks5 sets custom dialer", proxyURL: "socks5://127.0.0.1:1080", wantDial: true},
		{name: "socks5 with auth", proxyURL: "socks5://user:pass@127.0.0.1:1080", wantDial: true},
		{name: "http sets transport proxy", proxyURL: "http://127.0.0.1:8080", wantProxy: true},
		{name: "https sets transport proxy", proxyURL: "https://127.0.0.1:8080", wantProxy: true},
		{name: "unsupported scheme errors", proxyURL: "ftp://127.0.0.1:21", wantErr: true},
	}

	// baseTransport clones http.DefaultTransport, whose Proxy and DialContext are already
	// non-nil — so "!= nil" can't prove our proxy/dialer was applied. Check the real values.
	defaultDialPtr := reflect.ValueOf(http.DefaultTransport.(*http.Transport).DialContext).Pointer()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, err := NewHTTPClient(tc.proxyURL, 5*time.Second)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tc.proxyURL)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.proxyURL, err)
			}
			if client.Timeout != 5*time.Second {
				t.Errorf("timeout = %v, want 5s", client.Timeout)
			}
			// Empty proxyURL → a plain direct client with no custom transport.
			if tc.proxyURL == "" {
				if client.Transport != nil {
					t.Errorf("empty proxy must yield a direct client (nil Transport), got %T", client.Transport)
				}
				return
			}
			transport, ok := client.Transport.(*http.Transport)
			if !ok {
				t.Fatalf("transport is %T, want *http.Transport", client.Transport)
			}
			if tc.wantProxy {
				// Prove the CONFIGURED proxy is applied: transport.Proxy(req) must
				// return our URL, not the cloned default's ProxyFromEnvironment.
				req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
				u, perr := transport.Proxy(req)
				if perr != nil {
					t.Fatalf("transport.Proxy returned error: %v", perr)
				}
				if u == nil || u.String() != tc.proxyURL {
					t.Errorf("transport.Proxy(req) = %v, want %q (configured proxy not applied)", u, tc.proxyURL)
				}
			}
			if tc.wantDial {
				if transport.DialContext == nil {
					t.Fatal("DialContext is nil")
				}
				// Must be the socks5 dialer, not the cloned default DialContext.
				if reflect.ValueOf(transport.DialContext).Pointer() == defaultDialPtr {
					t.Error("DialContext is still the default; socks5 dialer was not applied")
				}
			}
		})
	}
}
