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

func TestRewriteFreedomFinalRulesPrivateEgressInvalidJSON(t *testing.T) {
	_, changed, err := rewriteFreedomFinalRulesPrivateEgress("{not json")
	if err == nil {
		t.Fatal("expected a json error for malformed config")
	}
	if changed {
		t.Fatal("malformed config must not report changed")
	}
}
