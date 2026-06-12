package mtproto

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestParseMetricLine(t *testing.T) {
	name, labels, val, err := parseMetricLine(`mtg_traffic{direction="to_client"} 12345`)
	if err != nil {
		t.Fatal(err)
	}
	if name != "mtg_traffic" {
		t.Fatalf("name=%q", name)
	}
	if labels["direction"] != "to_client" {
		t.Fatalf("labels=%v", labels)
	}
	if val != 12345 {
		t.Fatalf("val=%v", val)
	}

	name2, _, val2, err2 := parseMetricLine(`mtg_concurrency 7`)
	if err2 != nil {
		t.Fatal(err2)
	}
	if name2 != "mtg_concurrency" || val2 != 7 {
		t.Fatalf("got %q %v", name2, val2)
	}
}

func TestInstanceFromInbound(t *testing.T) {
	ib := &model.Inbound{
		Id:       3,
		Tag:      "inbound-3",
		Listen:   "0.0.0.0",
		Port:     8443,
		Protocol: model.MTProto,
		Settings: `{"fakeTlsDomain":"example.com","secret":"",` +
			`"debug":true,"proxyProtocolListener":true,"preferIp":"prefer-ipv4",` +
			`"domainFronting":{"ip":"127.0.0.1","port":9443,"proxyProtocol":true},` +
			`"routeThroughXray":true,"routeXrayPort":50000}`,
	}
	inst, ok := InstanceFromInbound(ib)
	if !ok {
		t.Fatal("expected a usable instance")
	}
	if inst.Secret == "" {
		t.Fatal("secret should be healed to a non-empty value")
	}
	if inst.Port != 8443 || inst.Id != 3 {
		t.Fatalf("bad instance %+v", inst)
	}
	if !inst.Debug || !inst.ProxyProtocolListener || inst.PreferIP != "prefer-ipv4" {
		t.Fatalf("scalar options not parsed: %+v", inst)
	}
	if inst.FrontingIP != "127.0.0.1" || inst.FrontingPort != 9443 || !inst.FrontingProxyProtocol {
		t.Fatalf("domain-fronting not parsed: %+v", inst)
	}
	if !inst.RouteThroughXray || inst.XrayRoutePort != 50000 {
		t.Fatalf("xray routing not parsed: %+v", inst)
	}

	if _, ok := InstanceFromInbound(&model.Inbound{Protocol: model.VLESS}); ok {
		t.Fatal("non-mtproto inbound should not produce an instance")
	}
}

func TestRenderConfig(t *testing.T) {
	// A bare instance emits only the required keys and the prometheus block,
	// with no optional keys and no [domain-fronting] section.
	bare := renderConfig(Instance{Secret: "ee00", Listen: "0.0.0.0", Port: 8443}, 5000)
	for _, unwanted := range []string{"debug", "proxy-protocol-listener", "prefer-ip", "[domain-fronting]"} {
		if strings.Contains(bare, unwanted) {
			t.Fatalf("bare config should not contain %q:\n%s", unwanted, bare)
		}
	}
	if !strings.Contains(bare, `bind-to = "0.0.0.0:8443"`) {
		t.Fatalf("missing bind-to:\n%s", bare)
	}
	if !strings.Contains(bare, "[stats.prometheus]") || !strings.Contains(bare, "127.0.0.1:5000") {
		t.Fatalf("prometheus block must always be present:\n%s", bare)
	}

	// A fully configured instance emits every option and the fronting section.
	full := renderConfig(Instance{
		Secret: "ee11", Listen: "0.0.0.0", Port: 443,
		Debug: true, ProxyProtocolListener: true, PreferIP: "only-ipv6",
		FrontingIP: "127.0.0.1", FrontingPort: 9443, FrontingProxyProtocol: true,
	}, 6000)
	for _, want := range []string{
		"debug = true\n",
		"proxy-protocol-listener = true\n",
		`prefer-ip = "only-ipv6"`,
		"[domain-fronting]",
		`ip = "127.0.0.1"`,
		"port = 9443",
		"proxy-protocol = true\n",
	} {
		if !strings.Contains(full, want) {
			t.Fatalf("full config missing %q:\n%s", want, full)
		}
	}
	// TOML requires top-level keys before any [section] header.
	if strings.Index(full, "prefer-ip") > strings.Index(full, "[domain-fronting]") {
		t.Fatalf("top-level keys must precede the [domain-fronting] section:\n%s", full)
	}
	if strings.LastIndex(full, "[domain-fronting]") > strings.Index(full, "[stats.prometheus]") {
		t.Fatalf("[domain-fronting] must precede [stats.prometheus]:\n%s", full)
	}
}

func TestRenderConfigXrayEgress(t *testing.T) {
	// Routing through Xray emits a [network] proxies upstream pointing at the
	// loopback SOCKS bridge, before the prometheus block.
	routed := renderConfig(Instance{
		Secret: "ee22", Listen: "0.0.0.0", Port: 443,
		RouteThroughXray: true, XrayRoutePort: 50000,
	}, 7000)
	if !strings.Contains(routed, "[network]") ||
		!strings.Contains(routed, `proxies = ["socks5://127.0.0.1:50000"]`) {
		t.Fatalf("routed config must emit the SOCKS upstream:\n%s", routed)
	}
	if strings.Index(routed, "[network]") > strings.Index(routed, "[stats.prometheus]") {
		t.Fatalf("[network] must precede [stats.prometheus]:\n%s", routed)
	}

	// Without the flag (or without a port) the section is omitted.
	for _, inst := range []Instance{
		{Secret: "ee", Listen: "0.0.0.0", Port: 443},
		{Secret: "ee", Listen: "0.0.0.0", Port: 443, RouteThroughXray: true},
	} {
		if got := renderConfig(inst, 7000); strings.Contains(got, "[network]") {
			t.Fatalf("unrouted config must omit [network]:\n%s", got)
		}
	}
}

func TestFingerprintReactsToOptions(t *testing.T) {
	base := Instance{Secret: "ee", Listen: "0.0.0.0", Port: 443}
	for name, mutate := range map[string]func(*Instance){
		"debug":         func(i *Instance) { i.Debug = true },
		"listener":      func(i *Instance) { i.ProxyProtocolListener = true },
		"preferIp":      func(i *Instance) { i.PreferIP = "only-ipv4" },
		"frontingIP":    func(i *Instance) { i.FrontingIP = "127.0.0.1" },
		"frontingPort":  func(i *Instance) { i.FrontingPort = 9443 },
		"frontingProxy": func(i *Instance) { i.FrontingProxyProtocol = true },
		"routeXray":     func(i *Instance) { i.RouteThroughXray = true },
		"routeXrayPort": func(i *Instance) { i.XrayRoutePort = 50000 },
	} {
		changed := base
		mutate(&changed)
		if base.fingerprint() == changed.fingerprint() {
			t.Fatalf("fingerprint must change when %s changes", name)
		}
	}
}
