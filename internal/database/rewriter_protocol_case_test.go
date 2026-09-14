package database

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestRewriteRemovedOutboundKeysSeesAnUppercaseFreedom(t *testing.T) {
	raw := `{"outbounds":[{"protocol":"Freedom","tag":"direct","settings":{},"streamSettings":{"sockopt":{"addressPortStrategy":"SrvPortOnly"}}}]}`
	var before struct {
		Outbounds []json.RawMessage `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(raw), &before); err != nil {
		t.Fatal(err)
	}
	// A refusal here is the proof the core treated it as freedom, not as unknown.
	if err := xray.ValidateOutboundConfig(before.Outbounds[0]); err == nil {
		t.Fatal("expected the vendored core to refuse the legacy addressPortStrategy")
	}

	updated, changed, err := rewriteRemovedOutboundKeys(raw)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !changed {
		t.Fatal(`an outbound spelled "Freedom" was left with the key the core refuses`)
	}
	var after struct {
		Outbounds []json.RawMessage `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(updated), &after); err != nil {
		t.Fatal(err)
	}
	if err := xray.ValidateOutboundConfig(after.Outbounds[0]); err != nil {
		t.Fatalf("rewritten outbound still refused by xray-core: %v", err)
	}
}

// The core lowercases a protocol id before looking up its handler, so a config
// that runs as freedom must migrate the same whether it says Freedom or freedom.
func TestRewritersTreatProtocolCaseAlike(t *testing.T) {
	tests := []struct {
		name    string
		rewrite func(string) (string, bool, error)
		raw     string
	}{
		{
			name:    "removed outbound keys",
			rewrite: rewriteRemovedOutboundKeys,
			raw:     `{"outbounds":[{"protocol":"freedom","tag":"direct","settings":{},"streamSettings":{"sockopt":{"addressPortStrategy":"SrvPortOnly"}}}]}`,
		},
		{
			name:    "freedom final rules reverse",
			rewrite: rewriteFreedomFinalRules,
			raw:     `{"outbounds":[{"protocol":"freedom","tag":"direct","settings":{"finalRules":[{"action":"allow","ip":["geoip:private"]}]}}]}`,
		},
		{
			name:    "freedom private egress block",
			rewrite: rewriteFreedomFinalRulesPrivateEgress,
			raw:     `{"outbounds":[{"protocol":"freedom","tag":"direct","settings":{"finalRules":[]}}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, wantChanged, err := tt.rewrite(tt.raw)
			if err != nil || !wantChanged {
				t.Fatalf("lowercase rewrite: changed=%v err=%v", wantChanged, err)
			}
			upper := strings.Replace(tt.raw, `"protocol":"freedom"`, `"protocol":"Freedom"`, 1)
			got, changed, err := tt.rewrite(upper)
			if err != nil {
				t.Fatalf("uppercase rewrite: %v", err)
			}
			if !changed {
				t.Fatalf(`the rewrite skipped an outbound spelled "Freedom"`)
			}
			if loweredGot, loweredWant := lowercaseProtocol(t, got), lowercaseProtocol(t, want); loweredGot != loweredWant {
				t.Fatalf("uppercase result differs from lowercase:\n got %s\nwant %s", loweredGot, loweredWant)
			}
		})
	}
}

// The rewriters keep the spelling they were given; only the migrated keys may
// differ, so the comparison ignores the case of the protocol id itself.
func lowercaseProtocol(t *testing.T, raw string) string {
	t.Helper()
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	outbounds, _ := cfg["outbounds"].([]any)
	for _, ob := range outbounds {
		obj, ok := ob.(map[string]any)
		if !ok {
			continue
		}
		if proto, ok := obj["protocol"].(string); ok {
			obj["protocol"] = strings.ToLower(proto)
		}
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
