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
