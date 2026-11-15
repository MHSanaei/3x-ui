package outbound

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// #7131: the probe instance used to carry no "dns" section at all, so an
// outbound whose address only the panel's configured resolvers could see
// failed the test while working in production. These tests pin the three
// halves of the fix: sanitizing the panel dns for probe use, threading it
// onto the temp config, and leaving configs untouched when there is no dns.

func mustRaw(t *testing.T, v any) json_util.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return json_util.RawMessage(raw)
}

func TestSanitizeProbeDNSKeepsResolutionKeysAndDropsRoutingCoupledOnes(t *testing.T) {
	in := map[string]any{
		"servers":       []any{"https+local://198.51.100.10/dns-query"},
		"hosts":         map[string]any{"node1.example.internal": "198.51.100.20"},
		"clientIp":      "198.51.100.1",
		"queryStrategy": "UseIPv4",
		"tag":           "dns_inbound",
		"script":        "return nil",
		"unknownKey":    "dropped",
	}

	out := SanitizeProbeDNS(mustRaw(t, in))
	if out == nil {
		t.Fatal("expected a sanitized dns section, got nil")
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal sanitized dns: %v", err)
	}

	for _, key := range []string{"servers", "hosts", "clientIp", "queryStrategy"} {
		if got[key] == nil {
			t.Errorf("resolution key %q missing from sanitized dns: %v", key, got)
		}
	}
	for _, key := range []string{"tag", "script", "unknownKey"} {
		if got[key] != nil {
			t.Errorf("routing-coupled/unrelated key %q survived sanitization: %v", key, got)
		}
	}
}

func TestSanitizeProbeDNSNilForEmptyOrUnusableInput(t *testing.T) {
	if out := SanitizeProbeDNS(nil); out != nil {
		t.Errorf("nil input should stay nil, got %s", out)
	}
	if out := SanitizeProbeDNS(json_util.RawMessage(``)); out != nil {
		t.Errorf("empty input should stay nil, got %s", out)
	}
	if out := SanitizeProbeDNS(json_util.RawMessage(`{}`)); out != nil {
		t.Errorf("section with no usable keys should become nil, got %s", out)
	}
	if out := SanitizeProbeDNS(json_util.RawMessage(`not json`)); out != nil {
		t.Errorf("invalid json should become nil, got %s", out)
	}
	// A dns section holding only routing-coupled keys is useless to the probe.
	if out := SanitizeProbeDNS(json_util.RawMessage(`{"tag":"dns_inbound"}`)); out != nil {
		t.Errorf("tag-only section should become nil, got %s", out)
	}
}

func TestWithProbeDNSInstallsSanitizedSectionOnProbeConfig(t *testing.T) {
	items := []*httpBatchItem{
		{tag: "proxy-a", outbound: map[string]any{"tag": "proxy-a", "protocol": "vless"}},
	}
	ports := []int{61001}

	// Without dns the config is exactly what buildBatchTestConfig made.
	plain := withProbeDNS(items, nil, ports, nil)
	if plain.DNSConfig != nil {
		t.Errorf("nil dns must leave the probe config without dns, got %s", plain.DNSConfig)
	}

	dns := json_util.RawMessage(`{"servers":["localhost"],"tag":"dns_inbound"}`)
	cfg := withProbeDNS(items, nil, ports, dns)
	if cfg.DNSConfig == nil {
		t.Fatal("dns section not installed on probe config")
	}
	if strings.Contains(string(cfg.DNSConfig), "dns_inbound") {
		t.Errorf("routing-coupled tag leaked into probe dns: %s", cfg.DNSConfig)
	}
	if !strings.Contains(string(cfg.DNSConfig), "localhost") {
		t.Errorf("servers missing from probe dns: %s", cfg.DNSConfig)
	}
	// The rest of the config is untouched by the dns pass.
	if cfg.InboundConfigs == nil || len(cfg.InboundConfigs) != len(items) {
		t.Errorf("inbounds changed by dns threading: %+v", cfg.InboundConfigs)
	}
	if cfg.OutboundConfigs == nil {
		t.Error("outbounds lost by dns threading")
	}
	// Type sanity: it must still marshal into a valid xray.Config shape.
	if _, err := json.Marshal(cfg); err != nil {
		t.Fatalf("marshal probe config: %v", err)
	}
	_ = xray.Config{}
}

func TestSetProbeDNSInstallsSanitizedSection(t *testing.T) {
	s := &OutboundService{}
	if s.probeDNS != nil {
		t.Fatal("zero-value service must start without probe dns")
	}
	s.SetProbeDNS(json_util.RawMessage(`{"tag":"dns_inbound","hosts":{"a.internal":"198.51.100.20"}}`))
	if s.probeDNS == nil {
		t.Fatal("SetProbeDNS did not install a section")
	}
	if strings.Contains(string(s.probeDNS), "dns_inbound") {
		t.Errorf("tag survived SetProbeDNS: %s", s.probeDNS)
	}
	s.SetProbeDNS(json_util.RawMessage(`not json`))
	if s.probeDNS != nil {
		t.Errorf("invalid dns must reset to nil, got %s", s.probeDNS)
	}
}

// End-to-end over the public API: a service with the panel dns installed must
// hand the temp instance a config whose dns section is the sanitized one —
// and a zero-value service must keep handing it none, the pre-fix behaviour
// for panels without a dns section (#7131).
func TestTestOutboundsThreadsPanelDNSIntoTempInstance(t *testing.T) {
	for _, tc := range []struct {
		name     string
		panelDNS string
		wantDNS  bool
	}{
		{name: "panel dns reaches the probe config", panelDNS: `{"servers":["https+local://198.51.100.10/dns-query"],"hosts":{"node1.example.internal":"198.51.100.20"},"tag":"dns_inbound"}`, wantDNS: true},
		{name: "no panel dns keeps the old behaviour", panelDNS: "", wantDNS: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got *xray.Config
			calls := 0
			withStubProcess(t, func(cfg *xray.Config, configPath string) batchProcess {
				calls++
				got = cfg
				return &stubProcess{cfg: cfg, startErr: errors.New("stop here")}
			})

			s := &OutboundService{}
			if tc.panelDNS != "" {
				s.SetProbeDNS(json_util.RawMessage(tc.panelDNS))
			}
			// A start error fails the batch fast without ever dialing; the
			// config is still built, which is all this test inspects.
			_, _ = s.TestOutbounds(mustJSON(t, []any{map[string]any{"tag": "a", "protocol": "vless"}}), "http://example.invalid/gen", "", "http")
			if calls == 0 {
				t.Fatal("temp instance never spawned")
			}
			if tc.wantDNS {
				if got.DNSConfig == nil {
					t.Fatal("panel dns did not reach the temp instance config")
				}
				var dns map[string]any
				if err := json.Unmarshal(got.DNSConfig, &dns); err != nil {
					t.Fatalf("probe dns not valid json: %v", err)
				}
				if dns["servers"] == nil || dns["hosts"] == nil {
					t.Errorf("resolution keys missing from probe dns: %v", dns)
				}
				if dns["tag"] != nil {
					t.Errorf("routing-coupled tag reached the probe config: %v", dns["tag"])
				}
			} else if got.DNSConfig != nil {
				t.Errorf("probe config unexpectedly carries dns: %s", got.DNSConfig)
			}
		})
	}
}
