package common

import "testing"

func TestEnsureURLScheme(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
		{"bare telegram handle", "t.me/xui_support", "https://t.me/xui_support"},
		{"bare domain with path", "example.com/help", "https://example.com/help"},
		{"already https", "https://t.me/xui_support", "https://t.me/xui_support"},
		{"already http", "http://example.com", "http://example.com"},
		{"telegram deep link", "tg://resolve?domain=xui_support", "tg://resolve?domain=xui_support"},
		{"mailto", "mailto:support@example.com", "mailto:support@example.com"},
		{"tel", "tel:+1234567890", "tel:+1234567890"},
		{"trims whitespace", "  t.me/xui_support  ", "https://t.me/xui_support"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EnsureURLScheme(tt.in); got != tt.want {
				t.Errorf("EnsureURLScheme(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
