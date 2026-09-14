package database

import (
	"encoding/json"
	"strings"
	"testing"

	corelog "github.com/xtls/xray-core/common/log"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestRewriteFreedomDomainStrategy(t *testing.T) {
	tests := []struct {
		name         string
		raw          string
		wantChanged  bool
		wantOutbound map[string]any
	}{
		{
			name:        "the deprecated settings key moves to sockopt",
			raw:         `{"outbounds":[{"protocol":"freedom","tag":"direct","settings":{"domainStrategy":"UseIPv4","finalRules":[{"action":"allow"}]}}]}`,
			wantChanged: true,
			wantOutbound: map[string]any{
				"protocol": "freedom", "tag": "direct",
				"settings":       map[string]any{"finalRules": []any{map[string]any{"action": "allow"}}},
				"streamSettings": map[string]any{"sockopt": map[string]any{"domainStrategy": "UseIPv4"}},
			},
		},
		{
			name:        "the outbound-root targetStrategy moves to sockopt and is dropped",
			raw:         `{"outbounds":[{"protocol":"freedom","tag":"direct","targetStrategy":"ForceIPv6","settings":{}}]}`,
			wantChanged: true,
			wantOutbound: map[string]any{
				"protocol": "freedom", "tag": "direct", "settings": map[string]any{},
				"streamSettings": map[string]any{"sockopt": map[string]any{"domainStrategy": "ForceIPv6"}},
			},
		},
		{
			name:        "the root key wins over the settings key, as in the core",
			raw:         `{"outbounds":[{"protocol":"freedom","tag":"direct","targetStrategy":"UseIPv4","settings":{"domainStrategy":"UseIPv6"}}]}`,
			wantChanged: true,
			wantOutbound: map[string]any{
				"protocol": "freedom", "tag": "direct", "settings": map[string]any{},
				"streamSettings": map[string]any{"sockopt": map[string]any{"domainStrategy": "UseIPv4"}},
			},
		},
		{
			name:        "the settings targetStrategy wins over domainStrategy",
			raw:         `{"outbounds":[{"protocol":"freedom","tag":"direct","settings":{"targetStrategy":"UseIPv6","domainStrategy":"UseIPv4"}}]}`,
			wantChanged: true,
			wantOutbound: map[string]any{
				"protocol": "freedom", "tag": "direct", "settings": map[string]any{},
				"streamSettings": map[string]any{"sockopt": map[string]any{"domainStrategy": "UseIPv6"}},
			},
		},
		{
			name:        "an AsIs alias is dropped and leaves the sockopt value alone",
			raw:         `{"outbounds":[{"protocol":"freedom","tag":"direct","settings":{"domainStrategy":"AsIs"},"streamSettings":{"sockopt":{"domainStrategy":"UseIPv6","tcpFastOpen":true}}}]}`,
			wantChanged: true,
			wantOutbound: map[string]any{
				"protocol": "freedom", "tag": "direct", "settings": map[string]any{},
				"streamSettings": map[string]any{"sockopt": map[string]any{"domainStrategy": "UseIPv6", "tcpFastOpen": true}},
			},
		},
		{
			name:        "the existing sockopt spelling is preserved",
			raw:         `{"outbounds":[{"protocol":"freedom","tag":"direct","settings":{"domainStrategy":"useipv4v6"}}]}`,
			wantChanged: true,
			wantOutbound: map[string]any{
				"protocol": "freedom", "tag": "direct", "settings": map[string]any{},
				"streamSettings": map[string]any{"sockopt": map[string]any{"domainStrategy": "useipv4v6"}},
			},
		},
		{
			name:        "a strategy the core refuses is dropped rather than moved",
			raw:         `{"outbounds":[{"protocol":"freedom","tag":"direct","settings":{"domainStrategy":"UseIPv5"}}]}`,
			wantChanged: true,
			wantOutbound: map[string]any{
				"protocol": "freedom", "tag": "direct", "settings": map[string]any{},
			},
		},
		{
			name:        "other protocols keep their outbound-root targetStrategy",
			raw:         `{"outbounds":[{"protocol":"vless","tag":"proxy","targetStrategy":"UseIPv4","settings":{}}]}`,
			wantChanged: false,
			wantOutbound: map[string]any{
				"protocol": "vless", "tag": "proxy", "targetStrategy": "UseIPv4",
				"settings": map[string]any{},
			},
		},
		{
			name:        "a freedom outbound without a strategy is left untouched",
			raw:         `{"outbounds":[{"protocol":"freedom","tag":"direct","settings":{"finalRules":[{"action":"allow"}]}}]}`,
			wantChanged: false,
			wantOutbound: map[string]any{
				"protocol": "freedom", "tag": "direct",
				"settings": map[string]any{"finalRules": []any{map[string]any{"action": "allow"}}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			updated, changed, err := rewriteFreedomDomainStrategy(tc.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if changed != tc.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tc.wantChanged)
			}
			var cfg struct {
				Outbounds []map[string]any `json:"outbounds"`
			}
			if err := json.Unmarshal([]byte(updated), &cfg); err != nil {
				t.Fatalf("rewritten template is not JSON: %v", err)
			}
			if len(cfg.Outbounds) != 1 {
				t.Fatalf("got %d outbounds, want 1", len(cfg.Outbounds))
			}
			got, _ := json.Marshal(cfg.Outbounds[0])
			want, _ := json.Marshal(tc.wantOutbound)
			if string(got) != string(want) {
				t.Fatalf("outbound = %s, want %s", got, want)
			}
		})
	}
}

type coreLogCapture struct{ msgs []string }

func (c *coreLogCapture) Handle(msg corelog.Message) { c.msgs = append(c.msgs, msg.String()) }

func (c *coreLogCapture) has(sub string) bool {
	return strings.Contains(strings.Join(c.msgs, "\n"), sub)
}

type discardLogHandler struct{}

func (discardLogHandler) Handle(corelog.Message) {}

// captureCoreLogs takes over the vendored core's log sink for the duration of
// one test, which is the only way to observe a config-load warning.
func captureCoreLogs(t *testing.T) *coreLogCapture {
	t.Helper()
	capture := new(coreLogCapture)
	corelog.RegisterHandler(capture)
	t.Cleanup(func() { corelog.RegisterHandler(discardLogHandler{}) })
	return capture
}

// Drives the real core: a rewrite that dropped the value instead of moving it
// would leave the config warning on every load and fail here.
func TestRewriteFreedomDomainStrategySatisfiesCore(t *testing.T) {
	for _, tc := range []struct {
		name      string
		raw       string
		wantValue string
	}{
		{
			name:      "deprecated settings key",
			raw:       `{"protocol":"freedom","tag":"direct","settings":{"domainStrategy":"UseIPv4","finalRules":[{"action":"allow"}]}}`,
			wantValue: `"domainStrategy": "UseIPv4"`,
		},
		{
			name:      "outbound-root targetStrategy",
			raw:       `{"protocol":"freedom","tag":"direct","targetStrategy":"ForceIPv6","settings":{}}`,
			wantValue: `"domainStrategy": "ForceIPv6"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			capture := captureCoreLogs(t)

			if err := xray.ValidateOutboundConfig([]byte(tc.raw)); err != nil {
				t.Fatalf("xray-core must accept the legacy outbound: %v", err)
			}
			if !capture.has("sockopt.domainStrategy") {
				t.Fatal("expected the core to warn about the legacy strategy placement")
			}

			updated, changed, err := rewriteFreedomDomainStrategy(
				`{"outbounds":[` + tc.raw + `]}`,
			)
			if err != nil || !changed {
				t.Fatalf("rewrite: changed=%v err=%v", changed, err)
			}
			var after struct {
				Outbounds []json.RawMessage `json:"outbounds"`
			}
			if err := json.Unmarshal([]byte(updated), &after); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(after.Outbounds[0]), tc.wantValue) {
				t.Fatalf("rewritten outbound = %s, want it to carry %s", after.Outbounds[0], tc.wantValue)
			}

			capture.msgs = nil
			if err := xray.ValidateOutboundConfig(after.Outbounds[0]); err != nil {
				t.Fatalf("xray-core refused the rewritten outbound: %v", err)
			}
			if capture.has("sockopt.domainStrategy") {
				t.Fatalf("rewritten outbound still warns on load: %v", capture.msgs)
			}
		})
	}
}

func TestRewriteFreedomDomainStrategyInvalidJSON(t *testing.T) {
	_, changed, err := rewriteFreedomDomainStrategy("{not json")
	if err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
	if changed {
		t.Fatal("invalid JSON must not report a change")
	}
}

func TestMigrateFreedomDomainStrategyRewritesStoredTemplate(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	// A CGO_ENABLED=0 build links a stubbed driver, so this test needs the same
	// C compiler the rest of the package's DB tests do.
	if err := InitDB(config.GetDBPath()); err != nil {
		if strings.Contains(err.Error(), "CGO_ENABLED=0") {
			t.Skipf("sqlite needs cgo: %v", err)
		}
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	legacy := `{"outbounds":[{"protocol":"freedom","tag":"direct","settings":{"domainStrategy":"UseIPv4"}}]}`
	seedTemplate(t, legacy)
	if err := db.Where("seeder_name = ?", "FreedomDomainStrategyFix").
		Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatalf("clear seeder history: %v", err)
	}

	if err := migrateFreedomDomainStrategy(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	got := storedTemplate(t)
	if !strings.Contains(got, `"sockopt"`) {
		t.Fatalf("stored template = %s, want the strategy in sockopt", got)
	}
	if strings.Contains(got, `"domainStrategy"`) {
		t.Fatalf("stored template = %s, want the deprecated key gone", got)
	}

	// The history gate is what keeps a hand-edited template from being rewritten
	// again on every restart, so run the real seeder list over a fresh legacy one.
	seedTemplate(t, legacy)
	if err := runSeeders(false); err != nil {
		t.Fatalf("runSeeders: %v", err)
	}
	if got := storedTemplate(t); got != legacy {
		t.Errorf("a completed seeder rewrote the template again: %s", got)
	}
}

func seedTemplate(t *testing.T, value string) {
	t.Helper()
	if err := db.Where("key = ?", "xrayTemplateConfig").Delete(&model.Setting{}).Error; err != nil {
		t.Fatalf("clear template: %v", err)
	}
	if err := db.Create(&model.Setting{Key: "xrayTemplateConfig", Value: value}).Error; err != nil {
		t.Fatalf("seed template: %v", err)
	}
}

func storedTemplate(t *testing.T) string {
	t.Helper()
	var setting model.Setting
	if err := db.Where("key = ?", "xrayTemplateConfig").First(&setting).Error; err != nil {
		t.Fatalf("reload template: %v", err)
	}
	return setting.Value
}
