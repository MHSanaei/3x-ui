package database

import (
	"encoding/json"
	"testing"
)

func TestRewriteFreedomFinalRulesPrivateEgress(t *testing.T) {
	hardened := []any{
		map[string]any{"action": "block", "ip": []any{"geoip:private"}},
		map[string]any{"action": "allow"},
	}

	tests := []struct {
		name        string
		raw         string
		wantChanged bool
		wantRules   []any
	}{
		{
			name:        "allow-only default is hardened",
			raw:         `{"outbounds":[{"protocol":"freedom","settings":{"domainStrategy":"AsIs","finalRules":[{"action":"allow"}]},"tag":"direct"}]}`,
			wantChanged: true,
			wantRules:   hardened,
		},
		{
			name:        "missing finalRules is hardened",
			raw:         `{"outbounds":[{"protocol":"freedom","settings":{"domainStrategy":"AsIs"},"tag":"direct"}]}`,
			wantChanged: true,
			wantRules:   hardened,
		},
		{
			name:        "null finalRules is hardened",
			raw:         `{"outbounds":[{"protocol":"freedom","settings":{"domainStrategy":"AsIs","finalRules":null},"tag":"direct"}]}`,
			wantChanged: true,
			wantRules:   hardened,
		},
		{
			name:        "empty finalRules is hardened",
			raw:         `{"outbounds":[{"protocol":"freedom","settings":{"domainStrategy":"AsIs","finalRules":[]},"tag":"direct"}]}`,
			wantChanged: true,
			wantRules:   hardened,
		},
		{
			name:        "legacy private-only allow is hardened",
			raw:         `{"outbounds":[{"protocol":"freedom","settings":{"finalRules":[{"action":"allow","ip":["geoip:private"]}]},"tag":"direct"}]}`,
			wantChanged: true,
			wantRules:   hardened,
		},
		{
			name:        "customized rules are preserved",
			raw:         `{"outbounds":[{"protocol":"freedom","settings":{"finalRules":[{"action":"block","ip":["1.2.3.4"]},{"action":"allow"}]},"tag":"direct"}]}`,
			wantChanged: false,
		},
		{
			name:        "non-freedom outbounds are ignored",
			raw:         `{"outbounds":[{"protocol":"blackhole","settings":{},"tag":"blocked"}]}`,
			wantChanged: false,
		},
		{
			name:        "empty config is untouched",
			raw:         "",
			wantChanged: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			updated, changed, err := rewriteFreedomFinalRulesPrivateEgress(tc.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if changed != tc.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tc.wantChanged)
			}
			if !tc.wantChanged {
				if updated != tc.raw {
					t.Fatalf("raw config mutated without change flag:\n%s", updated)
				}
				return
			}
			var cfg map[string]any
			if err := json.Unmarshal([]byte(updated), &cfg); err != nil {
				t.Fatalf("updated config is not valid json: %v", err)
			}
			outbounds := cfg["outbounds"].([]any)
			settings := outbounds[0].(map[string]any)["settings"].(map[string]any)
			gotRules, _ := json.Marshal(settings["finalRules"])
			wantRules, _ := json.Marshal(tc.wantRules)
			if string(gotRules) != string(wantRules) {
				t.Fatalf("finalRules = %s, want %s", gotRules, wantRules)
			}
		})
	}
}

func TestRewriteFreedomFinalRulesPreservesSplitRouting(t *testing.T) {
	const raw = `{
		"outbounds":[{"protocol":"freedom","settings":{"domainStrategy":"AsIs"},"tag":"direct"}],
		"routing":{"domainStrategy":"AsIs","rules":[
			{"type":"field","domain":["regexp:.*\\.ru$"],"outboundTag":"direct"},
			{"type":"field","network":"tcp,udp","outboundTag":"proxy"}
		]}
	}`
	updated, changed, err := rewriteFreedomFinalRulesPrivateEgress(raw)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !changed {
		t.Fatal("missing finalRules must be hardened")
	}
	var before, after map[string]any
	if err := json.Unmarshal([]byte(raw), &before); err != nil {
		t.Fatalf("decode before: %v", err)
	}
	if err := json.Unmarshal([]byte(updated), &after); err != nil {
		t.Fatalf("decode after: %v", err)
	}
	beforeRouting, _ := json.Marshal(before["routing"])
	afterRouting, _ := json.Marshal(after["routing"])
	if string(afterRouting) != string(beforeRouting) {
		t.Fatalf("split routing changed:\n got %s\nwant %s", afterRouting, beforeRouting)
	}
	outbound := after["outbounds"].([]any)[0].(map[string]any)
	settings := outbound["settings"].(map[string]any)
	if settings["domainStrategy"] != "AsIs" {
		t.Fatalf("freedom domainStrategy=%v want AsIs", settings["domainStrategy"])
	}
}

func TestRewriteFreedomFinalRulesPrivateEgressInvalidJSON(t *testing.T) {
	_, changed, err := rewriteFreedomFinalRulesPrivateEgress("{not json")
	if err == nil {
		t.Fatal("expected a json error for malformed config")
	}
	if changed {
		t.Fatal("malformed config must not report changed")
	}
}
