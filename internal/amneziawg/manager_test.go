package amneziawg

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func mkInboundSettings(t *testing.T, server *ServerSettings, clients []model.Client) string {
	t.Helper()
	bs, err := json.Marshal(InboundSettings{Server: server, Clients: clients})
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	return string(bs)
}

func validServer() *ServerSettings {
	return &ServerSettings{
		PrivateKey: "serverPriv",
		PublicKey:  "serverPub",
		SubnetIP:   "10.8.1.0",
		SubnetCIDR: 24,
	}
}

func TestInstanceFromInboundParsesEnabledPeers(t *testing.T) {
	settings := mkInboundSettings(t, validServer(), []model.Client{
		{Email: "a@x", Enable: true, PublicKey: "pubA", PreSharedKey: "pskA", AllowedIPs: []string{"10.8.1.2/32"}},
		{Email: "b@x", Enable: false, PublicKey: "pubB", AllowedIPs: []string{"10.8.1.3/32"}},
		{Email: "c@x", Enable: true, PublicKey: "", AllowedIPs: []string{"10.8.1.4/32"}}, // no key: skipped
		{Email: "d@x", Enable: true, PublicKey: "pubD", AllowedIPs: nil},                 // no address: skipped
	})
	ib := &model.Inbound{Id: 7, Tag: "awg-tag", Protocol: model.AmneziaWG, Port: 51820, Settings: settings}

	inst, ok := InstanceFromInbound(ib)
	if !ok {
		t.Fatal("expected a usable instance")
	}
	if inst.Id != 7 || inst.Tag != "awg-tag" || inst.ListenPort != 51820 {
		t.Fatalf("instance identity not carried over: %+v", inst)
	}
	if inst.InterfaceName != "awg7" {
		t.Fatalf("InterfaceName = %q, want awg7", inst.InterfaceName)
	}
	if len(inst.Address) != 1 || inst.Address[0] != "10.8.1.1/24" {
		t.Fatalf("Address = %v, want [10.8.1.1/24]", inst.Address)
	}
	if len(inst.Peers) != 1 {
		t.Fatalf("Peers = %+v, want exactly 1 (only a@x qualifies)", inst.Peers)
	}
	p := inst.Peers[0]
	if p.Email != "a@x" || p.PublicKey != "pubA" || p.PresharedKey != "pskA" || len(p.AllowedIPs) != 1 || p.AllowedIPs[0] != "10.8.1.2/32" {
		t.Fatalf("peer mismatch: %+v", p)
	}
}

func TestInstanceFromInboundRejectsWrongProtocol(t *testing.T) {
	settings := mkInboundSettings(t, validServer(), []model.Client{
		{Email: "a@x", Enable: true, PublicKey: "pubA", AllowedIPs: []string{"10.8.1.2/32"}},
	})
	ib := &model.Inbound{Id: 1, Protocol: model.VLESS, Settings: settings}
	if _, ok := InstanceFromInbound(ib); ok {
		t.Fatal("non-AmneziaWG inbound must be rejected")
	}
}

func TestInstanceFromInboundRejectsNil(t *testing.T) {
	if _, ok := InstanceFromInbound(nil); ok {
		t.Fatal("nil inbound must be rejected")
	}
}

func TestInstanceFromInboundRejectsMissingServer(t *testing.T) {
	ib := &model.Inbound{Id: 1, Protocol: model.AmneziaWG, Settings: `{"clients":[]}`}
	if _, ok := InstanceFromInbound(ib); ok {
		t.Fatal("settings with no server block must be rejected")
	}
}

func TestInstanceFromInboundRejectsUnparseableSettings(t *testing.T) {
	ib := &model.Inbound{Id: 1, Protocol: model.AmneziaWG, Settings: `not json`}
	if _, ok := InstanceFromInbound(ib); ok {
		t.Fatal("unparseable settings must be rejected")
	}
}

func TestInstanceFromInboundEmptyWhenNoEnabledPeers(t *testing.T) {
	settings := mkInboundSettings(t, validServer(), []model.Client{
		{Email: "a@x", Enable: false, PublicKey: "pubA", AllowedIPs: []string{"10.8.1.2/32"}},
	})
	ib := &model.Inbound{Id: 1, Protocol: model.AmneziaWG, Settings: settings}
	if _, ok := InstanceFromInbound(ib); ok {
		t.Fatal("an inbound with zero enabled peers must be skipped, like mtproto.InstanceFromInbound")
	}
}

func TestServerAddress(t *testing.T) {
	cases := []struct {
		subnet string
		cidr   int
		want   string
	}{
		{"10.8.1.0", 24, "10.8.1.1/24"},
		{"10.8.1.0", 0, "10.8.1.1/24"}, // cidr <= 0 defaults to /24
		{"192.168.5.10", 32, "192.168.5.10/32"},
	}
	for _, c := range cases {
		if got := serverAddress(c.subnet, c.cidr); got != c.want {
			t.Errorf("serverAddress(%q, %d) = %q, want %q", c.subnet, c.cidr, got, c.want)
		}
	}
}

// fixedObfuscation is a deterministic Obfuscation20 for tests that compare
// two instances for equality — GenerateObfuscation20 is randomized per call
// by design (see its doc comment) and must never be used where the test
// expects two "identical" instances to actually match.
func fixedObfuscation() Obfuscation20 {
	return Obfuscation20{Jc: 4, Jmin: 40, Jmax: 100, S1: 30, S2: 90, S3: 20, S4: 10, H1: "10-2000", H2: "3000-5000", H3: "6000-8000", H4: "9000-11000", I1: "<r 64>"}
}

func baseInstance() Instance {
	return Instance{
		Id:            1,
		Tag:           "awg-1",
		InterfaceName: "awg1",
		ListenPort:    51820,
		PrivateKey:    "priv",
		PublicKey:     "pub",
		Address:       []string{"10.8.1.1/24"},
		Obfuscation:   fixedObfuscation(),
		Peers: []Peer{
			{Email: "a@x", PublicKey: "pubA", PresharedKey: "pskA", AllowedIPs: []string{"10.8.1.2/32"}},
			{Email: "b@x", PublicKey: "pubB", AllowedIPs: []string{"10.8.1.3/32"}},
		},
	}
}

func TestStructuralFingerprintStableAndSensitive(t *testing.T) {
	a := baseInstance()
	b := baseInstance()
	if a.structuralFingerprint() != b.structuralFingerprint() {
		t.Fatal("identical instances must produce the same structural fingerprint")
	}
	b.ListenPort = 51821
	if a.structuralFingerprint() == b.structuralFingerprint() {
		t.Fatal("a listen port change must change the structural fingerprint")
	}
	c := baseInstance()
	c.Peers[0].AllowedIPs = []string{"10.8.1.99/32"}
	if a.structuralFingerprint() != c.structuralFingerprint() {
		t.Fatal("a peer-only change must NOT change the structural fingerprint")
	}
}

func TestPeersFingerprintOrderIndependentButContentSensitive(t *testing.T) {
	a := baseInstance()
	reordered := baseInstance()
	reordered.Peers[0], reordered.Peers[1] = reordered.Peers[1], reordered.Peers[0]
	if a.peersFingerprint() != reordered.peersFingerprint() {
		t.Fatal("reordering peers must not change the peers fingerprint")
	}

	changed := baseInstance()
	changed.Peers[0].AllowedIPs = []string{"10.8.1.250/32"}
	if a.peersFingerprint() == changed.peersFingerprint() {
		t.Fatal("changing a peer's AllowedIPs must change the peers fingerprint")
	}

	fewer := baseInstance()
	fewer.Peers = fewer.Peers[:1]
	if a.peersFingerprint() == fewer.peersFingerprint() {
		t.Fatal("removing a peer must change the peers fingerprint")
	}
}

func TestEnsureActionFor(t *testing.T) {
	cases := []struct {
		name                              string
		up                                bool
		curStruct, curHostRules, curPeers string
		newStruct, newHostRules, newPeers string
		want                              ensureAction
	}{
		{"down forces restart even if identical", false, "s", "f", "p", "s", "f", "p", ensureRestart},
		{"structural change forces restart", true, "s1", "f", "p", "s2", "f", "p", ensureRestart},
		{"port-forward change forces restart", true, "s", "f1", "p", "s", "f2", "p", ensureRestart},
		{"route-through-xray change forces restart", true, "s", "route:false", "p", "s", "route:true", "p", ensureRestart},
		{"peers-only change reloads", true, "s", "f", "p1", "s", "f", "p2", ensureReload},
		{"identical up interface is a noop", true, "s", "f", "p", "s", "f", "p", ensureNoop},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ensureActionFor(c.up, c.curStruct, c.curHostRules, c.curPeers, c.newStruct, c.newHostRules, c.newPeers)
			if got != c.want {
				t.Errorf("ensureActionFor() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestHostRulesFingerprintCoversForwardedPortsAndRouting(t *testing.T) {
	a := baseInstance()
	b := baseInstance()
	if a.hostRulesFingerprint() != b.hostRulesFingerprint() {
		t.Fatal("identical instances must produce the same host-rules fingerprint")
	}
	if a.hostRulesFingerprint() != "" {
		t.Fatal("peers with no forwarded ports and no routing must produce an empty fingerprint")
	}

	forwarded := baseInstance()
	forwarded.Peers[0].ForwardedPorts = "80,443"
	if a.hostRulesFingerprint() == forwarded.hostRulesFingerprint() {
		t.Fatal("adding ForwardedPorts must change the host-rules fingerprint")
	}

	routed := baseInstance()
	routed.Peers[0].RouteThroughXray = true
	if a.hostRulesFingerprint() == routed.hostRulesFingerprint() {
		t.Fatal("enabling RouteThroughXray must change the host-rules fingerprint")
	}

	routedOtherTag := baseInstance()
	routedOtherTag.Peers[0].RouteThroughXray = true
	routedOtherTag.Peers[0].RouteOutboundTag = "warp"
	if routed.hostRulesFingerprint() == routedOtherTag.hostRulesFingerprint() {
		t.Fatal("changing RouteOutboundTag must change the host-rules fingerprint")
	}
}

func TestRouteEgressComment(t *testing.T) {
	if got := routeEgressComment(""); got != "awg-route" {
		t.Errorf("empty email must fall back to awg-route, got %q", got)
	}
	a := routeEgressComment("a@x")
	b := routeEgressComment("b@x")
	if a == b {
		t.Fatal("different emails must produce different comment tags")
	}
	if a != routeEgressComment("a@x") {
		t.Fatal("the same email must always produce the same comment tag")
	}
}

func TestRouteEgressLines(t *testing.T) {
	up := routeEgressLines("-A", "awg1", "10.8.1.2/32", "a@x")
	if len(up) != 2 {
		t.Fatalf("expected one TPROXY line per protocol (tcp+udp), got %d: %v", len(up), up)
	}
	for _, proto := range []string{"tcp", "udp"} {
		found := false
		for _, l := range up {
			if !strings.Contains(l, "-p "+proto) {
				continue
			}
			found = true
			if !strings.Contains(l, "-i awg1") || !strings.Contains(l, "-s 10.8.1.2") ||
				!strings.Contains(l, fmt.Sprintf("--on-port %d", EgressPort)) ||
				!strings.Contains(l, "--on-ip 127.0.0.1") ||
				!strings.Contains(l, fmt.Sprintf("--tproxy-mark %#x/%#x", EgressFwmark, EgressFwmark)) ||
				!strings.Contains(l, "-A PREROUTING") {
				t.Errorf("%s line missing expected fields: %s", proto, l)
			}
		}
		if !found {
			t.Errorf("missing a %s TPROXY line in %v", proto, up)
		}
	}
	if strings.Contains(up[0], "10.8.1.2/32") {
		t.Errorf("expected the /32 mask stripped from the source match, got %s", up[0])
	}

	down := routeEgressLines("-D", "awg1", "10.8.1.2/32", "a@x")
	if len(down) != 2 || !strings.Contains(down[0], "-D PREROUTING") {
		t.Fatalf("expected symmetric -D lines, got %v", down)
	}

	if got := routeEgressLines("-A", "awg1", "", "a@x"); got != nil {
		t.Errorf("empty clientIP must yield no lines, got %v", got)
	}
}

func TestDefaultPostUpDownEmitsTproxyOnlyForRoutedPeers(t *testing.T) {
	inst := baseInstance()
	inst.Peers[0].RouteThroughXray = true
	inst.Peers[0].RouteOutboundTag = "warp"
	up, down := defaultPostUpDown(inst, "eth0")

	if !strings.Contains(up, "TPROXY") || !strings.Contains(up, fmt.Sprintf("--on-port %d", EgressPort)) {
		t.Errorf("expected a TPROXY rule for the routed peer in PostUp, got:\n%s", up)
	}
	if !strings.Contains(down, "TPROXY") {
		t.Errorf("expected a matching TPROXY removal in PostDown, got:\n%s", down)
	}
	if !strings.Contains(up, fmt.Sprintf("ip rule add fwmark %#x", EgressFwmark)) {
		t.Errorf("expected the shared policy route to be added once in PostUp, got:\n%s", up)
	}
	if strings.Contains(down, "ip rule") || strings.Contains(down, "ip route") {
		t.Error("the shared policy route must never be removed in PostDown -- other instances may still need it")
	}
	// Peer b@x has no routing enabled: only the one routed peer's tcp+udp
	// pair should appear.
	if got := strings.Count(up, "TPROXY"); got != 2 {
		t.Errorf("expected exactly 2 TPROXY lines (tcp+udp for the one routed peer), got %d in:\n%s", got, up)
	}

	none := baseInstance() // no peer opts in
	upNone, _ := defaultPostUpDown(none, "eth0")
	if strings.Contains(upNone, "TPROXY") || strings.Contains(upNone, "ip rule add fwmark") {
		t.Errorf("an instance with no routed peers must not emit any TPROXY/policy-route lines, got:\n%s", upNone)
	}
}

func TestGenerateServerConfigContainsExpectedLines(t *testing.T) {
	inst := baseInstance()
	inst.ExternalInterface = "eth0"
	cfg := generateServerConfig(inst)

	want := []string{
		"[Interface]",
		"PrivateKey = priv",
		"Address = 10.8.1.1/24",
		"ListenPort = 51820",
		"[Peer]",
		"PublicKey = pubA",
		"PresharedKey = pskA",
		"AllowedIPs = 10.8.1.2/32",
		"PublicKey = pubB",
		"AllowedIPs = 10.8.1.3/32",
		"MASQUERADE",
	}
	for _, w := range want {
		if !strings.Contains(cfg, w) {
			t.Errorf("generated config missing %q\n---\n%s", w, cfg)
		}
	}
	// The second peer has no PresharedKey — its block must not emit the field at all.
	if strings.Count(cfg, "PresharedKey") != 1 {
		t.Errorf("expected exactly one PresharedKey line (peer b@x has none), got config:\n%s", cfg)
	}
}

func TestWriteObfuscationDefaultsBlankH(t *testing.T) {
	var b strings.Builder
	writeObfuscation(&b, Obfuscation20{})
	out := b.String()
	for i, want := range []string{"H1 = 1", "H2 = 2", "H3 = 3", "H4 = 4"} {
		if !strings.Contains(out, want) {
			t.Errorf("blank H%d must fall back to default %q, got:\n%s", i+1, want, out)
		}
	}
	// S3/S4/I1 are zero-valued here and must be omitted entirely.
	if strings.Contains(out, "S3") || strings.Contains(out, "S4") || strings.Contains(out, "I1") {
		t.Errorf("zero-valued S3/S4/I1 must be omitted, got:\n%s", out)
	}
}

func TestInterfaceNameForID(t *testing.T) {
	if got := interfaceNameForID(42); got != "awg42" {
		t.Errorf("interfaceNameForID(42) = %q, want awg42", got)
	}
}
