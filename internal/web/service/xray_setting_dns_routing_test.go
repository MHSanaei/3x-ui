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
	if port, _ := rules[1]["port"].(string); port != "53" {
		t.Fatalf("allow-rule port = %v, want \"53\" (scoped to DNS traffic only)", rules[1]["port"])
	}
	if tag, _ := rules[2]["outboundTag"].(string); tag != "blocked" {
		t.Fatalf("private-block rule should still follow the allow-rule, got %v", rules[2])
	}
}

func TestEnsureDnsServerRouting_HandlesObjectServerEntryWithExplicitPort(t *testing.T) {
	in := `{
		"dns": {"servers": [{"address": "172.20.0.53", "port": 5353}]},
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
	if port, _ := rules[0]["port"].(string); port != "5353" {
		t.Fatalf("allow-rule port = %v, want the object's explicit \"5353\"", rules[0]["port"])
	}
}

func TestEnsureDnsServerRouting_GroupsDistinctPortsIntoSeparateRules(t *testing.T) {
	// Two internal resolvers on different ports must not be merged into
	// one ip+port rule — that would cross-allow ip1:port2 and ip2:port1,
	// widening the exception beyond what's actually needed.
	in := `{
		"dns": {"servers": ["172.20.0.53", "10.0.0.53:5353"]},
		"routing": {"rules": [{"type":"field","outboundTag":"blocked","ip":["geoip:private"]}]}
	}`
	out, err := EnsureDnsServerRouting(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	rules := rulesFromRaw(t, out)
	if len(rules) != 3 {
		t.Fatalf("rules len = %d, want 3 (one rule per port + the block rule): %s", len(rules), out)
	}
	if p, _ := rules[0]["port"].(string); p != "53" {
		t.Fatalf("rules[0] port = %v, want \"53\" (sorted ascending)", rules[0]["port"])
	}
	if ips := readRuleIPs(rules[0]["ip"]); len(ips) != 1 || ips[0] != "172.20.0.53" {
		t.Fatalf("rules[0] ip = %v, want [172.20.0.53]", ips)
	}
	if p, _ := rules[1]["port"].(string); p != "5353" {
		t.Fatalf("rules[1] port = %v, want \"5353\"", rules[1]["port"])
	}
	if ips := readRuleIPs(rules[1]["ip"]); len(ips) != 1 || ips[0] != "10.0.0.53" {
		t.Fatalf("rules[1] ip = %v, want [10.0.0.53]", ips)
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
	// 192.168.1.1 and fd00::53 share the default port 53; 10.0.0.53:5353
	// is its own group.
	if len(rules) != 3 {
		t.Fatalf("rules len = %d, want 3 (2 port groups + block rule): %s", len(rules), out)
	}
	port53 := rules[0]
	if p, _ := port53["port"].(string); p != "53" {
		t.Fatalf("rules[0] port = %v, want \"53\"", port53["port"])
	}
	want53 := map[string]bool{"192.168.1.1": true, "fd00::53": true}
	ips := readRuleIPs(port53["ip"])
	if len(ips) != len(want53) {
		t.Fatalf("rules[0] ip = %v, want entries matching %v", ips, want53)
	}
	for _, ip := range ips {
		if !want53[ip] {
			t.Fatalf("unexpected ip %q in port-53 rule %v", ip, ips)
		}
	}
	port5353 := rules[1]
	if p, _ := port5353["port"].(string); p != "5353" {
		t.Fatalf("rules[1] port = %v, want \"5353\"", port5353["port"])
	}
	if ips := readRuleIPs(port5353["ip"]); len(ips) != 1 || ips[0] != "10.0.0.53" {
		t.Fatalf("rules[1] ip = %v, want [10.0.0.53]", ips)
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
				{"type":"field","ip":["172.20.0.53"],"port":"53","outboundTag":"direct"},
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

	// Admin adds a second internal resolver on the same port.
	in2 := `{
		"dns": {"servers": ["172.20.0.53", "10.0.0.53"]},
		"routing": {
			"rules": [
				{"type":"field","ip":["172.20.0.53"],"port":"53","outboundTag":"direct"},
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
				{"type":"field","ip":["172.20.0.53"],"port":"53","outboundTag":"direct"},
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
				{"type":"field","ip":["172.20.0.53"],"port":"53","domain":["example.com"],"outboundTag":"direct"},
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

func TestEnsureDnsServerRouting_RepositionsRuleDraggedAfterBlockRule(t *testing.T) {
	// The Routing tab lets admins freely drag rules around
	// (RoutingTab.tsx move-up/move-down and drag-and-drop), and a managed
	// rule renders as an indistinguishable normal row there. If it ends up
	// at or after the block rule — directly, or incidentally while
	// reordering something else — the exact bug this file fixes comes
	// back silently: the block rule matches first and Xray's own DNS
	// traffic is dropped again. Detecting only ip/content drift isn't
	// enough; position must be checked too.
	in := `{
		"dns": {"servers": ["172.20.0.53"]},
		"routing": {
			"rules": [
				{"type":"field","outboundTag":"blocked","ip":["geoip:private"]},
				{"type":"field","ip":["172.20.0.53"],"port":"53","outboundTag":"direct"}
			]
		}
	}`
	out, err := EnsureDnsServerRouting(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	rules := rulesFromRaw(t, out)
	if len(rules) != 2 {
		t.Fatalf("rules len = %d, want 2: %s", len(rules), out)
	}
	if tag, _ := rules[0]["outboundTag"].(string); tag != "direct" {
		t.Fatalf("managed rule should be re-homed to index 0 (before the block rule), got %v\nfull: %s", rules[0], out)
	}
	if tag, _ := rules[1]["outboundTag"].(string); tag != "blocked" {
		t.Fatalf("block rule should now be at index 1, got %v\nfull: %s", rules[1], out)
	}
}

func TestEnsureDnsServerRouting_TreatsExplicitlyEnabledRuleAsOwned(t *testing.T) {
	// RuleFormModal.tsx's submit() always writes an "enabled" key, even
	// when the admin changed nothing and it was already true — merely
	// opening the auto-generated rule in the editor must not disown it.
	in := `{
		"dns": {"servers": ["172.20.0.53"]},
		"routing": {
			"rules": [
				{"type":"field","ip":["172.20.0.53"],"port":"53","outboundTag":"direct","enabled":true},
				{"type":"field","outboundTag":"blocked","ip":["geoip:private"]}
			]
		}
	}`
	out, err := EnsureDnsServerRouting(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	rules := rulesFromRaw(t, out)
	if len(rules) != 2 {
		t.Fatalf("rules len = %d, want 2 (no duplicate inserted): %s", len(rules), out)
	}
}

func TestEnsureDnsServerRouting_DisabledRuleIsDisownedAndReplaced(t *testing.T) {
	// toggleRule() in RoutingTab.tsx writes enabled=false on a plain
	// switch flip. An admin who explicitly disables the managed rule is
	// choosing to turn the exception off; re-enabling it on the next save
	// would silently override that. The disabled rule is left alone and a
	// fresh, enabled one is (re-)created to keep the fix working.
	in := `{
		"dns": {"servers": ["172.20.0.53"]},
		"routing": {
			"rules": [
				{"type":"field","ip":["172.20.0.53"],"port":"53","outboundTag":"direct","enabled":false},
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
		t.Fatalf("rules len = %d, want 3 (disabled rule kept as-is, fresh managed rule added): %s", len(rules), out)
	}
	if enabled, _ := rules[0]["enabled"].(bool); enabled {
		t.Fatalf("original disabled rule should be untouched, got %v", rules[0])
	}
	if _, ok := rules[1]["enabled"]; ok {
		t.Fatalf("freshly (re-)generated rule shouldn't carry an enabled key, got %v", rules[1])
	}
	if tag, _ := rules[1]["outboundTag"].(string); tag != "direct" {
		t.Fatalf("expected the fresh managed rule at index 1, got %v", rules[1])
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
