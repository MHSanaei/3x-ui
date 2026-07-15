package naive

import (
	"strings"
	"testing"
)

func TestValidateProxyURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "https", raw: "https://user:pass@example.com", wantErr: false},
		{name: "quic", raw: "quic://user:pass@example.com", wantErr: false},
		{name: "http", raw: "http://user:pass@example.com", wantErr: false},
		{name: "javascript", raw: "javascript:alert(1)", wantErr: true},
		{name: "file", raw: "file:///etc/passwd", wantErr: true},
		{name: "empty", raw: "", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateProxyURL(test.raw)
			if test.wantErr && err == nil {
				t.Fatalf("expected error for %q", test.raw)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", test.raw, err)
			}
		})
	}
}

func TestValidateTag(t *testing.T) {
	if err := ValidateTag("naive_tag-1"); err != nil {
		t.Fatalf("valid tag rejected: %v", err)
	}
	for _, invalid := range []string{"", "../../../etc", "; DROP TABLE naive_outbounds; --", strings.Repeat("a", 65)} {
		if err := ValidateTag(invalid); err == nil {
			t.Fatalf("expected invalid tag error for %q", invalid)
		}
	}
}

func TestValidateVersion(t *testing.T) {
	if err := ValidateVersion("v130.0.6723.91-1"); err != nil {
		t.Fatalf("valid version rejected: %v", err)
	}
	for _, invalid := range []string{"latest", "130.0.6723.91-1", "v130", "v130.0.6723-1"} {
		if err := ValidateVersion(invalid); err == nil {
			t.Fatalf("expected invalid version error for %q", invalid)
		}
	}
}

func FuzzValidateProxyURL(f *testing.F) {
	for _, seed := range []string{
		"https://user:pass@example.com",
		"quic://user:pass@example.com",
		"http://user:pass@example.com",
		"javascript:alert(1)",
		"file:///etc/passwd",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		err := ValidateProxyURL(raw)
		allowed := strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "quic://") || strings.HasPrefix(raw, "http://")
		if raw != "" && !allowed && err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	})
}
