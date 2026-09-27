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

func TestRewriteDNSOutboundLegacyKeys(t *testing.T) {
	tests := []struct {
		name         string
		raw          string
		wantChanged  bool
		wantOutbound map[string]any
	}{
		{
			name:        "reject keeps the blocked qTypes and answers rCode 5",
			raw:         `{"outbounds":[{"protocol":"dns","tag":"dns-out","settings":{"rewriteNetwork":"udp","rewriteAddress":"8.8.8.8","rewritePort":53,"nonIPQuery":"reject","blockTypes":[65,28]}}]}`,
			wantChanged: true,
			wantOutbound: map[string]any{
				"protocol": "dns", "tag": "dns-out",
				"settings": map[string]any{
					"rewriteNetwork": "udp", "rewriteAddress": "8.8.8.8", "rewritePort": float64(53),
					"rules": []any{
						map[string]any{"action": "return", "qType": "65,28", "rCode": float64(5)},
						map[string]any{"action": "hijack", "qType": "1,28"},
						map[string]any{"action": "return", "rCode": float64(5)},
					},
				},
			},
		},
		{
			name:        "drop with no blocked qTypes keeps only the hijack and the answer",
			raw:         `{"outbounds":[{"protocol":"dns","tag":"dns-out","settings":{"nonIPQuery":"drop","blockTypes":[]}}]}`,
			wantChanged: true,
			wantOutbound: map[string]any{
				"protocol": "dns", "tag": "dns-out",
				"settings": map[string]any{
					"rules": []any{
						map[string]any{"action": "hijack", "qType": "1,28"},
						map[string]any{"action": "drop"},
					},
				},
			},
		},
		{
			name:        "skip passes everything else through and keeps a lone qType a number",
			raw:         `{"outbounds":[{"protocol":"dns","tag":"dns-out","settings":{"nonIPQuery":"skip","blockTypes":[28]}}]}`,
			wantChanged: true,
			wantOutbound: map[string]any{
				"protocol": "dns", "tag": "dns-out",
				"settings": map[string]any{
					"rules": []any{
						map[string]any{"action": "drop", "qType": float64(28)},
						map[string]any{"action": "hijack", "qType": "1,28"},
						map[string]any{"action": "direct"},
					},
				},
			},
		},
		{
			name:        "the num field could hold a bare number or a string",
			raw:         `{"outbounds":[{"protocol":"dns","tag":"dns-out","settings":{"nonIPQuery":"reject","blockTypes":"65, 28"}}]}`,
			wantChanged: true,
			wantOutbound: map[string]any{
				"protocol": "dns", "tag": "dns-out",
				"settings": map[string]any{
					"rules": []any{
						map[string]any{"action": "return", "qType": "65,28", "rCode": float64(5)},
						map[string]any{"action": "hijack", "qType": "1,28"},
						map[string]any{"action": "return", "rCode": float64(5)},
					},
				},
			},
		},
		{
			name:        "a missing mode answered as reject, the core's default",
			raw:         `{"outbounds":[{"protocol":"dns","tag":"dns-out","settings":{"blockTypes":[28]}}]}`,
			wantChanged: true,
			wantOutbound: map[string]any{
				"protocol": "dns", "tag": "dns-out",
				"settings": map[string]any{
					"rules": []any{
						map[string]any{"action": "return", "qType": float64(28), "rCode": float64(5)},
						map[string]any{"action": "hijack", "qType": "1,28"},
						map[string]any{"action": "return", "rCode": float64(5)},
					},
				},
			},
		},
		{
			name:        "existing rules win, because the core refuses the mix",
			raw:         `{"outbounds":[{"protocol":"dns","tag":"dns-out","settings":{"nonIPQuery":"drop","blockTypes":[28],"rules":[{"action":"hijack","qType":1}]}}]}`,
			wantChanged: true,
			wantOutbound: map[string]any{
				"protocol": "dns", "tag": "dns-out",
				"settings": map[string]any{
					"rules": []any{map[string]any{"action": "hijack", "qType": float64(1)}},
				},
			},
		},
		{
			name:        "a null legacy pair is not a legacy config, as in the core",
			raw:         `{"outbounds":[{"protocol":"dns","tag":"dns-out","settings":{"nonIPQuery":null,"blockTypes":null}}]}`,
			wantChanged: false,
			wantOutbound: map[string]any{
				"protocol": "dns", "tag": "dns-out",
				"settings": map[string]any{"nonIPQuery": nil, "blockTypes": nil},
			},
		},
		{
			name:        "null rules leave the legacy pair authoritative",
			raw:         `{"outbounds":[{"protocol":"dns","tag":"dns-out","settings":{"nonIPQuery":"drop","blockTypes":[28],"rules":null}}]}`,
			wantChanged: true,
			wantOutbound: map[string]any{
				"protocol": "dns", "tag": "dns-out",
				"settings": map[string]any{
					"rules": []any{
						map[string]any{"action": "drop", "qType": float64(28)},
						map[string]any{"action": "hijack", "qType": "1,28"},
						map[string]any{"action": "drop"},
					},
				},
			},
		},
		{
			name:        "the core lowercases the protocol id it dispatches on",
			raw:         `{"outbounds":[{"protocol":"DNS","tag":"dns-out","settings":{"nonIPQuery":"drop","blockTypes":[]}}]}`,
			wantChanged: true,
			wantOutbound: map[string]any{
				"protocol": "DNS", "tag": "dns-out",
				"settings": map[string]any{
					"rules": []any{
						map[string]any{"action": "hijack", "qType": "1,28"},
						map[string]any{"action": "drop"},
					},
				},
			},
		},
		{
			name:        "a dns outbound already on rules is left alone",
			raw:         `{"outbounds":[{"protocol":"dns","tag":"dns-out","settings":{"rules":[{"action":"hijack","qType":1}]}}]}`,
			wantChanged: false,
			wantOutbound: map[string]any{
				"protocol": "dns", "tag": "dns-out",
				"settings": map[string]any{
					"rules": []any{map[string]any{"action": "hijack", "qType": float64(1)}},
				},
			},
		},
		{
			name:        "the same key names on another protocol are left alone",
			raw:         `{"outbounds":[{"protocol":"freedom","tag":"direct","settings":{"nonIPQuery":"drop","blockTypes":[28]}}]}`,
			wantChanged: false,
			wantOutbound: map[string]any{
				"protocol": "freedom", "tag": "direct",
				"settings": map[string]any{"nonIPQuery": "drop", "blockTypes": []any{float64(28)}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			updated, changed, err := rewriteDNSOutboundLegacyKeys(tc.raw)
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

type dnsCoreLogCapture struct{ msgs []string }

func (c *dnsCoreLogCapture) Handle(msg corelog.Message) { c.msgs = append(c.msgs, msg.String()) }

func (c *dnsCoreLogCapture) has(sub string) bool {
	return strings.Contains(strings.Join(c.msgs, "\n"), sub)
}

type dnsDiscardLogHandler struct{}

func (dnsDiscardLogHandler) Handle(corelog.Message) {}

func captureDNSCoreLogs(t *testing.T) *dnsCoreLogCapture {
	t.Helper()
	capture := new(dnsCoreLogCapture)
	corelog.RegisterHandler(capture)
	t.Cleanup(func() { corelog.RegisterHandler(dnsDiscardLogHandler{}) })
	return capture
}

// Drives the real core: the legacy keys warn on every load, rules next to them
// are refused outright, and it reads JSON null the way this rewrite has to.
func TestRewriteDNSOutboundLegacyKeysSatisfiesCore(t *testing.T) {
	for _, tc := range []struct {
		name              string
		raw               string
		wantLoadError     bool
		wantLegacyWarning bool
		wantChanged       bool
	}{
		{
			name:              "deprecated keys",
			raw:               `{"protocol":"dns","tag":"dns-out","settings":{"nonIPQuery":"reject","blockTypes":[65,28]}}`,
			wantLegacyWarning: true,
			wantChanged:       true,
		},
		{
			name:          "deprecated keys next to rules",
			raw:           `{"protocol":"dns","tag":"dns-out","settings":{"nonIPQuery":"drop","blockTypes":[28],"rules":[{"action":"hijack","qType":1}]}}`,
			wantLoadError: true,
			wantChanged:   true,
		},
		{
			name: "a null legacy pair warns about nothing and builds no policy",
			raw:  `{"protocol":"dns","tag":"dns-out","settings":{"nonIPQuery":null,"blockTypes":null}}`,
		},
		{
			name:              "null rules keep the legacy pair in charge, and it warns",
			raw:               `{"protocol":"dns","tag":"dns-out","settings":{"nonIPQuery":"drop","blockTypes":[28],"rules":null}}`,
			wantLegacyWarning: true,
			wantChanged:       true,
		},
		{
			name:              "an upper-case protocol id is a dns outbound to the core",
			raw:               `{"protocol":"DNS","tag":"dns-out","settings":{"nonIPQuery":"drop","blockTypes":[]}}`,
			wantLegacyWarning: true,
			wantChanged:       true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			capture := captureDNSCoreLogs(t)

			if err := xray.ValidateOutboundConfig([]byte(tc.raw)); (err != nil) != tc.wantLoadError {
				t.Fatalf("legacy outbound load error = %v, want error: %v", err, tc.wantLoadError)
			}
			if capture.has("nonIPQuery") != tc.wantLegacyWarning {
				t.Fatalf("legacy warning = %v, want %v: %v", capture.has("nonIPQuery"), tc.wantLegacyWarning, capture.msgs)
			}

			updated, changed, err := rewriteDNSOutboundLegacyKeys(`{"outbounds":[` + tc.raw + `]}`)
			if err != nil || changed != tc.wantChanged {
				t.Fatalf("rewrite: changed=%v want %v err=%v", changed, tc.wantChanged, err)
			}
			if !changed {
				if !strings.Contains(updated, tc.raw) {
					t.Fatalf("unchanged outbound was rewritten: %s", updated)
				}
				return
			}
			var after struct {
				Outbounds []json.RawMessage `json:"outbounds"`
			}
			if err := json.Unmarshal([]byte(updated), &after); err != nil {
				t.Fatal(err)
			}

			capture.msgs = nil
			if err := xray.ValidateOutboundConfig(after.Outbounds[0]); err != nil {
				t.Fatalf("xray-core refused the rewritten outbound: %v", err)
			}
			if capture.has("nonIPQuery") {
				t.Fatalf("rewritten outbound still warns on load: %v", capture.msgs)
			}
		})
	}
}

func TestRewriteDNSOutboundLegacyKeysInvalidJSON(t *testing.T) {
	_, changed, err := rewriteDNSOutboundLegacyKeys("{not json")
	if err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
	if changed {
		t.Fatal("invalid JSON must not report a change")
	}
}

func TestMigrateDNSOutboundLegacyKeysRewritesStoredTemplate(t *testing.T) {
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

	legacy := `{"outbounds":[{"protocol":"dns","tag":"dns-out","settings":{"nonIPQuery":"drop","blockTypes":[28]}}]}`
	seedDNSOutboundTemplate(t, legacy)
	if err := db.Where("seeder_name = ?", "DNSOutboundLegacyKeysFix").
		Delete(&model.HistoryOfSeeders{}).Error; err != nil {
		t.Fatalf("clear seeder history: %v", err)
	}

	if err := migrateDNSOutboundLegacyKeys(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	got := storedDNSOutboundTemplate(t)
	var cfg struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(got), &cfg); err != nil {
		t.Fatalf("stored template is not JSON: %v", err)
	}
	if len(cfg.Outbounds) != 1 {
		t.Fatalf("stored outbounds = %d, want 1", len(cfg.Outbounds))
	}
	settings, _ := cfg.Outbounds[0]["settings"].(map[string]any)
	if _, present := settings["nonIPQuery"]; present {
		t.Errorf("stored outbound kept nonIPQuery: %s", got)
	}
	if _, present := settings["blockTypes"]; present {
		t.Errorf("stored outbound kept blockTypes: %s", got)
	}
	rules, _ := settings["rules"].([]any)
	if len(rules) != 3 {
		t.Fatalf("stored rules = %s, want the three legacy rules in %s", settings["rules"], got)
	}

	// The history gate is what keeps a hand-edited template from being rewritten
	// again on every restart, so run the real seeder list over a fresh legacy one.
	seedDNSOutboundTemplate(t, legacy)
	if err := runSeeders(false); err != nil {
		t.Fatalf("runSeeders: %v", err)
	}
	if got := storedDNSOutboundTemplate(t); got != legacy {
		t.Errorf("a completed seeder rewrote the template again: %s", got)
	}
}

func seedDNSOutboundTemplate(t *testing.T, value string) {
	t.Helper()
	if err := db.Where("key = ?", "xrayTemplateConfig").Delete(&model.Setting{}).Error; err != nil {
		t.Fatalf("clear template: %v", err)
	}
	if err := db.Create(&model.Setting{Key: "xrayTemplateConfig", Value: value}).Error; err != nil {
		t.Fatalf("seed template: %v", err)
	}
}

func storedDNSOutboundTemplate(t *testing.T) string {
	t.Helper()
	var setting model.Setting
	if err := db.Where("key = ?", "xrayTemplateConfig").First(&setting).Error; err != nil {
		t.Fatalf("reload template: %v", err)
	}
	return setting.Value
}
