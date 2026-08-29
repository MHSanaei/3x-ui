package service

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"github.com/op/go-logging"
	"github.com/xtls/xray-core/infra/conf"
)

func TestMain(m *testing.M) {
	// A test binary re-executed with MTG_FAKE_CHILD=1 poses as an mtg child
	// process (see mtproto_fake_test.go) and never reaches the test runner.
	if os.Getenv("MTG_FAKE_CHILD") == "1" {
		fakeMtgChildMain()
	}
	// injectPanelEgress logs when it skips injection; the package logger must
	// exist before any test exercises a skipped path.
	xuilogger.InitLogger(logging.ERROR)
	os.Exit(m.Run())
}

// stockOutbounds mirrors the shipped template: a freedom outbound tagged
// "direct" resolving AsIs, which is what leaves the dns section inert until
// the injector switches it.
const stockOutbounds = `[{"protocol":"freedom","settings":{"domainStrategy":"AsIs"},"tag":"direct"},{"protocol":"blackhole","settings":{},"tag":"blocked"}]`

// adGuardDNSResult pulls out the two things the injector is responsible for:
// the servers it points the core at, and the direct outbound's strategy.
func adGuardDNSResult(t *testing.T, cfg *xray.Config) ([]any, string) {
	t.Helper()
	var dns struct {
		Servers []any `json:"servers"`
	}
	if len(cfg.DNSConfig) > 0 {
		if err := json.Unmarshal(cfg.DNSConfig, &dns); err != nil {
			t.Fatalf("dns section is not valid JSON: %v\n%s", err, cfg.DNSConfig)
		}
	}
	var outbounds []struct {
		Tag      string `json:"tag"`
		Settings struct {
			DomainStrategy string `json:"domainStrategy"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(cfg.OutboundConfigs, &outbounds); err != nil {
		t.Fatalf("outbounds are not valid JSON: %v", err)
	}
	strategy := ""
	for _, outbound := range outbounds {
		if outbound.Tag == adGuardDirectOutboundTag {
			strategy = outbound.Settings.DomainStrategy
		}
	}
	return dns.Servers, strategy
}

func TestInjectAdGuardDNS(t *testing.T) {
	t.Run("points the core at AdGuard Home and switches the direct outbound", func(t *testing.T) {
		cfg := &xray.Config{OutboundConfigs: json_util.RawMessage(stockOutbounds)}
		injectAdGuardDNS(cfg, 5335)

		servers, strategy := adGuardDNSResult(t, cfg)
		if len(servers) != 2 {
			t.Fatalf("servers = %v, want AdGuard Home plus a fallback", servers)
		}
		// Address and port must stay separate fields: a server written as one
		// "host:port" string is parsed as a URL and refused outright.
		first, ok := servers[0].(map[string]any)
		if !ok {
			t.Fatalf("servers[0] = %#v, want an address/port object", servers[0])
		}
		if first["address"] != "127.0.0.1" || first["port"] != float64(5335) {
			t.Errorf("servers[0] = %#v, want address 127.0.0.1 port 5335", first)
		}
		// A stopped AdGuard Home must not take every outbound down with it.
		if servers[len(servers)-1] != adGuardDNSFallback {
			t.Errorf("servers = %v, want %s kept as the fallback", servers, adGuardDNSFallback)
		}
		// Without this half the dns section would never be consulted at all.
		if strategy != "UseIP" {
			t.Errorf("direct outbound domainStrategy = %q, want UseIP", strategy)
		}
	})

	// The section this builds is only ever validated by the core itself, at
	// startup, on the admin's server. Getting it wrong there is not a bad
	// setting -- the core refuses to start and takes every inbound with it,
	// which is exactly what a "host:port" server string did. Checking it here
	// against the core's own parser is the only place that failure is cheap.
	t.Run("builds a dns section the core itself accepts", func(t *testing.T) {
		cfg := &xray.Config{OutboundConfigs: json_util.RawMessage(stockOutbounds)}
		injectAdGuardDNS(cfg, 5335)

		var parsed conf.DNSConfig
		if err := json.Unmarshal(cfg.DNSConfig, &parsed); err != nil {
			t.Fatalf("xray-core cannot parse the dns section: %v\n%s", err, cfg.DNSConfig)
		}
		if _, err := parsed.Build(); err != nil {
			t.Fatalf("xray-core rejects the dns section: %v\n%s", err, cfg.DNSConfig)
		}
	})

	t.Run("leaves a hand-written dns section alone", func(t *testing.T) {
		own := `{"servers":["8.8.8.8"]}`
		cfg := &xray.Config{
			DNSConfig:       json_util.RawMessage(own),
			OutboundConfigs: json_util.RawMessage(stockOutbounds),
		}
		injectAdGuardDNS(cfg, 5335)

		if string(cfg.DNSConfig) != own {
			t.Errorf("dns section was rewritten to %s", cfg.DNSConfig)
		}
		// The outbound must not move either, or the admin's own resolver would
		// silently start being used for every dial.
		if _, strategy := adGuardDNSResult(t, cfg); strategy != "AsIs" {
			t.Errorf("direct outbound domainStrategy = %q, want it left at AsIs", strategy)
		}
	})

	t.Run("refuses rather than half-apply when there is no direct outbound", func(t *testing.T) {
		cfg := &xray.Config{OutboundConfigs: json_util.RawMessage(`[{"protocol":"blackhole","settings":{},"tag":"blocked"}]`)}
		injectAdGuardDNS(cfg, 5335)

		// A dns section without the UseIP half is inert, and an inert setting
		// that reports itself as on is worse than an honest refusal.
		if len(cfg.DNSConfig) != 0 {
			t.Errorf("dns section was written anyway: %s", cfg.DNSConfig)
		}
	})

	t.Run("refuses an unusable port", func(t *testing.T) {
		for _, port := range []int{0, -1, 70000} {
			cfg := &xray.Config{OutboundConfigs: json_util.RawMessage(stockOutbounds)}
			injectAdGuardDNS(cfg, port)
			if len(cfg.DNSConfig) != 0 {
				t.Errorf("port %d: dns section written anyway: %s", port, cfg.DNSConfig)
			}
			if _, strategy := adGuardDNSResult(t, cfg); strategy != "AsIs" {
				t.Errorf("port %d: direct outbound changed to %q", port, strategy)
			}
		}
	})
}

func TestEnsureAPIServices(t *testing.T) {
	// legacy template without RoutingService gets it injected
	out := ensureAPIServices(json_util.RawMessage(`{"services":["HandlerService","LoggerService","StatsService"],"tag":"api"}`))
	var parsed struct {
		Services []string `json:"services"`
		Tag      string   `json:"tag"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"HandlerService": true, "StatsService": true, "RoutingService": true, "LoggerService": true}
	if len(parsed.Services) != 4 {
		t.Fatalf("expected 4 services, got %v", parsed.Services)
	}
	for _, svc := range parsed.Services {
		if !want[svc] {
			t.Fatalf("unexpected service %q", svc)
		}
	}
	if parsed.Tag != "api" {
		t.Fatalf("tag must be preserved, got %q", parsed.Tag)
	}

	// complete api block is returned unchanged (no marshal churn)
	full := json_util.RawMessage(`{"services":["HandlerService","StatsService","RoutingService"],"tag":"api"}`)
	if got := ensureAPIServices(full); string(got) != string(full) {
		t.Fatalf("complete api block must pass through untouched, got %s", got)
	}

	// absent api block stays absent
	if got := ensureAPIServices(nil); got != nil {
		t.Fatalf("nil api block must stay nil, got %s", got)
	}
}

func TestEnsureStatsPolicy(t *testing.T) {
	// default-template shape: level "0" exists with traffic flags — the online
	// flag is added and the siblings survive untouched
	out := ensureStatsPolicy(json_util.RawMessage(`{"levels":{"0":{"handshake":4,"statsUserUplink":true,"statsUserDownlink":true}},"system":{"statsInboundDownlink":true}}`))
	var parsed struct {
		Levels map[string]map[string]any `json:"levels"`
		System map[string]any            `json:"system"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}
	level0 := parsed.Levels["0"]
	if level0["statsUserOnline"] != true {
		t.Fatalf("statsUserOnline must be injected into level 0, got %v", level0)
	}
	if level0["statsUserUplink"] != true || level0["statsUserDownlink"] != true || level0["handshake"] != float64(4) {
		t.Fatalf("sibling keys must be preserved, got %v", level0)
	}
	if parsed.System["statsInboundDownlink"] != true {
		t.Fatalf("system block must be preserved, got %v", parsed.System)
	}

	// missing levels block: level "0" is created with the flag
	out = ensureStatsPolicy(json_util.RawMessage(`{"system":{}}`))
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Levels["0"]["statsUserOnline"] != true {
		t.Fatalf("level 0 must be created with statsUserOnline, got %s", out)
	}

	// every level gets the flag, an explicit false included — the flag is
	// panel infrastructure, like the api services
	out = ensureStatsPolicy(json_util.RawMessage(`{"levels":{"0":{"statsUserOnline":false},"1":{"connIdle":300}}}`))
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"0", "1"} {
		if parsed.Levels[key]["statsUserOnline"] != true {
			t.Fatalf("level %s must have statsUserOnline forced on, got %s", key, out)
		}
	}
	if parsed.Levels["1"]["connIdle"] != float64(300) {
		t.Fatalf("level 1 siblings must be preserved, got %s", out)
	}

	// already-enabled input passes through byte-identical (no marshal churn,
	// no spurious restart)
	full := json_util.RawMessage(`{"levels":{"0":{"statsUserOnline":true}}}`)
	if got := ensureStatsPolicy(full); string(got) != string(full) {
		t.Fatalf("already-enabled policy must pass through untouched, got %s", got)
	}

	// absent policy block stays absent
	if got := ensureStatsPolicy(nil); got != nil {
		t.Fatalf("nil policy must stay nil, got %s", got)
	}

	// unparsable policy is left untouched
	bad := json_util.RawMessage(`{not json`)
	if got := ensureStatsPolicy(bad); string(got) != string(bad) {
		t.Fatalf("unparsable policy must be left untouched, got %s", got)
	}
}

func egressTestConfig() *xray.Config {
	return &xray.Config{
		RouterConfig:    json_util.RawMessage(`{"domainStrategy":"AsIs","rules":[{"type":"field","inboundTag":["api"],"outboundTag":"api"}]}`),
		OutboundConfigs: json_util.RawMessage(`[{"protocol":"freedom","tag":"direct"},{"protocol":"socks","tag":"warp"}]`),
		InboundConfigs: []xray.InboundConfig{
			{Port: 62789, Protocol: "tunnel", Tag: "api", Listen: json_util.RawMessage(`"127.0.0.1"`)},
		},
	}
}

type egressRouting struct {
	DomainStrategy string `json:"domainStrategy"`
	Rules          []struct {
		InboundTag  []string `json:"inboundTag"`
		OutboundTag string   `json:"outboundTag"`
		Type        string   `json:"type"`
	} `json:"rules"`
}

func TestInjectPanelEgress(t *testing.T) {
	cfg := egressTestConfig()
	injectPanelEgress(cfg, "warp")

	if len(cfg.InboundConfigs) != 2 {
		t.Fatalf("expected the egress inbound to be appended, got %d inbounds", len(cfg.InboundConfigs))
	}
	ib := cfg.InboundConfigs[1]
	if ib.Tag != PanelEgressInboundTag || ib.Protocol != "socks" || ib.Port != panelEgressBasePort {
		t.Fatalf("unexpected egress inbound: %+v", ib)
	}
	if string(ib.Listen) != `"127.0.0.1"` {
		t.Fatalf("egress inbound must listen on loopback, got %s", ib.Listen)
	}

	var routing egressRouting
	if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
		t.Fatal(err)
	}
	if routing.DomainStrategy != "AsIs" {
		t.Fatalf("routing keys outside rules must be preserved, got %+v", routing)
	}
	if len(routing.Rules) != 2 {
		t.Fatalf("expected egress rule + existing rule, got %+v", routing.Rules)
	}
	first := routing.Rules[0]
	if first.Type != "field" || first.OutboundTag != "warp" ||
		len(first.InboundTag) != 1 || first.InboundTag[0] != PanelEgressInboundTag {
		t.Fatalf("egress rule must be prepended, got %+v", first)
	}
}

func TestInjectPanelEgress_BalancerTag(t *testing.T) {
	cfg := egressTestConfig()
	cfg.RouterConfig = json_util.RawMessage(`{"domainStrategy":"AsIs","rules":[],"balancers":[{"tag":"lb","selector":["warp"]}]}`)

	// A tag that names a balancer must be targeted via balancerTag so the
	// router resolves it; an outbound tag coexisting with balancers still uses
	// outboundTag.
	injectPanelEgress(cfg, "lb")

	var routing struct {
		Rules []struct {
			InboundTag  []string `json:"inboundTag"`
			OutboundTag string   `json:"outboundTag"`
			BalancerTag string   `json:"balancerTag"`
			Type        string   `json:"type"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
		t.Fatal(err)
	}
	if len(routing.Rules) != 1 {
		t.Fatalf("expected the egress rule, got %+v", routing.Rules)
	}
	first := routing.Rules[0]
	if first.BalancerTag != "lb" || first.OutboundTag != "" {
		t.Fatalf("a balancer tag must target balancerTag, not outboundTag, got %+v", first)
	}
	if len(first.InboundTag) != 1 || first.InboundTag[0] != PanelEgressInboundTag {
		t.Fatalf("egress rule must bind the egress inbound, got %+v", first)
	}

	// A non-balancer tag alongside balancers keeps the plain outbound path.
	cfg2 := egressTestConfig()
	cfg2.RouterConfig = json_util.RawMessage(`{"rules":[],"balancers":[{"tag":"lb","selector":["warp"]}]}`)
	injectPanelEgress(cfg2, "warp")
	var routing2 struct {
		Rules []struct {
			OutboundTag string `json:"outboundTag"`
			BalancerTag string `json:"balancerTag"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(cfg2.RouterConfig, &routing2); err != nil {
		t.Fatal(err)
	}
	if routing2.Rules[0].OutboundTag != "warp" || routing2.Rules[0].BalancerTag != "" {
		t.Fatalf("a concrete outbound must target outboundTag, got %+v", routing2.Rules[0])
	}
}

func TestInjectPanelEgress_PortCollision(t *testing.T) {
	cfg := egressTestConfig()
	cfg.InboundConfigs = append(cfg.InboundConfigs,
		xray.InboundConfig{Port: panelEgressBasePort, Protocol: "vless", Tag: "in-1"},
		xray.InboundConfig{Port: panelEgressBasePort + 1, Protocol: "vless", Tag: "in-2"},
	)
	injectPanelEgress(cfg, "direct")
	got := cfg.InboundConfigs[len(cfg.InboundConfigs)-1]
	if got.Tag != PanelEgressInboundTag || got.Port != panelEgressBasePort+2 {
		t.Fatalf("egress inbound must skip taken ports, got %+v", got)
	}
}

func TestInjectPanelEgress_TagCollisionSkips(t *testing.T) {
	cfg := egressTestConfig()
	cfg.InboundConfigs = append(cfg.InboundConfigs,
		xray.InboundConfig{Port: 1234, Protocol: "socks", Tag: PanelEgressInboundTag},
	)
	before := string(cfg.RouterConfig)
	injectPanelEgress(cfg, "direct")
	if len(cfg.InboundConfigs) != 2 || string(cfg.RouterConfig) != before {
		t.Fatal("a user inbound owning the egress tag must make injection a no-op")
	}
}

func TestInjectPanelEgress_NoRoutingSection(t *testing.T) {
	cfg := egressTestConfig()
	cfg.RouterConfig = nil
	injectPanelEgress(cfg, "direct")

	var routing egressRouting
	if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
		t.Fatal(err)
	}
	if len(routing.Rules) != 1 || routing.Rules[0].OutboundTag != "direct" {
		t.Fatalf("a routing section must be created with the egress rule, got %+v", routing)
	}
	if len(cfg.InboundConfigs) != 2 {
		t.Fatal("egress inbound must still be appended")
	}
}

func TestInjectPanelEgress_BadRoutingSkips(t *testing.T) {
	cfg := egressTestConfig()
	cfg.RouterConfig = json_util.RawMessage(`{not json`)
	injectPanelEgress(cfg, "direct")
	if len(cfg.InboundConfigs) != 1 {
		t.Fatal("unparsable routing must skip the whole injection, inbound included")
	}
	if string(cfg.RouterConfig) != `{not json` {
		t.Fatal("unparsable routing must be left untouched")
	}
}

func TestInjectPanelEgress_MissingTargetSkips(t *testing.T) {
	cfg := egressTestConfig()
	before := string(cfg.RouterConfig)
	injectPanelEgress(cfg, "removed-subscription-outbound")
	if len(cfg.InboundConfigs) != 1 {
		t.Fatalf("a missing target must not expose the panel bridge, got %+v", cfg.InboundConfigs)
	}
	if string(cfg.RouterConfig) != before {
		t.Fatalf("a missing target must leave routing untouched, got %s", cfg.RouterConfig)
	}
}

func TestInjectPanelEgress_BadOutboundsSkips(t *testing.T) {
	cfg := egressTestConfig()
	cfg.OutboundConfigs = json_util.RawMessage(`{not json`)
	before := string(cfg.RouterConfig)
	injectPanelEgress(cfg, "direct")
	if len(cfg.InboundConfigs) != 1 {
		t.Fatalf("unparsable outbounds must not expose the panel bridge, got %+v", cfg.InboundConfigs)
	}
	if string(cfg.RouterConfig) != before {
		t.Fatalf("unparsable outbounds must leave routing untouched, got %s", cfg.RouterConfig)
	}
}

func TestInjectNodeEgresses_MissingTargetSkips(t *testing.T) {
	cfg := egressTestConfig()
	injectNodeEgresses(cfg, []*model.Node{
		{Id: 1, Enable: true, OutboundTag: "removed-subscription-outbound"},
		{Id: 2, Enable: true, OutboundTag: "warp"},
	})

	if len(cfg.InboundConfigs) != 2 {
		t.Fatalf("only the node with a valid target should get a bridge, got %+v", cfg.InboundConfigs)
	}
	bridge := cfg.InboundConfigs[1]
	if bridge.Tag != NodeEgressInboundTag(2) || bridge.Port != nodeEgressBasePort+2 {
		t.Fatalf("unexpected node egress bridge: %+v", bridge)
	}

	var routing egressRouting
	if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
		t.Fatal(err)
	}
	if len(routing.Rules) != 2 || routing.Rules[0].OutboundTag != "warp" ||
		len(routing.Rules[0].InboundTag) != 1 || routing.Rules[0].InboundTag[0] != NodeEgressInboundTag(2) {
		t.Fatalf("only the valid node egress rule should be prepended, got %+v", routing.Rules)
	}
}

func TestInjectNodeEgresses_BadOutboundsSkips(t *testing.T) {
	cfg := egressTestConfig()
	cfg.OutboundConfigs = json_util.RawMessage(`{not json`)
	before := string(cfg.RouterConfig)
	injectNodeEgresses(cfg, []*model.Node{{Id: 1, Enable: true, OutboundTag: "direct"}})

	if len(cfg.InboundConfigs) != 1 {
		t.Fatalf("unparsable outbounds must not expose a node bridge, got %+v", cfg.InboundConfigs)
	}
	if string(cfg.RouterConfig) != before {
		t.Fatalf("unparsable outbounds must leave routing untouched, got %s", cfg.RouterConfig)
	}
}

func TestInjectNodeEgresses_BalancerTarget(t *testing.T) {
	cfg := egressTestConfig()
	cfg.RouterConfig = json_util.RawMessage(`{"rules":[],"balancers":[{"tag":"lb","selector":["warp"]}]}`)
	injectNodeEgresses(cfg, []*model.Node{{Id: 1, Enable: true, OutboundTag: "lb"}})

	var routing struct {
		Rules []struct {
			OutboundTag string `json:"outboundTag"`
			BalancerTag string `json:"balancerTag"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
		t.Fatal(err)
	}
	if len(cfg.InboundConfigs) != 2 || len(routing.Rules) != 1 ||
		routing.Rules[0].BalancerTag != "lb" || routing.Rules[0].OutboundTag != "" {
		t.Fatalf("a valid balancer target must create the node bridge and rule, got %+v", routing.Rules)
	}
}

func TestInjectNodeEgresses_TagCollisionSkips(t *testing.T) {
	cfg := egressTestConfig()
	cfg.InboundConfigs = append(cfg.InboundConfigs,
		xray.InboundConfig{Port: 1234, Protocol: "socks", Tag: NodeEgressInboundTag(1)},
	)
	before := string(cfg.RouterConfig)
	injectNodeEgresses(cfg, []*model.Node{{Id: 1, Enable: true, OutboundTag: "direct"}})

	if len(cfg.InboundConfigs) != 2 || string(cfg.RouterConfig) != before {
		t.Fatal("an existing node egress tag must make that node injection a no-op")
	}
}

func TestInjectNodeEgresses_PortCollision(t *testing.T) {
	cfg := egressTestConfig()
	cfg.InboundConfigs = append(cfg.InboundConfigs,
		xray.InboundConfig{Port: nodeEgressBasePort + 1, Protocol: "vless", Tag: "in-1"},
		xray.InboundConfig{Port: nodeEgressBasePort + 2, Protocol: "vless", Tag: "in-2"},
	)
	injectNodeEgresses(cfg, []*model.Node{{Id: 1, Enable: true, OutboundTag: "direct"}})

	bridge := cfg.InboundConfigs[len(cfg.InboundConfigs)-1]
	if bridge.Tag != NodeEgressInboundTag(1) || bridge.Port != nodeEgressBasePort+3 {
		t.Fatalf("node egress must skip taken ports, got %+v", bridge)
	}
}

func TestInjectNodeEgresses_NoRoutingSection(t *testing.T) {
	cfg := egressTestConfig()
	cfg.RouterConfig = nil
	injectNodeEgresses(cfg, []*model.Node{{Id: 1, Enable: true, OutboundTag: "direct"}})

	var routing egressRouting
	if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
		t.Fatal(err)
	}
	if len(cfg.InboundConfigs) != 2 || len(routing.Rules) != 1 ||
		routing.Rules[0].OutboundTag != "direct" ||
		len(routing.Rules[0].InboundTag) != 1 || routing.Rules[0].InboundTag[0] != NodeEgressInboundTag(1) {
		t.Fatalf("a routing section must be created with the node egress rule, got %+v", routing.Rules)
	}
}

func TestInjectNodeEgresses_BadRoutingSkips(t *testing.T) {
	cfg := egressTestConfig()
	cfg.RouterConfig = json_util.RawMessage(`{not json`)
	injectNodeEgresses(cfg, []*model.Node{{Id: 1, Enable: true, OutboundTag: "direct"}})

	if len(cfg.InboundConfigs) != 1 {
		t.Fatalf("unparsable routing must not expose a node bridge, got %+v", cfg.InboundConfigs)
	}
	if string(cfg.RouterConfig) != `{not json` {
		t.Fatalf("unparsable routing must be left untouched, got %s", cfg.RouterConfig)
	}
}

func mtprotoInbound(tag string, settings string) *model.Inbound {
	return &model.Inbound{Tag: tag, Protocol: model.MTProto, Enable: true, Settings: settings}
}

func TestInjectMtprotoEgress_WithOutbound(t *testing.T) {
	cfg := egressTestConfig()
	injectMtprotoEgress(cfg, mtprotoInbound("inbound-443",
		`{"routeThroughXray":true,"routeXrayPort":50000,"outboundTag":"warp"}`))

	if len(cfg.InboundConfigs) != 2 {
		t.Fatalf("expected the bridge inbound to be appended, got %d", len(cfg.InboundConfigs))
	}
	ib := cfg.InboundConfigs[1]
	if ib.Tag != "inbound-443" || ib.Protocol != "socks" || ib.Port != 50000 {
		t.Fatalf("unexpected bridge inbound: %+v", ib)
	}
	if string(ib.Listen) != `"127.0.0.1"` {
		t.Fatalf("bridge must listen on loopback, got %s", ib.Listen)
	}

	var routing egressRouting
	if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
		t.Fatal(err)
	}
	if len(routing.Rules) != 2 {
		t.Fatalf("expected the egress rule prepended to the existing rule, got %+v", routing.Rules)
	}
	first := routing.Rules[0]
	if first.Type != "field" || first.OutboundTag != "warp" ||
		len(first.InboundTag) != 1 || first.InboundTag[0] != "inbound-443" {
		t.Fatalf("egress rule must bind the inbound tag to the outbound, got %+v", first)
	}
}

func TestInjectMtprotoEgress_NoOutboundLeavesRouting(t *testing.T) {
	cfg := egressTestConfig()
	before := string(cfg.RouterConfig)
	injectMtprotoEgress(cfg, mtprotoInbound("inbound-443",
		`{"routeThroughXray":true,"routeXrayPort":50001}`))

	if len(cfg.InboundConfigs) != 2 || cfg.InboundConfigs[1].Port != 50001 {
		t.Fatalf("bridge must still be appended without an outbound, got %+v", cfg.InboundConfigs)
	}
	if string(cfg.RouterConfig) != before {
		t.Fatalf("no outbound means no rule change, got %s", cfg.RouterConfig)
	}
}

func TestInjectMtprotoEgress_BalancerTag(t *testing.T) {
	cfg := egressTestConfig()
	cfg.RouterConfig = json_util.RawMessage(`{"rules":[],"balancers":[{"tag":"lb","selector":["warp"]}]}`)
	injectMtprotoEgress(cfg, mtprotoInbound("inbound-443",
		`{"routeThroughXray":true,"routeXrayPort":50002,"outboundTag":"lb"}`))

	var routing struct {
		Rules []struct {
			OutboundTag string `json:"outboundTag"`
			BalancerTag string `json:"balancerTag"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
		t.Fatal(err)
	}
	if len(routing.Rules) != 1 || routing.Rules[0].BalancerTag != "lb" || routing.Rules[0].OutboundTag != "" {
		t.Fatalf("a balancer tag must target balancerTag, got %+v", routing.Rules)
	}
}

func TestInjectMtprotoEgress_Disabled(t *testing.T) {
	// Not routed, and routed-but-portless, are both no-ops.
	for _, settings := range []string{
		`{"routeThroughXray":false,"routeXrayPort":50000}`,
		`{"routeThroughXray":true}`,
		`{"routeThroughXray":true,"routeXrayPort":0}`,
	} {
		cfg := egressTestConfig()
		before := string(cfg.RouterConfig)
		injectMtprotoEgress(cfg, mtprotoInbound("inbound-443", settings))
		if len(cfg.InboundConfigs) != 1 || string(cfg.RouterConfig) != before {
			t.Fatalf("settings %s must be a no-op, got %d inbounds", settings, len(cfg.InboundConfigs))
		}
	}
}

func TestInjectMtprotoEgress_TagCollisionSkips(t *testing.T) {
	cfg := egressTestConfig()
	cfg.InboundConfigs = append(cfg.InboundConfigs,
		xray.InboundConfig{Port: 443, Protocol: "vless", Tag: "inbound-443"})
	before := string(cfg.RouterConfig)
	injectMtprotoEgress(cfg, mtprotoInbound("inbound-443",
		`{"routeThroughXray":true,"routeXrayPort":50003,"outboundTag":"warp"}`))
	if len(cfg.InboundConfigs) != 2 || string(cfg.RouterConfig) != before {
		t.Fatal("a real inbound already owning the tag must make the bridge a no-op")
	}
}

func TestInjectMtprotoEgress_MissingTargetSkips(t *testing.T) {
	cfg := egressTestConfig()
	before := string(cfg.RouterConfig)
	injectMtprotoEgress(cfg, mtprotoInbound("inbound-443",
		`{"routeThroughXray":true,"routeXrayPort":50004,"outboundTag":"removed-subscription-outbound"}`))

	if len(cfg.InboundConfigs) != 1 {
		t.Fatalf("a missing target must not expose the mtproto bridge, got %+v", cfg.InboundConfigs)
	}
	if string(cfg.RouterConfig) != before {
		t.Fatalf("a missing target must leave routing untouched, got %s", cfg.RouterConfig)
	}
}

func TestInjectMtprotoEgress_BadOutboundsSkips(t *testing.T) {
	cfg := egressTestConfig()
	cfg.OutboundConfigs = json_util.RawMessage(`{not json`)
	before := string(cfg.RouterConfig)
	injectMtprotoEgress(cfg, mtprotoInbound("inbound-443",
		`{"routeThroughXray":true,"routeXrayPort":50005,"outboundTag":"direct"}`))

	if len(cfg.InboundConfigs) != 1 {
		t.Fatalf("unparsable outbounds must not expose the mtproto bridge, got %+v", cfg.InboundConfigs)
	}
	if string(cfg.RouterConfig) != before {
		t.Fatalf("unparsable outbounds must leave routing untouched, got %s", cfg.RouterConfig)
	}
}

func TestInjectMtprotoEgress_BadRoutingSkips(t *testing.T) {
	cfg := egressTestConfig()
	cfg.RouterConfig = json_util.RawMessage(`{not json`)
	injectMtprotoEgress(cfg, mtprotoInbound("inbound-443",
		`{"routeThroughXray":true,"routeXrayPort":50006,"outboundTag":"direct"}`))

	if len(cfg.InboundConfigs) != 1 {
		t.Fatalf("unparsable routing must not expose the mtproto bridge, got %+v", cfg.InboundConfigs)
	}
	if string(cfg.RouterConfig) != `{not json` {
		t.Fatalf("unparsable routing must be left untouched, got %s", cfg.RouterConfig)
	}
}

func amneziawgInbound(id int, tag string, clients []model.Client) *model.Inbound {
	server := amneziawg.ServerSettings{SubnetIP: "10.8.1.0", SubnetCIDR: 24}
	settings, _ := json.Marshal(amneziawg.InboundSettings{Server: &server, Clients: clients})
	return &model.Inbound{Id: id, Tag: tag, Protocol: model.AmneziaWG, Enable: true, Settings: string(settings)}
}

func TestInjectAmneziawgnetSocks_CreatesRelayTaggedWithInboundsOwnTag(t *testing.T) {
	cfg := egressTestConfig()
	before := string(cfg.RouterConfig)
	inbound := amneziawgInbound(7, "awg-7", []model.Client{
		{Email: "a@x", Enable: true, PublicKey: "pub-a", AllowedIPs: []string{"10.8.1.2/32"}},
	})
	injectAmneziawgnetSocks(cfg, []*model.Inbound{inbound})

	if len(cfg.InboundConfigs) != 2 {
		t.Fatalf("expected the relay inbound to be appended, got %d inbounds", len(cfg.InboundConfigs))
	}
	ib := cfg.InboundConfigs[1]
	if ib.Tag != "awg-7" || ib.Protocol != "socks" || ib.Port != amneziawgnet.SOCKSPortForInbound(7) {
		t.Fatalf("relay inbound must reuse the inbound's own tag (so per-inbound stats totals keep matching, and it's already selectable in the stock Routing page) and this instance's own derived port, got %+v", ib)
	}
	if string(ib.Listen) != `"127.0.0.1"` {
		t.Fatalf("relay inbound must listen on loopback, got %s", ib.Listen)
	}
	if !strings.Contains(string(ib.Settings), `"auth":"password"`) || !strings.Contains(string(ib.Settings), `"udp":true`) {
		t.Fatalf("relay inbound must require password auth and allow UDP ASSOCIATE, got %s", ib.Settings)
	}
	if !strings.Contains(string(ib.Settings), `"a@x"`) {
		t.Fatalf("relay inbound must have an account for the peer's email, got %s", ib.Settings)
	}
	if !strings.Contains(string(ib.Sniffing), `"enabled":true`) {
		t.Fatalf("relay inbound must enable sniffing -- a peer's own DNS resolution means the decapsulated traffic never carries a domain at the network layer, so domain-based Routing rules can only ever match via sniffing the payload, got %s", ib.Sniffing)
	}
	// No auto-generated routing rule: it's entirely up to the admin's own
	// Routing-page rules, same as any other protocol's inbound tag.
	if string(cfg.RouterConfig) != before {
		t.Fatalf("injectAmneziawgnetSocks must never touch the routing section, got %s", cfg.RouterConfig)
	}
}

func TestInjectAmneziawgnetSocks_MultipleInboundsEachGetOwnRelay(t *testing.T) {
	cfg := egressTestConfig()
	inbound1 := amneziawgInbound(1, "awg-1", []model.Client{
		{Email: "a@x", Enable: true, PublicKey: "pub-a", AllowedIPs: []string{"10.8.1.2/32"}},
	})
	inbound2 := amneziawgInbound(2, "awg-2", []model.Client{
		{Email: "b@x", Enable: true, PublicKey: "pub-b", AllowedIPs: []string{"10.9.1.2/32"}},
	})
	injectAmneziawgnetSocks(cfg, []*model.Inbound{inbound1, inbound2})

	if len(cfg.InboundConfigs) != 3 {
		t.Fatalf("expected one relay inbound per inbound (plus the pre-existing one), got %d inbounds: %+v", len(cfg.InboundConfigs), cfg.InboundConfigs)
	}
	byTag := map[string]int{}
	for _, ib := range cfg.InboundConfigs[1:] {
		byTag[ib.Tag] = ib.Port
	}
	if byTag["awg-1"] != amneziawgnet.SOCKSPortForInbound(1) || byTag["awg-2"] != amneziawgnet.SOCKSPortForInbound(2) {
		t.Fatalf("each inbound must get its own tag and its own derived port, got %+v", byTag)
	}
}

func TestInjectAmneziawgnetSocks_NoQualifyingPeerSkipsRelay(t *testing.T) {
	cases := []struct {
		name   string
		client model.Client
		enable bool
	}{
		{"client disabled", model.Client{Email: "a@x", Enable: false, PublicKey: "pub-a", AllowedIPs: []string{"10.8.1.2/32"}}, true},
		{"no PublicKey", model.Client{Email: "a@x", Enable: true, AllowedIPs: []string{"10.8.1.2/32"}}, true},
		{"no AllowedIPs", model.Client{Email: "a@x", Enable: true, PublicKey: "pub-a"}, true},
		{"inbound disabled", model.Client{Email: "a@x", Enable: true, PublicKey: "pub-a", AllowedIPs: []string{"10.8.1.2/32"}}, false},
		{"no Email", model.Client{Enable: true, PublicKey: "pub-a", AllowedIPs: []string{"10.8.1.2/32"}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := egressTestConfig()
			inbound := amneziawgInbound(1, "awg-1", []model.Client{c.client})
			inbound.Enable = c.enable
			injectAmneziawgnetSocks(cfg, []*model.Inbound{inbound})
			if len(cfg.InboundConfigs) != 1 {
				t.Fatalf("%s must be a no-op, got %d inbounds", c.name, len(cfg.InboundConfigs))
			}
		})
	}
}

func TestInjectAmneziawgnetSocks_AlwaysOnRegardlessOfLegacyRouteThroughXrayField(t *testing.T) {
	// Unlike the retired kernel-module bridge, the embedded relay has no
	// opt-in gate: there is no alternative datapath once traffic is
	// decapsulated in gVisor. A stale RouteThroughXray=false left over from
	// a pre-cutover install must not suppress the relay inbound.
	cfg := egressTestConfig()
	server := amneziawg.ServerSettings{SubnetIP: "10.8.1.0", SubnetCIDR: 24, RouteThroughXray: false}
	settings, _ := json.Marshal(amneziawg.InboundSettings{
		Server: &server,
		Clients: []model.Client{
			{Email: "a@x", Enable: true, PublicKey: "pub-a", AllowedIPs: []string{"10.8.1.2/32"}},
		},
	})
	inbound := &model.Inbound{Id: 1, Tag: "awg-1", Protocol: model.AmneziaWG, Enable: true, Settings: string(settings)}
	injectAmneziawgnetSocks(cfg, []*model.Inbound{inbound})
	if len(cfg.InboundConfigs) != 2 {
		t.Fatalf("the relay inbound must always be created regardless of RouteThroughXray, got %+v", cfg.InboundConfigs)
	}
}

func TestInjectAmneziawgnetSocks_WrongProtocolOrNodeSkipped(t *testing.T) {
	cfg := egressTestConfig()
	vless := &model.Inbound{Id: 1, Tag: "in-1", Protocol: model.VLESS, Enable: true}
	nodeID := 5
	nodeHosted := amneziawgInbound(2, "awg-2", []model.Client{
		{Email: "a@x", Enable: true, PublicKey: "pub-a", AllowedIPs: []string{"10.8.1.2/32"}},
	})
	nodeHosted.NodeID = &nodeID
	injectAmneziawgnetSocks(cfg, []*model.Inbound{vless, nodeHosted})
	if len(cfg.InboundConfigs) != 1 {
		t.Fatalf("a non-AmneziaWG or node-hosted inbound must never get a relay inbound, got %+v", cfg.InboundConfigs)
	}
}

func TestInjectAmneziawgnetSocks_TagCollisionSkipsThatInboundOnly(t *testing.T) {
	cfg := egressTestConfig()
	cfg.InboundConfigs = append(cfg.InboundConfigs,
		xray.InboundConfig{Port: 1234, Protocol: "vless", Tag: "awg-1"})
	inbound1 := amneziawgInbound(1, "awg-1", []model.Client{
		{Email: "a@x", Enable: true, PublicKey: "pub-a", AllowedIPs: []string{"10.8.1.2/32"}},
	})
	inbound2 := amneziawgInbound(2, "awg-2", []model.Client{
		{Email: "b@x", Enable: true, PublicKey: "pub-b", AllowedIPs: []string{"10.9.1.2/32"}},
	})
	injectAmneziawgnetSocks(cfg, []*model.Inbound{inbound1, inbound2})

	// Started with 2 (api + the colliding vless entry); only awg-2's relay
	// inbound should have been added, awg-1's skipped since its tag is taken.
	if len(cfg.InboundConfigs) != 3 {
		t.Fatalf("expected only the non-colliding inbound's relay inbound to be added, got %+v", cfg.InboundConfigs)
	}
	found := false
	for _, ib := range cfg.InboundConfigs {
		if ib.Tag == "awg-2" && ib.Protocol == "socks" {
			found = true
		}
	}
	if !found {
		t.Fatal("awg-2's relay inbound must still be created despite awg-1's tag collision")
	}
}

// amneziawgV6Inbound builds an AmneziaWG inbound with IPv6 enabled and a
// given external interface -- amneziawgInbound's own ServerSettings never
// sets these, so injectAmneziawgV6Egress's tests need their own variant.
func amneziawgV6Inbound(id int, tag string, ext6 string, clients []model.Client) *model.Inbound {
	server := amneziawg.ServerSettings{
		SubnetIP: "10.8.1.0", SubnetCIDR: 24,
		IPv6Enabled: true, IPv6ExternalInterface: ext6,
	}
	settings, _ := json.Marshal(amneziawg.InboundSettings{Server: &server, Clients: clients})
	return &model.Inbound{Id: id, Tag: tag, Protocol: model.AmneziaWG, Enable: true, Settings: string(settings)}
}

// amneziawgV6InboundNotActive builds an inbound that fails V6AliasesActive
// (either toggle can do it), unlike amneziawgV6Inbound which always passes it.
func amneziawgV6InboundNotActive(id int, tag string, ipv6Enabled bool, ext6 string, clients []model.Client) *model.Inbound {
	server := amneziawg.ServerSettings{
		SubnetIP: "10.8.1.0", SubnetCIDR: 24,
		IPv6Enabled: ipv6Enabled, IPv6ExternalInterface: ext6,
	}
	settings, _ := json.Marshal(amneziawg.InboundSettings{Server: &server, Clients: clients})
	return &model.Inbound{Id: id, Tag: tag, Protocol: model.AmneziaWG, Enable: true, Settings: string(settings)}
}

// injectAmneziawgV6Egress runs after injectAmneziawgnetSocks in the real
// GetXrayConfig() pipeline and depends on its relay inbound already
// existing (see the "live" tag check) -- every test below calls both, in
// that order, to match production.
func injectAmneziawgSocksThenV6(cfg *xray.Config, inbounds []*model.Inbound) {
	injectAmneziawgnetSocks(cfg, inbounds)
	injectAmneziawgV6Egress(cfg, inbounds)
}

type v6EgressRouting struct {
	Rules []struct {
		InboundTag  []string `json:"inboundTag"`
		User        []string `json:"user"`
		OutboundTag string   `json:"outboundTag"`
		Type        string   `json:"type"`
	} `json:"rules"`
}

type v6EgressOutbound struct {
	Tag         string `json:"tag"`
	Protocol    string `json:"protocol"`
	SendThrough string `json:"sendThrough"`
}

func TestInjectAmneziawgV6Egress_CreatesOutboundAndRuleForV6Peer(t *testing.T) {
	cfg := egressTestConfig()
	inbound := amneziawgV6Inbound(7, "awg-7", "eth0", []model.Client{
		{Email: "a@x", Enable: true, PublicKey: "pub-a", AllowedIPs: []string{"10.8.1.2/32", "fd86:ea04:1115::2/128"}},
	})
	injectAmneziawgSocksThenV6(cfg, []*model.Inbound{inbound})

	var outbounds []v6EgressOutbound
	if err := json.Unmarshal(cfg.OutboundConfigs, &outbounds); err != nil {
		t.Fatal(err)
	}
	wantTag := amneziawgV6EgressTag(7, "a@x")
	var got *v6EgressOutbound
	for i := range outbounds {
		if outbounds[i].Tag == wantTag {
			got = &outbounds[i]
		}
	}
	if got == nil {
		t.Fatalf("expected an outbound tagged %q, got %+v", wantTag, outbounds)
	}
	if got.Protocol != "freedom" || got.SendThrough != "fd86:ea04:1115::2" {
		t.Fatalf("outbound must be a freedom outbound bound to the peer's own v6 address, got %+v", got)
	}
	// Pre-existing outbounds (direct, warp) must survive untouched.
	if len(outbounds) != 3 {
		t.Fatalf("expected the 2 pre-existing outbounds plus 1 new one, got %+v", outbounds)
	}

	var routing v6EgressRouting
	if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
		t.Fatal(err)
	}
	ruleIdx := -1
	for i := range routing.Rules {
		if routing.Rules[i].OutboundTag == wantTag {
			ruleIdx = i
		}
	}
	if ruleIdx == -1 {
		t.Fatalf("expected a routing rule targeting %q, got %+v", wantTag, routing.Rules)
	}
	rule := routing.Rules[ruleIdx]
	if rule.Type != "field" || len(rule.User) != 1 || rule.User[0] != "a@x" ||
		len(rule.InboundTag) != 1 || rule.InboundTag[0] != "awg-7" {
		t.Fatalf("rule must match this peer's email and inbound tag, got %+v", rule)
	}
}

func TestInjectAmneziawgV6Egress_SkipsPeerWithoutV6Address(t *testing.T) {
	cfg := egressTestConfig()
	before := string(cfg.OutboundConfigs)
	inbound := amneziawgV6Inbound(1, "awg-1", "eth0", []model.Client{
		{Email: "a@x", Enable: true, PublicKey: "pub-a", AllowedIPs: []string{"10.8.1.2/32"}}, // v4 only
	})
	injectAmneziawgSocksThenV6(cfg, []*model.Inbound{inbound})
	if string(cfg.OutboundConfigs) != before {
		t.Fatalf("a peer with no v6 AllowedIPs entry must not get an outbound, got %s", cfg.OutboundConfigs)
	}
}

// The documented "leave the interface blank to auto-detect" happy path must
// not silently emit a sendThrough for an address the host was never told to
// own -- there is no auto-detect, so that would fail every connection.
func TestInjectAmneziawgV6Egress_SkipsWhenIPv6EnabledButInterfaceBlank(t *testing.T) {
	cfg := egressTestConfig()
	before := string(cfg.OutboundConfigs)
	inbound := amneziawgV6InboundNotActive(1, "awg-1", true, "", []model.Client{
		{Email: "a@x", Enable: true, PublicKey: "pub-a", AllowedIPs: []string{"10.8.1.2/32", "fd86::2/128"}},
	})
	injectAmneziawgSocksThenV6(cfg, []*model.Inbound{inbound})
	if string(cfg.OutboundConfigs) != before {
		t.Fatalf("IPv6Enabled with a blank interface must not get an outbound (no auto-detect exists), got %s", cfg.OutboundConfigs)
	}
}

// The inverse of amneziawgV6Inbound's own always-true IPv6Enabled: a filled
// IPv6ExternalInterface alone (e.g. left over from a previous enable) must
// not activate egress on its own.
func TestInjectAmneziawgV6Egress_SkipsWhenIPv6DisabledEvenWithInterfaceSet(t *testing.T) {
	cfg := egressTestConfig()
	before := string(cfg.OutboundConfigs)
	inbound := amneziawgV6InboundNotActive(1, "awg-1", false, "eth0", []model.Client{
		{Email: "a@x", Enable: true, PublicKey: "pub-a", AllowedIPs: []string{"10.8.1.2/32", "fd86::2/128"}},
	})
	injectAmneziawgSocksThenV6(cfg, []*model.Inbound{inbound})
	if string(cfg.OutboundConfigs) != before {
		t.Fatalf("IPv6Enabled false must not get an outbound even with a leftover interface set, got %s", cfg.OutboundConfigs)
	}
}

func TestInjectAmneziawgV6Egress_MultiplePeersEachGetOwnOutboundAndRule(t *testing.T) {
	cfg := egressTestConfig()
	inbound := amneziawgV6Inbound(1, "awg-1", "eth0", []model.Client{
		{Email: "a@x", Enable: true, PublicKey: "pub-a", AllowedIPs: []string{"fd86:ea04:1115::2/128"}},
		{Email: "b@x", Enable: true, PublicKey: "pub-b", AllowedIPs: []string{"fd86:ea04:1115::3/128"}},
	})
	injectAmneziawgSocksThenV6(cfg, []*model.Inbound{inbound})

	var outbounds []v6EgressOutbound
	if err := json.Unmarshal(cfg.OutboundConfigs, &outbounds); err != nil {
		t.Fatal(err)
	}
	tagA, tagB := amneziawgV6EgressTag(1, "a@x"), amneziawgV6EgressTag(1, "b@x")
	seen := map[string]string{}
	for _, o := range outbounds {
		seen[o.Tag] = o.SendThrough
	}
	if seen[tagA] != "fd86:ea04:1115::2" || seen[tagB] != "fd86:ea04:1115::3" {
		t.Fatalf("each peer must get its own outbound bound to its own address, got %+v", seen)
	}
}

func TestInjectAmneziawgV6Egress_StableTagAcrossRegenerations(t *testing.T) {
	// Same instance data, two independent injections -- hot_diff.go relies on
	// the tag being a pure function of (inboundID, email) so it recognizes
	// "unchanged" rather than remove+recreate on every poll.
	inbound := amneziawgV6Inbound(1, "awg-1", "eth0", []model.Client{
		{Email: "a@x", Enable: true, PublicKey: "pub-a", AllowedIPs: []string{"fd86:ea04:1115::2/128"}},
	})
	cfg1 := egressTestConfig()
	injectAmneziawgSocksThenV6(cfg1, []*model.Inbound{inbound})
	cfg2 := egressTestConfig()
	injectAmneziawgSocksThenV6(cfg2, []*model.Inbound{inbound})

	var out1, out2 []v6EgressOutbound
	json.Unmarshal(cfg1.OutboundConfigs, &out1)
	json.Unmarshal(cfg2.OutboundConfigs, &out2)
	if len(out1) != len(out2) || out1[len(out1)-1].Tag != out2[len(out2)-1].Tag {
		t.Fatalf("tag must be stable across independent regenerations, got %+v vs %+v", out1, out2)
	}
}

func TestInjectAmneziawgV6Egress_SkipsWrongProtocolOrNodeHostedOrDisabled(t *testing.T) {
	cfg := egressTestConfig()
	before := string(cfg.OutboundConfigs)
	vless := &model.Inbound{Id: 1, Tag: "in-1", Protocol: model.VLESS, Enable: true}
	nodeID := 5
	nodeHosted := amneziawgV6Inbound(2, "awg-2", "eth0", []model.Client{
		{Email: "a@x", Enable: true, PublicKey: "pub-a", AllowedIPs: []string{"fd86:ea04:1115::2/128"}},
	})
	nodeHosted.NodeID = &nodeID
	disabled := amneziawgV6Inbound(3, "awg-3", "eth0", []model.Client{
		{Email: "b@x", Enable: true, PublicKey: "pub-b", AllowedIPs: []string{"fd86:ea04:1115::3/128"}},
	})
	disabled.Enable = false
	injectAmneziawgSocksThenV6(cfg, []*model.Inbound{vless, nodeHosted, disabled})
	if string(cfg.OutboundConfigs) != before {
		t.Fatalf("wrong-protocol, node-hosted, and disabled inbounds must never get a v6 outbound, got %s", cfg.OutboundConfigs)
	}
}

func TestInjectAmneziawgV6Egress_SkipsWhenRelayInboundNotCreated(t *testing.T) {
	cfg := egressTestConfig()
	// A pre-existing inbound already holds this AmneziaWG inbound's tag, so
	// injectAmneziawgnetSocks (called first, matching production order)
	// skips creating its relay SOCKS5 inbound entirely.
	cfg.InboundConfigs = append(cfg.InboundConfigs,
		xray.InboundConfig{Port: 1234, Protocol: "vless", Tag: "awg-1"})
	before := string(cfg.OutboundConfigs)
	inbound := amneziawgV6Inbound(1, "awg-1", "eth0", []model.Client{
		{Email: "a@x", Enable: true, PublicKey: "pub-a", AllowedIPs: []string{"fd86:ea04:1115::2/128"}},
	})
	injectAmneziawgSocksThenV6(cfg, []*model.Inbound{inbound})
	if string(cfg.OutboundConfigs) != before {
		t.Fatalf("no v6 outbound should be created when the relay inbound itself never got created, got %s", cfg.OutboundConfigs)
	}
}

func TestInjectAmneziawgV6Egress_OutboundTagCollisionSkipsThatPeerOnly(t *testing.T) {
	cfg := egressTestConfig()
	inbound := amneziawgV6Inbound(1, "awg-1", "eth0", []model.Client{
		{Email: "a@x", Enable: true, PublicKey: "pub-a", AllowedIPs: []string{"fd86:ea04:1115::2/128"}},
		{Email: "b@x", Enable: true, PublicKey: "pub-b", AllowedIPs: []string{"fd86:ea04:1115::3/128"}},
	})
	// Pre-seed a colliding outbound tag for a@x specifically.
	collidingTag := amneziawgV6EgressTag(1, "a@x")
	existing, _ := json.Marshal([]any{map[string]any{"tag": collidingTag, "protocol": "freedom"}})
	cfg.OutboundConfigs = json_util.RawMessage(existing)

	injectAmneziawgSocksThenV6(cfg, []*model.Inbound{inbound})

	var outbounds []v6EgressOutbound
	if err := json.Unmarshal(cfg.OutboundConfigs, &outbounds); err != nil {
		t.Fatal(err)
	}
	tagB := amneziawgV6EgressTag(1, "b@x")
	foundB := false
	countA := 0
	for _, o := range outbounds {
		if o.Tag == collidingTag {
			countA++
		}
		if o.Tag == tagB {
			foundB = true
		}
	}
	if countA != 1 {
		t.Fatalf("a@x's pre-existing outbound must not be duplicated, got %d copies", countA)
	}
	if !foundB {
		t.Fatal("b@x must still get its own outbound despite a@x's tag collision")
	}
}

func TestInjectAmneziawgV6Egress_BadOutboundsOrRoutingSkips(t *testing.T) {
	inbound := amneziawgV6Inbound(1, "awg-1", "eth0", []model.Client{
		{Email: "a@x", Enable: true, PublicKey: "pub-a", AllowedIPs: []string{"fd86:ea04:1115::2/128"}},
	})

	cfg := egressTestConfig()
	cfg.OutboundConfigs = json_util.RawMessage(`{not json`)
	injectAmneziawgSocksThenV6(cfg, []*model.Inbound{inbound})
	if string(cfg.OutboundConfigs) != `{not json` {
		t.Fatalf("unparsable outbounds must be left untouched, got %s", cfg.OutboundConfigs)
	}

	cfg2 := egressTestConfig()
	cfg2.RouterConfig = json_util.RawMessage(`{not json`)
	injectAmneziawgSocksThenV6(cfg2, []*model.Inbound{inbound})
	if string(cfg2.RouterConfig) != `{not json` {
		t.Fatalf("unparsable routing must be left untouched, got %s", cfg2.RouterConfig)
	}
}

func TestInjectAmneziawgV6Egress_NoQualifyingPeerLeavesConfigUntouched(t *testing.T) {
	cfg := egressTestConfig()
	beforeOut, beforeRoute := string(cfg.OutboundConfigs), string(cfg.RouterConfig)
	inbound := amneziawgV6Inbound(1, "awg-1", "eth0", nil) // no clients at all
	injectAmneziawgV6Egress(cfg, []*model.Inbound{inbound})
	if string(cfg.OutboundConfigs) != beforeOut || string(cfg.RouterConfig) != beforeRoute {
		t.Fatalf("an inbound with no qualifying peer must leave the config byte-identical")
	}
}

func TestInjectAmneziawgV6Egress_RulesPrependedBeforeExistingRules(t *testing.T) {
	cfg := egressTestConfig() // already has one rule, targeting "api"
	inbound := amneziawgV6Inbound(1, "awg-1", "eth0", []model.Client{
		{Email: "a@x", Enable: true, PublicKey: "pub-a", AllowedIPs: []string{"fd86:ea04:1115::2/128"}},
	})
	injectAmneziawgSocksThenV6(cfg, []*model.Inbound{inbound})

	var routing v6EgressRouting
	if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
		t.Fatal(err)
	}
	if len(routing.Rules) != 2 {
		t.Fatalf("expected the new rule plus the pre-existing one, got %+v", routing.Rules)
	}
	if routing.Rules[0].OutboundTag != amneziawgV6EgressTag(1, "a@x") {
		t.Fatalf("the new infra rule must be prepended ahead of the pre-existing rule, got %+v", routing.Rules[0])
	}
	if routing.Rules[1].OutboundTag != "api" {
		t.Fatalf("the pre-existing rule must survive, got %+v", routing.Rules[1])
	}
}
