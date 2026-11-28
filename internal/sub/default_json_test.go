package sub

import (
	"encoding/json"
	"strings"
	"testing"

	corelog "github.com/xtls/xray-core/common/log"
	"github.com/xtls/xray-core/infra/conf"
)

// xray-core moves freedom settings.domainStrategy to sockopt with a warning on
// every load (#6482); the embed omits it and gets the AsIs default.
func TestDefaultJSON_FreedomOutboundHasNoLegacyDomainStrategy(t *testing.T) {
	var cfg map[string]any
	if err := json.Unmarshal([]byte(defaultJson), &cfg); err != nil {
		t.Fatalf("unmarshal embedded default.json: %v", err)
	}
	outbounds, _ := cfg["outbounds"].([]any)
	var sawFreedom bool
	for _, raw := range outbounds {
		ob, _ := raw.(map[string]any)
		proto, _ := ob["protocol"].(string)
		if !strings.EqualFold(proto, "freedom") {
			continue
		}
		sawFreedom = true
		settings, _ := ob["settings"].(map[string]any)
		if _, ok := settings["domainStrategy"]; ok {
			t.Fatalf("freedom outbound %q still has settings.domainStrategy=%v; use sockopt or omit (AsIs default)", ob["tag"], settings["domainStrategy"])
		}
		if _, ok := settings["targetStrategy"]; ok {
			t.Fatalf("freedom outbound %q still has settings.targetStrategy", ob["tag"])
		}
		if _, ok := ob["targetStrategy"]; ok {
			t.Fatalf("freedom outbound %q still has root targetStrategy", ob["tag"])
		}
	}
	if !sawFreedom {
		t.Fatal("embedded default.json has no freedom outbound to check")
	}
}

type coreWarnings struct{ msgs []string }

func (c *coreWarnings) Handle(msg corelog.Message) { c.msgs = append(c.msgs, msg.String()) }

type discardCoreLog struct{}

func (discardCoreLog) Handle(corelog.Message) {}

// A deprecated key in the document is a warning in every subscriber's client on every
// load, and a hard failure once the core drops the key.
func TestJSONSubscriptionLoadsInCoreWithoutDeprecationWarning(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "core-load", "core-link", 4933, 1, tcpTLSStream)
	body, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("core-load", "sub.example.com", false)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	var config conf.Config
	if err := json.Unmarshal([]byte(body), &config); err != nil {
		t.Fatalf("core cannot parse the document: %v\n%s", err, body)
	}
	if len(config.OutboundConfigs) != 3 {
		t.Fatalf("outbounds = %d, want proxy, direct and block\n%s", len(config.OutboundConfigs), body)
	}

	warnings := new(coreWarnings)
	corelog.RegisterHandler(warnings)
	t.Cleanup(func() { corelog.RegisterHandler(discardCoreLog{}) })
	if _, err := config.Build(); err != nil {
		t.Fatalf("core rejected the document: %v\n%s", err, body)
	}
	for _, msg := range warnings.msgs {
		if strings.Contains(msg, "deprecated") {
			t.Errorf("core warning: %s", msg)
		}
	}
}
