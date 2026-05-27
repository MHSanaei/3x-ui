package netproxy

import (
	"net/http"
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
			if tc.wantProxy {
				transport, ok := client.Transport.(*http.Transport)
				if !ok || transport.Proxy == nil {
					t.Errorf("expected transport with Proxy set for %q", tc.proxyURL)
				}
			}
			if tc.wantDial {
				transport, ok := client.Transport.(*http.Transport)
				if !ok || transport.DialContext == nil {
					t.Errorf("expected transport with DialContext set for %q", tc.proxyURL)
				}
			}
		})
	}
}
