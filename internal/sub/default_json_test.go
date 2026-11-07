package sub

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDefaultJSON_UsesEncryptedDNSWithoutHostnameBootstrap(t *testing.T) {
	var cfg map[string]any
	if err := json.Unmarshal([]byte(defaultJson), &cfg); err != nil {
		t.Fatalf("unmarshal embedded default.json: %v", err)
	}
	dns, _ := cfg["dns"].(map[string]any)
	servers, _ := dns["servers"].([]any)
	if len(servers) == 0 {
		t.Fatal("embedded default.json has no DNS server")
	}
	server, _ := servers[0].(map[string]any)
	address, _ := server["address"].(string)
	if address != "https://8.8.8.8/dns-query" {
		t.Fatalf("default DNS address = %q, want encrypted IP endpoint", address)
	}
	if strategy, _ := dns["queryStrategy"].(string); strategy != "UseIP" {
		t.Fatalf("default DNS queryStrategy = %q, want dual-stack UseIP", strategy)
	}
}

// The catch-all proxy rule has no IPv4-only matcher. UDP, including IPv6
// STUN/ICE and DNS traffic that reaches Xray's local SOCKS inbound, therefore
// follows the proxy outbound instead of falling through to direct.
func TestDefaultJSON_ProxiesTCPAndUDPWithoutIPFamilyRestriction(t *testing.T) {
	var cfg map[string]any
	if err := json.Unmarshal([]byte(defaultJson), &cfg); err != nil {
		t.Fatalf("unmarshal embedded default.json: %v", err)
	}
	routing, _ := cfg["routing"].(map[string]any)
	rules, _ := routing["rules"].([]any)
	if len(rules) == 0 {
		t.Fatal("embedded default.json has no routing rules")
	}
	rule, _ := rules[len(rules)-1].(map[string]any)
	if rule["network"] != "tcp,udp" || rule["outboundTag"] != "proxy" {
		t.Fatalf("final routing rule = %v, want family-neutral tcp,udp -> proxy", rule)
	}
	if _, limitedToIPs := rule["ip"]; limitedToIPs {
		t.Fatalf("final proxy rule unexpectedly limits IP families: %v", rule)
	}
}

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

