package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestShouldSkipLegacyUnencryptedOutboundRejection(t *testing.T) {
	prohibited := errors.New("vless without TLS or other encryption is prohibited unless the server address is a private IP or domain")

	tests := []struct {
		name    string
		version string
		err     error
		want    bool
	}{
		{name: "older core", version: "26.4.25", err: prohibited, want: true},
		{name: "boundary version", version: "26.7.11", err: prohibited, want: false},
		{name: "newer core", version: "26.10.0", err: prohibited, want: false},
		{name: "unknown version", version: "Unknown", err: prohibited, want: false},
		{name: "unparseable version", version: "26.7", err: prohibited, want: false},
		{name: "empty version", version: "", err: prohibited, want: false},
		{name: "unrelated validation error", version: "26.4.25", err: errors.New("invalid outbound"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSkipLegacyUnencryptedOutboundRejection(tt.version, tt.err); got != tt.want {
				t.Fatalf("shouldSkipLegacyUnencryptedOutboundRejection(%q, %v) = %v, want %v", tt.version, tt.err, got, tt.want)
			}
		})
	}
}

func TestCompareXrayCoreVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
		ok   bool
	}{
		{a: "26.7.11", b: "26.7.11", want: 0, ok: true},
		{a: "26.10.0", b: "26.7.11", want: 1, ok: true},
		{a: "v26.4.25", b: "26.7.11", want: -1, ok: true},
		{a: "Unknown", b: "26.7.11", want: 0, ok: false},
	}

	for _, tt := range tests {
		got, ok := compareXrayCoreVersions(tt.a, tt.b)
		if got != tt.want || ok != tt.ok {
			t.Errorf("compareXrayCoreVersions(%q, %q) = (%d, %v), want (%d, %v)", tt.a, tt.b, got, ok, tt.want, tt.ok)
		}
	}
}

func TestUnwrapXrayTemplateConfig(t *testing.T) {
	real := `{"log":{},"inbounds":[],"outbounds":[],"routing":{}}`

	t.Run("passes through a clean config", func(t *testing.T) {
		if got := UnwrapXrayTemplateConfig(real); got != real {
			t.Fatalf("clean config was modified: %s", got)
		}
	})

	t.Run("passes through invalid JSON unchanged", func(t *testing.T) {
		in := "not json at all"
		if got := UnwrapXrayTemplateConfig(in); got != in {
			t.Fatalf("invalid input was modified: %s", got)
		}
	})

	t.Run("unwraps one layer of response-shaped wrapper", func(t *testing.T) {
		wrapper := `{"inboundTags":["tag"],"outboundTestUrl":"x","xraySetting":` + real + `}`
		got := UnwrapXrayTemplateConfig(wrapper)
		if !equalJSON(t, got, real) {
			t.Fatalf("want %s, got %s", real, got)
		}
	})

	t.Run("unwraps multiple stacked layers", func(t *testing.T) {
		lvl1 := `{"xraySetting":` + real + `}`
		lvl2 := `{"xraySetting":` + lvl1 + `}`
		lvl3 := `{"xraySetting":` + lvl2 + `}`
		got := UnwrapXrayTemplateConfig(lvl3)
		if !equalJSON(t, got, real) {
			t.Fatalf("want %s, got %s", real, got)
		}
	})

	t.Run("handles an xraySetting stored as a JSON-encoded string", func(t *testing.T) {
		encoded, _ := json.Marshal(real) // becomes a quoted string
		wrapper := `{"xraySetting":` + string(encoded) + `}`
		got := UnwrapXrayTemplateConfig(wrapper)
		if !equalJSON(t, got, real) {
			t.Fatalf("want %s, got %s", real, got)
		}
	})

	t.Run("does not unwrap when top level already has real xray keys", func(t *testing.T) {
		// Pathological but defensible: if a user's actual config somehow
		// has both the real keys and an unrelated `xraySetting` key, we
		// must not strip it.
		in := `{"inbounds":[],"xraySetting":{"some":"thing"}}`
		got := UnwrapXrayTemplateConfig(in)
		if got != in {
			t.Fatalf("should have left real config alone, got %s", got)
		}
	})

	t.Run("stops at a reasonable depth", func(t *testing.T) {
		// Build a deeper-than-maxDepth chain that ends at something
		// non-wrapped, and confirm we end up at some valid JSON (we
		// don't loop forever and we don't blow the stack).
		s := real
		for range 16 {
			s = `{"xraySetting":` + s + `}`
		}
		got := UnwrapXrayTemplateConfig(s)
		if !strings.Contains(got, `"inbounds"`) && !strings.Contains(got, `"xraySetting"`) {
			t.Fatalf("unexpected tail: %s", got)
		}
	})
}

func equalJSON(t *testing.T, a, b string) bool {
	t.Helper()
	var va, vb any
	if err := json.Unmarshal([]byte(a), &va); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(b), &vb); err != nil {
		return false
	}
	ja, _ := json.Marshal(va)
	jb, _ := json.Marshal(vb)
	return string(ja) == string(jb)
}

// firstRuleOutbound parses the (post-hoisted) config and returns
// routing.rules[0].outboundTag, or "" if anything is missing.
func firstRuleOutbound(t *testing.T, raw string) string {
	t.Helper()
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal cfg: %v", err)
	}
	routing, _ := cfg["routing"].(map[string]any)
	rules, _ := routing["rules"].([]any)
	if len(rules) == 0 {
		return ""
	}
	first, _ := rules[0].(map[string]any)
	tag, _ := first["outboundTag"].(string)
	return tag
}

func TestEnsureStatsRouting_HoistsApiRuleFromMiddle(t *testing.T) {
	// #4113 repro shape: admin added a cascade outbound and put a
	// catch-all routing rule above the api rule. stats query path
	// gets starved by the catch-all unless we hoist the api rule.
	in := `{
		"routing": {
			"rules": [
				{"type":"field","inboundTag":["inbound-vless"],"outboundTag":"vless-cascade"},
				{"type":"field","inboundTag":["api"],"outboundTag":"api"},
				{"type":"field","outboundTag":"blocked","ip":["geoip:private"]}
			]
		}
	}`
	out, err := EnsureStatsRouting(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := firstRuleOutbound(t, out); got != "api" {
		t.Fatalf("api rule should be at index 0 after hoist, got first outboundTag = %q\nfull: %s", got, out)
	}
}

func TestEnsureStatsRouting_NoOpWhenAlreadyFirst(t *testing.T) {
	// Don't churn the JSON when nothing needs fixing — same string in,
	// same string out. Lets the diff in the panel UI stay quiet for
	// well-formed configs.
	in := `{"routing":{"rules":[{"type":"field","inboundTag":["api"],"outboundTag":"api"},{"type":"field","outboundTag":"blocked","ip":["geoip:private"]}]}}`
	out, err := EnsureStatsRouting(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != in {
		t.Fatalf("expected unchanged input, got: %s", out)
	}
}

func TestEnsureStatsRouting_InsertsDefaultWhenMissing(t *testing.T) {
	// Some admins delete the api rule by accident. Re-add a default
	// at the front so stats keep working after the next save.
	in := `{"routing":{"rules":[{"type":"field","outboundTag":"vless-cascade","inboundTag":["inbound-vless"]}]}}`
	out, err := EnsureStatsRouting(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := firstRuleOutbound(t, out); got != "api" {
		t.Fatalf("default api rule should be inserted at index 0, got %q\nfull: %s", got, out)
	}
	// The original rule should still be there, just shifted.
	var cfg map[string]any
	json.Unmarshal([]byte(out), &cfg)
	rules := cfg["routing"].(map[string]any)["rules"].([]any)
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules after insert, got %d: %v", len(rules), rules)
	}
}

func TestEnsureStatsRouting_NoRoutingBlock(t *testing.T) {
	// Pathological but possible: empty config or one without a routing
	// section. Don't crash, and create the section with the api rule.
	in := `{"log":{}}`
	out, err := EnsureStatsRouting(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := firstRuleOutbound(t, out); got != "api" {
		t.Fatalf("api rule should be created when routing was missing, got %q\nfull: %s", got, out)
	}
}

func TestEnsureStatsRouting_InvalidJsonReturnsAsIs(t *testing.T) {
	// SaveXraySetting calls CheckXrayConfig before this helper, so
	// invalid JSON shouldn't reach us in practice — but be defensive
	// about garbage in (return same garbage out plus an error) so the
	// caller can choose to skip the hoist instead of corrupting input.
	in := "definitely not json"
	out, err := EnsureStatsRouting(in)
	if err == nil {
		t.Fatalf("expected error for invalid json, got none")
	}
	if out != in {
		t.Fatalf("expected raw passthrough on error, got %q", out)
	}
}

func TestEnsureStatsRouting_AcceptsInboundTagAsString(t *testing.T) {
	// Some manually-edited configs use a single string instead of an
	// array for inboundTag. Make sure we still recognize the api rule.
	in := `{"routing":{"rules":[{"type":"field","inboundTag":["other"],"outboundTag":"vless-cascade"},{"type":"field","inboundTag":"api","outboundTag":"api"}]}}`
	out, err := EnsureStatsRouting(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := firstRuleOutbound(t, out); got != "api" {
		t.Fatalf("api rule with string-form inboundTag should hoist to front, got %q\nfull: %s", got, out)
	}
}
