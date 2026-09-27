package database

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/xtls/xray-core/infra/conf"
	"github.com/xtls/xray-core/proxy/dns"
	"google.golang.org/protobuf/proto"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// The core drops a lone numeric qType 0 from its PortList, and a rule with no
// qTypes matches every query, so blockTypes [0] must not be written that way.
func TestRewriteDNSOutboundLegacyKeysKeepsQTypeZeroPolicy(t *testing.T) {
	for _, mode := range []string{"skip", "reject"} {
		t.Run(mode, func(t *testing.T) {
			legacy := `{"protocol":"dns","tag":"dns-out","settings":{"nonIPQuery":"` + mode + `","blockTypes":[0]}}`
			updated, changed, err := rewriteDNSOutboundLegacyKeys(`{"outbounds":[` + legacy + `]}`)
			if err != nil || !changed {
				t.Fatalf("rewrite: changed=%v err=%v", changed, err)
			}
			got := coreDNSOutboundPolicy(t, firstTemplateOutbound(t, updated))
			if want := coreDNSOutboundPolicy(t, []byte(legacy)); !proto.Equal(got, want) {
				t.Fatalf("rewritten policy = %v, want the legacy policy %v", got, want)
			}
		})
	}
}

// Installs that already ran the legacy-keys seeder store the match-all rule, and
// that seeder never runs again, so the repair has to reach them on its own.
func TestSeedersRepairStoredDNSQTypeZero(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	if err := InitDB(config.GetDBPath()); err != nil {
		if strings.Contains(err.Error(), "CGO_ENABLED=0") {
			t.Skipf("sqlite needs cgo: %v", err)
		}
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = CloseDB() })

	// The legacy-keys seeder matched the protocol id without case, so it wrote both.
	for _, protocol := range []string{"dns", "DNS"} {
		t.Run(protocol, func(t *testing.T) {
			legacy := `{"protocol":"` + protocol + `","tag":"dns-out","settings":{"nonIPQuery":"drop","blockTypes":[0]}}`
			stored := `{"protocol":"` + protocol + `","tag":"dns-out","settings":{"rules":[{"action":"drop","qType":0},{"action":"hijack","qType":"1,28"},{"action":"drop"}]}}`
			seedDNSOutboundTemplate(t, `{"outbounds":[`+stored+`]}`)
			if err := db.Where("seeder_name = ?", "DNSOutboundQTypeZeroFix").
				Delete(&model.HistoryOfSeeders{}).Error; err != nil {
				t.Fatalf("clear seeder history: %v", err)
			}

			if err := runSeeders(false); err != nil {
				t.Fatalf("runSeeders: %v", err)
			}

			got := coreDNSOutboundPolicy(t, firstTemplateOutbound(t, storedDNSOutboundTemplate(t)))
			if want := coreDNSOutboundPolicy(t, []byte(legacy)); !proto.Equal(got, want) {
				t.Fatalf("repaired policy = %v, want the legacy policy %v", got, want)
			}
		})
	}
}

func firstTemplateOutbound(t *testing.T, template string) []byte {
	t.Helper()
	var cfg struct {
		Outbounds []json.RawMessage `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(template), &cfg); err != nil || len(cfg.Outbounds) == 0 {
		t.Fatalf("template has no outbound (%v): %s", err, template)
	}
	return cfg.Outbounds[0]
}

func coreDNSOutboundPolicy(t *testing.T, raw []byte) *dns.Config {
	t.Helper()
	var outbound conf.OutboundDetourConfig
	if err := json.Unmarshal(raw, &outbound); err != nil {
		t.Fatalf("unmarshal outbound: %v", err)
	}
	handler, err := outbound.Build()
	if err != nil {
		t.Fatalf("core build: %v", err)
	}
	instance, err := handler.ProxySettings.GetInstance()
	if err != nil {
		t.Fatalf("core settings: %v", err)
	}
	cfg, ok := instance.(*dns.Config)
	if !ok {
		t.Fatalf("core settings type = %T, want *dns.Config", instance)
	}
	return cfg
}
