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

func TestParseRemoteRoutingURLKeepsInlineCompatibility(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantSource string
		wantRemote bool
		wantErr    bool
	}{
		{name: "deeplink stays inline", input: "happ://routing/onadd/abc"},
		{name: "plain HTTP stays inline", input: "http://example.com/rules"},
		{name: "multiline stays inline", input: "https://example.com/rules\nMATCH,PROXY"},
		{name: "HTTPS source", input: "  https://example.com/rules#ignored  ", wantSource: "https://example.com/rules", wantRemote: true},
		{name: "uppercase scheme", input: "HTTPS://example.com/rules", wantSource: "https://example.com/rules", wantRemote: true},
		{name: "credentials rejected", input: "https://user:pass@example.com/rules", wantRemote: true, wantErr: true},
		{name: "missing host rejected", input: "https:///rules", wantRemote: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, remote, err := ParseRemoteRoutingURL(tt.input)
			if got != tt.wantSource || remote != tt.wantRemote || (err != nil) != tt.wantErr {
				t.Fatalf("got=%q remote=%v err=%v", got, remote, err)
			}
		})
	}
}
