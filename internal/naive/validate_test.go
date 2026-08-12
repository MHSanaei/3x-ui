package naive

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestValidateProxyURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{name: "https", raw: "https://user:pass@example.com:443"},
		{name: "quic", raw: "quic://user:pass@example.com:443"},
		{name: "http", raw: "http://user:pass@example.com:80"},
		{name: "IPv6", raw: "https://user:pass@[2001:db8::1]:443"},
		{name: "empty", raw: "", wantErr: "proxy URL is required"},
		{name: "unsupported scheme", raw: "file:///etc/passwd", wantErr: "unsupported proxy scheme"},
		{name: "missing host", raw: "https:///proxy", wantErr: "proxy host is required"},
		{name: "zero port", raw: "https://example.com:0", wantErr: "invalid proxy port"},
		{name: "oversized port", raw: "https://example.com:65536", wantErr: "invalid proxy port"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProxyURL(tt.raw)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateProxyURL(%q) error = %v", tt.raw, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateProxyURL(%q) unexpectedly succeeded", tt.raw)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("ValidateProxyURL(%q) error = %q, want %q", tt.raw, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTag(t *testing.T) {
	if err := ValidateTag("naive_tag-1"); err != nil {
		t.Fatalf("ValidateTag(valid) error = %v", err)
	}
	for _, invalid := range []string{"", "../../../etc", "; DROP TABLE naive_outbounds; --", strings.Repeat("a", 65)} {
		err := ValidateTag(invalid)
		if err == nil {
			t.Fatalf("ValidateTag(%q) unexpectedly succeeded", invalid)
		}
		if err.Error() != "invalid naive tag" {
			t.Fatalf("ValidateTag(%q) error = %q", invalid, err)
		}
	}
}

func TestValidateVersion(t *testing.T) {
	if err := ValidateVersion("v150.0.7871.63-1"); err != nil {
		t.Fatalf("ValidateVersion(valid) error = %v", err)
	}
	for _, invalid := range []string{"latest", "150.0.7871.63-1", "v150", "v150.0.7871-1"} {
		err := ValidateVersion(invalid)
		if err == nil {
			t.Fatalf("ValidateVersion(%q) unexpectedly succeeded", invalid)
		}
		if err.Error() != "invalid naive version" {
			t.Fatalf("ValidateVersion(%q) error = %q", invalid, err)
		}
	}
}

func FuzzValidateProxyURL(f *testing.F) {
	for _, seed := range []string{
		"https://user:pass@example.com:443",
		"quic://user:pass@example.com:443",
		"http://user:pass@example.com:80",
		"https://user:pass@[2001:db8::1]:443",
		"file:///etc/passwd",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if ValidateProxyURL(raw) != nil {
			return
		}
		parsed, err := url.Parse(strings.TrimSpace(raw))
		if err != nil {
			t.Fatalf("accepted URL no longer parses: %v", err)
		}
		switch strings.ToLower(parsed.Scheme) {
		case "https", "quic", "http":
		default:
			t.Fatalf("accepted unsupported scheme %q", parsed.Scheme)
		}
		if parsed.Hostname() == "" {
			t.Fatal("accepted empty hostname")
		}
		if rawPort := parsed.Port(); rawPort != "" {
			port, err := strconv.Atoi(rawPort)
			if err != nil || port < 1 || port > 65535 {
				t.Fatalf("accepted invalid port %q", rawPort)
			}
		}
	})
}
