package clientip

import "testing"

func TestResolve(t *testing.T) {
	tests := []struct {
		name                     string
		remoteAddr, xRealIP, xff string
		trusted                  string
		want                     string
	}{
		{name: "untrusted peer cannot set headers", remoteAddr: "203.0.113.7:4434", xRealIP: "8.8.8.8", xff: "9.9.9.9", trusted: "127.0.0.1/32", want: "203.0.113.7"},
		{name: "first untrusted hop behind two proxies", remoteAddr: "127.0.0.1:5555", xff: "203.0.113.9, 10.1.1.1", trusted: "127.0.0.1/32,10.0.0.0/8", want: "203.0.113.9"},
		{name: "client-sent leftmost entry is ignored", remoteAddr: "127.0.0.1:5555", xff: "6.6.6.6, 203.0.113.9, 10.1.1.1", trusted: "127.0.0.1/32,10.0.0.0/8", want: "203.0.113.9"},
		{name: "real IP agreeing with the chain", remoteAddr: "127.0.0.1:5555", xRealIP: "198.51.100.4", xff: "198.51.100.4", trusted: "127.0.0.1/32", want: "198.51.100.4"},
		{name: "real IP contradicting the chain falls back to the proxy", remoteAddr: "127.0.0.1:5555", xRealIP: "6.6.6.6", xff: "198.51.100.4", trusted: "127.0.0.1/32", want: "127.0.0.1"},
		{name: "real IP alone from a trusted proxy", remoteAddr: "127.0.0.1:5555", xRealIP: "198.51.100.4", trusted: "127.0.0.1/32", want: "198.51.100.4"},
		{name: "malformed headers fall back to the proxy", remoteAddr: "127.0.0.1:5555", xRealIP: "also-garbage", xff: "garbage", trusted: "127.0.0.1/32", want: "127.0.0.1"},
		{name: "bare address in the trusted list", remoteAddr: "[::1]:5555", xff: "198.51.100.4", trusted: "127.0.0.1/32, ::1", want: "198.51.100.4"},
		{name: "unparseable trusted entry is skipped", remoteAddr: "10.2.3.4:5555", xff: "198.51.100.4", trusted: "not-an-ip,10.0.0.0/8", want: "198.51.100.4"},
		{name: "unparseable peer has no identity", remoteAddr: "not-an-address", trusted: "127.0.0.1/32", want: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Resolve(tt.remoteAddr, tt.xRealIP, tt.xff, tt.trusted); got != tt.want {
				t.Fatalf("Resolve(%q, %q, %q, %q) = %q, want %q", tt.remoteAddr, tt.xRealIP, tt.xff, tt.trusted, got, tt.want)
			}
		})
	}
}
