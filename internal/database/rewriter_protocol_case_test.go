package database

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
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

func TestRewriteUppercaseFreedomFinalRules(t *testing.T) {
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
			name:        "stock allow-only rules are hardened",
			raw:         `{"outbounds":[{"protocol":"Freedom","tag":"direct","settings":{"finalRules":[{"action":"allow"}]}}]}`,
			wantChanged: true,
			wantRules:   hardened,
		},
		{
			name:        "legacy private-only allow is hardened",
			raw:         `{"outbounds":[{"protocol":"FREEDOM","tag":"direct","settings":{"finalRules":[{"action":"allow","ip":["geoip:private"]}]}}]}`,
			wantChanged: true,
			wantRules:   hardened,
		},
		{
			name:        "missing finalRules is hardened",
			raw:         `{"outbounds":[{"protocol":"Freedom","tag":"direct","settings":{}}]}`,
			wantChanged: true,
			wantRules:   hardened,
		},
		{
			name:        "customized rules are preserved",
			raw:         `{"outbounds":[{"protocol":"Freedom","tag":"direct","settings":{"finalRules":[{"action":"block","ip":["1.2.3.4"]},{"action":"allow"}]}}]}`,
			wantChanged: false,
		},
		{
			name:        "a canonical spelling was already handled by its own seeder",
			raw:         `{"outbounds":[{"protocol":"freedom","tag":"direct","settings":{"finalRules":[{"action":"allow"}]}}]}`,
			wantChanged: false,
		},
		{
			name:        "another protocol is ignored",
			raw:         `{"outbounds":[{"protocol":"blackhole","tag":"blocked","settings":{}}]}`,
			wantChanged: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			updated, changed, err := rewriteUppercaseFreedomFinalRules(tc.raw)
			if err != nil {
				t.Fatalf("rewrite: %v", err)
			}
			if changed != tc.wantChanged {
				t.Fatalf("changed = %v, want %v (out: %s)", changed, tc.wantChanged, updated)
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

// The two earlier seeders recorded their rows before this predicate existed, so
// this pins the re-run reaching a panel whose history already has both.
func TestUppercaseFreedomFinalRulesFixReachesHistoryGatedPanels(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	if err := InitDB(config.GetDBPath()); err != nil {
		if strings.Contains(err.Error(), "CGO_ENABLED=0") {
			t.Skipf("sqlite needs cgo: %v", err)
		}
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	stock := `{"outbounds":[{"protocol":"Freedom","tag":"direct","settings":{"finalRules":[{"action":"allow"}]}}]}`
	seedTemplate(t, stock)
	// InitDB pre-seeds a fresh install's rows, so the earlier two are already
	// recorded and this seeder has to look unapplied for the run to reach it.
	for _, name := range []string{"FreedomFinalRulesReverseFix", "FreedomFinalRulesPrivateEgressBlock"} {
		var count int64
		if err := db.Model(&model.HistoryOfSeeders{}).
			Where("seeder_name = ?", name).Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
		if count == 0 {
			t.Fatalf("%s is no longer pre-seeded on a fresh install", name)
		}
	}
	if err := db.Where("seeder_name = ?", "UppercaseFreedomFinalRulesFix").
		Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatalf("clear seeder history: %v", err)
	}

	if err := runSeeders(false); err != nil {
		t.Fatalf("runSeeders: %v", err)
	}

	var cfg struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(storedTemplate(t)), &cfg); err != nil {
		t.Fatalf("stored template is not JSON: %v", err)
	}
	settings, _ := cfg.Outbounds[0]["settings"].(map[string]any)
	if settings["finalRules"] == nil {
		t.Fatal("the hardening never reached an outbound spelled Freedom")
	}
	if proto, _ := cfg.Outbounds[0]["protocol"].(string); proto != "Freedom" {
		t.Fatalf("the seeder rewrote the protocol id: %q", proto)
	}
	if hardened := storedTemplate(t); !strings.Contains(hardened, `"geoip:private"`) {
		t.Fatalf("stored finalRules are not the hardened pair: %s", hardened)
	}

	// A second pass must leave the template alone: the row it just wrote gates it.
	after := storedTemplate(t)
	if err := runSeeders(false); err != nil {
		t.Fatalf("second runSeeders: %v", err)
	}
	if got := storedTemplate(t); got != after {
		t.Fatalf("the re-run seed rewrote the template again:\n got %s\nwant %s", got, after)
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
