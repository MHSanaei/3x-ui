package service

import (
	"encoding/json"
	"testing"
)

func rulesFromRaw(t *testing.T, raw string) []map[string]any {
	t.Helper()
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return routingRulesFromCfg(cfg)
}

func TestEnsureDnsServerRouting_NoOpWithoutDnsServers(t *testing.T) {
	in := `{"routing":{"rules":[{"type":"field","outboundTag":"blocked","ip":["geoip:private"]}]}}`
	out, err := EnsureDnsServerRouting(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != in {
		t.Fatalf("expected unchanged input, got: %s", out)
	}
}

func TestEnsureDnsServerRouting_NoOpForPublicDnsServer(t *testing.T) {
	in := `{
		"dns": {"servers": ["1.1.1.1"]},
		"routing": {"rules": [{"type":"field","outboundTag":"blocked","ip":["geoip:private"]}]}
	}`
	out, err := EnsureDnsServerRouting(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != in {
		t.Fatalf("expected unchanged input for a public DNS server, got: %s", out)
	}
}

func TestEnsureDnsServerRouting_NoOpWithoutPrivateBlockRule(t *testing.T) {
	// Private DNS server, but nothing in routing would block it — no rule
	// needed.
	in := `{
		"dns": {"servers": ["172.20.0.53"]},
		"routing": {"rules": [{"type":"field","inboundTag":["api"],"outboundTag":"api"}]}
	}`
	out, err := EnsureDnsServerRouting(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != in {
		t.Fatalf("expected unchanged input without a private-block rule, got: %s", out)
	}
}

func TestEnsureDnsServerRouting_InsertsAllowRuleBeforeBlock(t *testing.T) {
	// Reproduces the reported bug: dns.servers on a private docker IP
	// (e.g. a same-network AdGuard Home) plus the panel's default
	// geoip:private block rule silently drops Xray's own DNS traffic.
	in := `{
		"dns": {"servers": ["172.20.0.53"], "queryStrategy": "UseIPv4"},
		"routing": {
			"rules": [
				{"type":"field","inboundTag":["api"],"outboundTag":"api"},
				{"type":"field","outboundTag":"blocked","ip":["geoip:private"]},
				{"type":"field","outboundTag":"blocked","protocol":["bittorrent"]}
			]
		}
	}`
	out, err := EnsureDnsServerRouting(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	rules := rulesFromRaw(t, out)
	if len(rules) != 4 {
		t.Fatalf("rules len = %d, want 4: %s", len(rules), out)
	}
	if tag, _ := rules[1]["outboundTag"].(string); tag != "direct" {
		t.Fatalf("expected inserted allow-rule at index 1, got outboundTag %v\nfull: %s", rules[1]["outboundTag"], out)
	}
	ips := readRuleIPs(rules[1]["ip"])
	if len(ips) != 1 || ips[0] != "172.20.0.53" {
		t.Fatalf("allow-rule ip = %v, want [172.20.0.53]", ips)
	}
	if tag, _ := rules[2]["outboundTag"].(string); tag != "blocked" {
		t.Fatalf("private-block rule should still follow the allow-rule, got %v", rules[2])
	}
}

func TestEnsureDnsServerRouting_HandlesObjectServerEntry(t *testing.T) {
	in := `{
		"dns": {"servers": [{"address": "172.20.0.53", "port": 53}]},
		"routing": {"rules": [{"type":"field","outboundTag":"blocked","ip":["geoip:private"]}]}
	}`
	out, err := EnsureDnsServerRouting(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	rules := rulesFromRaw(t, out)
	ips := readRuleIPs(rules[0]["ip"])
	if len(ips) != 1 || ips[0] != "172.20.0.53" {
		t.Fatalf("allow-rule ip = %v, want [172.20.0.53]", ips)
	}
}

func TestEnsureDnsServerRouting_StripsSchemePortAndPath(t *testing.T) {
	in := `{
		"dns": {"servers": [
			"tcp://10.0.0.53:5353",
			"https+local://192.168.1.1/dns-query",
			"[fd00::53]:53"
		]},
		"routing": {"rules": [{"type":"field","outboundTag":"blocked","ip":["geoip:private"]}]}
	}`
	out, err := EnsureDnsServerRouting(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	rules := rulesFromRaw(t, out)
	ips := readRuleIPs(rules[0]["ip"])
	want := map[string]bool{"10.0.0.53": true, "192.168.1.1": true, "fd00::53": true}
	if len(ips) != len(want) {
		t.Fatalf("allow-rule ip = %v, want 3 entries matching %v", ips, want)
	}
	for _, ip := range ips {
		if !want[ip] {
			t.Fatalf("unexpected ip %q in allow-rule %v", ip, ips)
		}
	}
}

func TestEnsureDnsServerRouting_SkipsSpecialAndDomainAddresses(t *testing.T) {
	in := `{
		"dns": {"servers": ["localhost", "fakedns", "dns.google", "8.8.8.8"]},
		"routing": {"rules": [{"type":"field","outboundTag":"blocked","ip":["geoip:private"]}]}
	}`
	out, err := EnsureDnsServerRouting(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != in {
		t.Fatalf("expected unchanged input (no private literal IPs present), got: %s", out)
	}
}

func TestEnsureDnsServerRouting_IdempotentOnSecondSave(t *testing.T) {
	in := `{
		"dns": {"servers": ["172.20.0.53"]},
		"routing": {"rules": [{"type":"field","outboundTag":"blocked","ip":["geoip:private"]}]}
	}`
	first, err := EnsureDnsServerRouting(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	second, err := EnsureDnsServerRouting(first)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if second != first {
		t.Fatalf("expected no further change on second pass\nfirst:  %s\nsecond: %s", first, second)
	}
}

func TestEnsureDnsServerRouting_UpdatesOwnedRuleWhenServersChange(t *testing.T) {
	in := `{
		"dns": {"servers": ["172.20.0.53"]},
		"routing": {
			"rules": [
				{"type":"field","ip":["172.20.0.53"],"outboundTag":"direct"},
				{"type":"field","outboundTag":"blocked","ip":["geoip:private"]}
			]
		}
	}`
	out, err := EnsureDnsServerRouting(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != in {
		t.Fatalf("dns servers unchanged, expected no-op, got: %s", out)
	}

	// Admin adds a second internal resolver.
	in2 := `{
		"dns": {"servers": ["172.20.0.53", "10.0.0.53"]},
		"routing": {
			"rules": [
				{"type":"field","ip":["172.20.0.53"],"outboundTag":"direct"},
				{"type":"field","outboundTag":"blocked","ip":["geoip:private"]}
			]
		}
	}`
	out2, err := EnsureDnsServerRouting(in2)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	rules := rulesFromRaw(t, out2)
	if len(rules) != 2 {
		t.Fatalf("rules len = %d, want 2 (existing rule updated in place): %s", len(rules), out2)
	}
	ips := readRuleIPs(rules[0]["ip"])
	if len(ips) != 2 || ips[0] != "10.0.0.53" || ips[1] != "172.20.0.53" {
		t.Fatalf("allow-rule ip = %v, want [10.0.0.53 172.20.0.53]", ips)
	}
}

func TestEnsureDnsServerRouting_RemovesOwnedRuleWhenNoLongerNeeded(t *testing.T) {
	// Admin switches dns.servers to a public resolver — our previously
	// inserted allow-rule is now dead weight and should be dropped.
	in := `{
		"dns": {"servers": ["1.1.1.1"]},
		"routing": {
			"rules": [
				{"type":"field","ip":["172.20.0.53"],"outboundTag":"direct"},
				{"type":"field","outboundTag":"blocked","ip":["geoip:private"]}
			]
		}
	}`
	out, err := EnsureDnsServerRouting(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	rules := rulesFromRaw(t, out)
	if len(rules) != 1 {
		t.Fatalf("rules len = %d, want 1 (stale allow-rule removed): %s", len(rules), out)
	}
	if tag, _ := rules[0]["outboundTag"].(string); tag != "blocked" {
		t.Fatalf("remaining rule should be the block rule, got %v", rules[0])
	}
}

func TestEnsureDnsServerRouting_DoesNotTouchManualRuleWithExtraMatchers(t *testing.T) {
	// A hand-written rule that also allows the DNS IP but carries an extra
	// matcher isn't recognized as "ours" and must be left alone; a fresh
	// managed rule is inserted alongside it instead.
	in := `{
		"dns": {"servers": ["172.20.0.53"]},
		"routing": {
			"rules": [
				{"type":"field","ip":["172.20.0.53"],"domain":["example.com"],"outboundTag":"direct"},
				{"type":"field","outboundTag":"blocked","ip":["geoip:private"]}
			]
		}
	}`
	out, err := EnsureDnsServerRouting(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	rules := rulesFromRaw(t, out)
	if len(rules) != 3 {
		t.Fatalf("rules len = %d, want 3 (manual rule kept, managed rule inserted): %s", len(rules), out)
	}
	if _, ok := rules[0]["domain"]; !ok {
		t.Fatalf("manual rule with domain matcher should be untouched, got %v", rules[0])
	}
}

func TestEnsureDnsServerRouting_InvalidJsonReturnsAsIs(t *testing.T) {
	in := "definitely not json"
	out, err := EnsureDnsServerRouting(in)
	if err == nil {
		t.Fatalf("expected error for invalid json, got none")
	}
	if out != in {
		t.Fatalf("expected raw passthrough on error, got %q", out)
	}
}
