package sub

import "testing"

func TestApplyVlessRoute(t *testing.T) {
	const id = "11111111-2222-4333-8444-555555555555"
	tests := []struct {
		name  string
		id    string
		route string
		want  string
	}{
		{"empty route unchanged", id, "", id},
		{"whitespace route unchanged", id, "   ", id},
		{"443 -> 01bb", id, "443", "11111111-2222-01bb-8444-555555555555"},
		{"53 -> 0035", id, "53", "11111111-2222-0035-8444-555555555555"},
		{"0 -> 0000", id, "0", "11111111-2222-0000-8444-555555555555"},
		{"65535 -> ffff", id, "65535", "11111111-2222-ffff-8444-555555555555"},
		{"trimmed value", id, "  443 ", "11111111-2222-01bb-8444-555555555555"},
		{"out of range high unchanged", id, "65536", id},
		{"negative unchanged", id, "-1", id},
		{"non-numeric unchanged", id, "abc", id},
		{"legacy multi-segment unchanged", id, "53,443", id},
		{"non-uuid id unchanged", "short", "443", "short"},
		{"empty id unchanged", "", "443", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := applyVlessRoute(tt.id, tt.route); got != tt.want {
				t.Fatalf("applyVlessRoute(%q, %q) = %q, want %q", tt.id, tt.route, got, tt.want)
			}
		})
	}
}

func TestHostVlessRoute(t *testing.T) {
	if got := hostVlessRoute(map[string]any{"vlessRoute": "443"}); got != "443" {
		t.Fatalf(`hostVlessRoute = %q, want "443"`, got)
	}
	if got := hostVlessRoute(map[string]any{}); got != "" {
		t.Fatalf(`hostVlessRoute(missing) = %q, want ""`, got)
	}
}
