package amneziawg

import (
	"encoding/json"
	"fmt"
	"slices"
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
		{"10.8.1.0", 0, "10.8.1.1/24"},  // cidr <= 0 defaults to /24
		{"10.8.1.5", 24, "10.8.1.1/24"}, // non-network base: must not collide with peer allocation starting at .2
		{"10.8.1.254", 24, "10.8.1.1/24"},
		{"192.168.5.10", 32, "192.168.5.10/32"}, // /32 has no host bits: used as-is
	}
	for _, c := range cases {
		if got := serverAddress(c.subnet, c.cidr); got != c.want {
			t.Errorf("serverAddress(%q, %d) = %q, want %q", c.subnet, c.cidr, got, c.want)
		}
	}
}

// fixedObfuscation is a deterministic Obfuscation31 for tests that compare
// two instances for equality — GenerateObfuscation31 is randomized per call
// by design (see its doc comment) and must never be used where the test
// expects two "identical" instances to actually match.
func fixedObfuscation() Obfuscation31 {
	return Obfuscation31{
		Jc: 4, Jmin: 40, Jmax: 100, S1: 30, S2: 90, S3: 20, S4: 10,
		H1: "10-2000", H2: "3000-5000", H3: "6000-8000", H4: "9000-11000",
		I1: "<r 64>", I2: "<r 80>",
		HeaderProtectionKey:    "MCPfRGcDGotJ6TcnIdDqsemj2cMIiGHnPUHM5ivXN18=",
		ContentPaddingAddition: "16-48",
		RekeyAfterTime:         "110-140",
		RekeyTimeout:           "4-8",
		RejectAfterTime:        "190-250",
		KeepaliveTimeout:       "9-15",
		MaxHandshakeAttempts:   "20-40",
		RandomTrailers:         true,
		DisableCookies:         true,
	}
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

	d := baseInstance()
	d.IPv6Enabled = true
	if a.structuralFingerprint() == d.structuralFingerprint() {
		t.Fatal("enabling IPv6 must change the structural fingerprint")
	}

	e := baseInstance()
	e.IPv6Enabled = true
	f := baseInstance()
	f.IPv6Enabled = true
	f.IPv6ExternalInterface = "eth1"
	if e.structuralFingerprint() == f.structuralFingerprint() {
		t.Fatal("changing IPv6ExternalInterface must change the structural fingerprint -- otherwise the edit is a complete no-op")
	}

	g := baseInstance()
	g.RouteThroughXray = true
	if a.structuralFingerprint() == g.structuralFingerprint() {
		t.Fatal("toggling RouteThroughXray must change the structural fingerprint -- it changes whether PostUp/PostDown contain any TPROXY rules at all")
	}

	h := baseInstance()
	h.Obfuscation.HeaderProtectionKey = "b0FpFDbtxb4W1YXsSq1NsPXbiTNDGeahIS1AmXjxx2c="
	if a.structuralFingerprint() == h.structuralFingerprint() {
		t.Fatal("changing HeaderProtectionKey must change the structural fingerprint -- the rendered config changed")
	}

	i := baseInstance()
	i.Obfuscation.RandomTrailers = false
	if a.structuralFingerprint() == i.structuralFingerprint() {
		t.Fatal("toggling RandomTrailers must change the structural fingerprint -- the rendered config changed")
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
		{"peer-ip change (its TPROXY rule) forces restart", true, "s", "ip:old", "p", "s", "ip:new", "p", ensureRestart},
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

func TestNextTrafficBaseline(t *testing.T) {
	prev := map[string]peerCounters{"pubA": {rx: 100, tx: 200}}

	if got := nextTrafficBaseline(ensureReload, prev); len(got) != 1 || got["pubA"] != prev["pubA"] {
		t.Errorf("a reload must preserve the previous baseline (syncconf never resets kernel counters), got %v", got)
	}
	if got := nextTrafficBaseline(ensureRestart, prev); len(got) != 0 {
		t.Errorf("a restart must reset the baseline to empty (awg-quick down+up zeroes kernel counters), got %v", got)
	}
}

func TestHostRulesFingerprintCoversForwardedPortsAndPeerIP(t *testing.T) {
	a := baseInstance()
	b := baseInstance()
	if a.hostRulesFingerprint() != b.hostRulesFingerprint() {
		t.Fatal("identical instances must produce the same host-rules fingerprint")
	}

	forwarded := baseInstance()
	forwarded.Peers[0].ForwardedPorts = "80,443"
	if a.hostRulesFingerprint() == forwarded.hostRulesFingerprint() {
		t.Fatal("adding ForwardedPorts must change the host-rules fingerprint")
	}

	fewer := baseInstance()
	fewer.Peers = fewer.Peers[:1]
	if a.hostRulesFingerprint() == fewer.hostRulesFingerprint() {
		t.Fatal("removing a peer must change the host-rules fingerprint -- one fewer peer entry exists regardless of what's tracked per peer")
	}

	// RouteThroughXray off (baseInstance's default): no TPROXY rule depends
	// on a peer's IPv4 address, so re-IPing one must NOT force a bounce --
	// this is the whole point of making the bridge opt-in: an instance that
	// never uses it keeps the syncconf fast path for a plain re-IP.
	reIPedNoRoute := baseInstance()
	reIPedNoRoute.Peers[0].AllowedIPs = []string{"10.8.1.250/32"}
	if a.hostRulesFingerprint() != reIPedNoRoute.hostRulesFingerprint() {
		t.Fatal("with RouteThroughXray off, changing a peer's IP must NOT change the host-rules fingerprint")
	}

	// A peer with forwarded ports has its DNAT rule keyed on its IPv4 address
	// too, regardless of RouteThroughXray -- re-IPing it must force a bounce,
	// or the old DNAT rule survives pointed at an address a different peer
	// can be handed next.
	forwardedReIPed := baseInstance()
	forwardedReIPed.Peers[0].ForwardedPorts = "80,443"
	forwardedBase := baseInstance()
	forwardedBase.Peers[0].ForwardedPorts = "80,443"
	forwardedReIPed.Peers[0].AllowedIPs = []string{"10.8.1.250/32"}
	if forwardedBase.hostRulesFingerprint() == forwardedReIPed.hostRulesFingerprint() {
		t.Fatal("with RouteThroughXray off but ForwardedPorts set, changing a peer's IP must change the host-rules fingerprint -- its DNAT rule is keyed on that IP")
	}

	// RouteThroughXray on: now the TPROXY rule really is keyed on the IP.
	routed := baseInstance()
	routed.RouteThroughXray = true
	routedReIPed := baseInstance()
	routedReIPed.RouteThroughXray = true
	routedReIPed.Peers[0].AllowedIPs = []string{"10.8.1.250/32"}
	if routed.hostRulesFingerprint() == routedReIPed.hostRulesFingerprint() {
		t.Fatal("with RouteThroughXray on, changing a peer's IP must change the host-rules fingerprint -- its TPROXY rule is keyed on that IP")
	}

	// IPv6Enabled off (baseInstance's default): no NDP-proxy entry depends
	// on a peer's IPv6 address either, so adding one must not force a bounce.
	ip6AddedNoIPv6 := baseInstance()
	ip6AddedNoIPv6.Peers[0].AllowedIPs = []string{"10.8.1.2/32", "fd86:ea04:1115::2/128"}
	if a.hostRulesFingerprint() != ip6AddedNoIPv6.hostRulesFingerprint() {
		t.Fatal("with IPv6Enabled off, adding a peer's IPv6 address must NOT change the host-rules fingerprint")
	}

	ip6Base := baseInstance()
	ip6Base.IPv6Enabled = true
	ip6Added := baseInstance()
	ip6Added.IPv6Enabled = true
	ip6Added.Peers[0].AllowedIPs = []string{"10.8.1.2/32", "fd86:ea04:1115::2/128"}
	if ip6Base.hostRulesFingerprint() == ip6Added.hostRulesFingerprint() {
		t.Fatal("with IPv6Enabled on, adding a peer's IPv6 address must change the host-rules fingerprint -- its NDP-proxy entry is keyed on it, and a change here must force the full bounce that (re-)runs PostUp")
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
	up := routeEgressLines("-A", "awg1", "10.8.1.2/32", "a@x", 63101)
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
				!strings.Contains(l, "--on-port 63101") ||
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

	down := routeEgressLines("-D", "awg1", "10.8.1.2/32", "a@x", 63101)
	if len(down) != 2 || !strings.Contains(down[0], "-D PREROUTING") {
		t.Fatalf("expected symmetric -D lines, got %v", down)
	}

	if got := routeEgressLines("-A", "awg1", "", "a@x", 63101); got != nil {
		t.Errorf("empty clientIP must yield no lines, got %v", got)
	}
}

func TestEgressPortForInbound(t *testing.T) {
	if got := EgressPortForInbound(1); got != EgressBasePort+1 {
		t.Errorf("EgressPortForInbound(1) = %d, want %d", got, EgressBasePort+1)
	}
	if EgressPortForInbound(1) == EgressPortForInbound(2) {
		t.Fatal("different inbound ids must derive different ports")
	}
}

func TestDefaultPostUpDownOmitsTproxyWhenRouteThroughXrayOff(t *testing.T) {
	inst := baseInstance() // RouteThroughXray defaults to false
	up, down := defaultPostUpDown(inst, "eth0")

	if strings.Contains(up, "TPROXY") || strings.Contains(up, "ip rule add fwmark") {
		t.Errorf("RouteThroughXray off must emit no TPROXY/policy-route lines in PostUp, got:\n%s", up)
	}
	if strings.Contains(down, "TPROXY") {
		t.Errorf("RouteThroughXray off must emit no TPROXY lines in PostDown, got:\n%s", down)
	}
}

func TestDefaultPostUpDownEmitsTproxyForEveryPeerWhenRouteThroughXrayOn(t *testing.T) {
	inst := baseInstance() // two peers, a@x and b@x
	inst.RouteThroughXray = true
	up, down := defaultPostUpDown(inst, "eth0")

	wantPort := fmt.Sprintf("--on-port %d", EgressPortForInbound(inst.Id))
	if !strings.Contains(up, "TPROXY") || !strings.Contains(up, wantPort) {
		t.Errorf("expected TPROXY rules targeting this instance's own bridge port in PostUp, got:\n%s", up)
	}
	if !strings.Contains(down, "TPROXY") {
		t.Errorf("expected matching TPROXY removals in PostDown, got:\n%s", down)
	}
	if !strings.Contains(up, fmt.Sprintf("ip rule add fwmark %#x", EgressFwmark)) {
		t.Errorf("expected the shared policy route to be added once in PostUp, got:\n%s", up)
	}
	// grep -c, not -q: -q's early exit can SIGPIPE "ip rule list" and, under
	// pipefail, make the existence check itself report failure even when the
	// rule was found -- which would re-run "ip rule add" and reintroduce the
	// exact duplicate this check exists to prevent.
	if wantCheck := fmt.Sprintf("ip rule list | grep -c 'fwmark %#x lookup %d' >/dev/null", EgressFwmark, EgressTable); !strings.Contains(up, wantCheck) {
		t.Errorf("expected a pipefail-safe existence check before 'ip rule add', so repeated bounces don't accumulate duplicate rules, got:\n%s", up)
	}
	if strings.Contains(down, "ip rule") || strings.Contains(down, "ip route") {
		t.Error("the shared policy route must never be removed in PostDown -- other instances may still need it")
	}
	// Both peers get TPROXY'd once opted in: 2 peers * 2 protocols.
	if got := strings.Count(up, "TPROXY"); got != 4 {
		t.Errorf("expected exactly 4 TPROXY lines (tcp+udp for each of the 2 peers), got %d in:\n%s", got, up)
	}

	none := Instance{Id: 2, InterfaceName: "awg2", RouteThroughXray: true} // no peers at all
	upNone, _ := defaultPostUpDown(none, "eth0")
	if strings.Contains(upNone, "TPROXY") || strings.Contains(upNone, "ip rule add fwmark") {
		t.Errorf("an instance with no peers must not emit any TPROXY/policy-route lines, got:\n%s", upNone)
	}
}

// TPROXY never rewrites a packet's own destination address -- only the
// routing decision changes -- so a default-deny INPUT chain that sanity-checks
// "is this destination actually local" (e.g. UFW's ufw-not-local, via
// addrtype --dst-type LOCAL) drops it before Xray's socket ever sees it, even
// though TPROXY's own mangle-table counters keep incrementing the whole time.
// This was a real, hard-to-diagnose production outage: RouteThroughXray
// looked fully configured (TPROXY rule present, Xray socket listening with
// IP_TRANSPARENT set) yet every peer's traffic silently vanished.
func TestDefaultPostUpDownAddsInputAcceptForFwmarkWhenRouteThroughXrayOn(t *testing.T) {
	inst := baseInstance() // two peers, a@x and b@x
	inst.RouteThroughXray = true
	up, down := defaultPostUpDown(inst, "eth0")

	wantCheck := fmt.Sprintf("iptables -C INPUT -m mark --mark %#x -j ACCEPT", EgressFwmark)
	wantInsert := fmt.Sprintf("iptables -I INPUT 1 -m mark --mark %#x -j ACCEPT", EgressFwmark)
	if !strings.Contains(up, wantCheck) || !strings.Contains(up, wantInsert) {
		t.Errorf("expected an idempotent INPUT accept for the shared fwmark in PostUp, got:\n%s", up)
	}
	if strings.Contains(down, "-m mark --mark") {
		t.Error("the shared INPUT accept must never be removed in PostDown -- other instances may still need it, same as the policy route")
	}

	none := Instance{Id: 2, InterfaceName: "awg2", RouteThroughXray: true} // no peers at all
	upNone, _ := defaultPostUpDown(none, "eth0")
	if strings.Contains(upNone, "-m mark --mark") {
		t.Errorf("an instance with no peers must not emit the INPUT accept either, got:\n%s", upNone)
	}
}

// PostDown is joined with "; " and run under `set -e`, so one command that
// fails because something already flushed the firewall state out from under
// the interface (a ufw/firewalld reload) would otherwise abort every command
// after it -- including the nat-table DNAT deletes a filter-table flush does
// NOT remove, which then survive and accumulate across bounces. Every
// teardown command must be best-effort; PostUp must not be.
func TestDefaultPostUpDownMakesEveryTeardownCommandBestEffort(t *testing.T) {
	inst := baseInstance() // two peers, a@x and b@x
	inst.RouteThroughXray = true
	inst.IPv6Enabled = true
	inst.Peers[0].ForwardedPorts = "80,443"
	up, down := defaultPostUpDown(inst, "eth0")

	for _, cmd := range strings.Split(down, "; ") {
		if !strings.HasSuffix(cmd, "|| true") {
			t.Errorf("every PostDown command must end with '|| true' so a flushed firewall doesn't abort the rest, got: %q", cmd)
		}
	}
	if strings.Contains(up, "|| true") {
		t.Error("PostUp must stay strict -- a real setup failure there should surface, not be silently swallowed")
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
	writeObfuscation(&b, Obfuscation31{})
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
	// Same for every 3.1 field: empty means the feature stays off the wire.
	for _, key := range []string{
		"I2", "I3", "I4", "I5",
		"HeaderProtectionKey", "ContentPaddingAddition",
		"RekeyAfterTime", "RekeyTimeout", "RejectAfterTime",
		"KeepaliveTimeout", "MaxHandshakeAttempts",
		"RandomTrailers", "DisableCookies",
	} {
		if strings.Contains(out, key) {
			t.Errorf("zero-valued %s must be omitted, got:\n%s", key, out)
		}
	}
}

func TestWriteObfuscationEmits31FieldsInOrder(t *testing.T) {
	var b strings.Builder
	writeObfuscation(&b, fixedObfuscation())
	out := b.String()

	want := []string{
		"Jc = 4",
		"H4 = 9000-11000",
		"I1 = <r 64>",
		"I2 = <r 80>",
		"HeaderProtectionKey = MCPfRGcDGotJ6TcnIdDqsemj2cMIiGHnPUHM5ivXN18=",
		"ContentPaddingAddition = 16-48",
		"RekeyAfterTime = 110-140",
		"RekeyTimeout = 4-8",
		"RejectAfterTime = 190-250",
		"KeepaliveTimeout = 9-15",
		"MaxHandshakeAttempts = 20-40",
		"RandomTrailers = on",
		"DisableCookies = on",
	}
	pos := -1
	for _, w := range want {
		i := strings.Index(out, w)
		if i < 0 {
			t.Fatalf("missing %q in:\n%s", w, out)
		}
		if i < pos {
			t.Fatalf("%q out of order (must follow the previous key) in:\n%s", w, out)
		}
		pos = i
	}
	// I3-I5 are empty in the fixture and must not appear.
	for _, absent := range []string{"I3", "I4", "I5"} {
		if strings.Contains(out, absent) {
			t.Errorf("empty %s must be omitted, got:\n%s", absent, out)
		}
	}
}

func TestParseAwgVersion(t *testing.T) {
	cases := []struct {
		out       string
		wantMajor int
		wantMinor int
		wantOK    bool
	}{
		{"wireguard-tools v3.1.20260812 - https://git.zx2c4.com/wireguard-tools/", 3, 1, true},
		{"wireguard-tools v1.0.20250706", 1, 0, true},
		{"3.0.20260730", 3, 0, true},
		{"awg: command not found", 0, 0, false},
		{"", 0, 0, false},
	}
	for _, c := range cases {
		major, minor, ok := parseAwgVersion(c.out)
		if ok != c.wantOK || major != c.wantMajor || minor != c.wantMinor {
			t.Errorf("parseAwgVersion(%q) = (%d, %d, %v), want (%d, %d, %v)", c.out, major, minor, ok, c.wantMajor, c.wantMinor, c.wantOK)
		}
	}
}

func TestInterfaceNameForID(t *testing.T) {
	if got := interfaceNameForID(42); got != "awg42" {
		t.Errorf("interfaceNameForID(42) = %q, want awg42", got)
	}
}

func TestInboundIDForInterfaceName(t *testing.T) {
	cases := []struct {
		name   string
		wantID int
		wantOK bool
	}{
		{"awg42", 42, true},
		{"awg0", 0, true},
		{"awg", 0, false},    // no digits after the prefix
		{"wg0", 0, false},    // wrong prefix entirely (plain WireGuard)
		{"awgabc", 0, false}, // non-numeric suffix
		{"awg-1", 0, false},  // Atoi rejects the leading '-' as part of TrimPrefix's leftover, but guard anyway
	}
	for _, c := range cases {
		id, ok := inboundIDForInterfaceName(c.name)
		if ok != c.wantOK || (ok && id != c.wantID) {
			t.Errorf("inboundIDForInterfaceName(%q) = (%d, %v), want (%d, %v)", c.name, id, ok, c.wantID, c.wantOK)
		}
	}
}

func TestOrphanedInterfaces(t *testing.T) {
	confFiles := []string{
		"awg1.conf",   // in want -> not orphaned
		"awg2.conf",   // not in want -> orphaned
		"awg3.conf",   // not in want -> orphaned
		"notes.txt",   // wrong suffix -> ignored
		"awgxyz.conf", // unparseable id -> ignored
	}
	want := map[int]struct{}{1: {}}

	got := orphanedInterfaces(confFiles, want)
	slices.Sort(got)
	if wantOut := []string{"awg2", "awg3"}; !slices.Equal(got, wantOut) {
		t.Errorf("orphanedInterfaces() = %v, want %v", got, wantOut)
	}
}

func TestOrphanedInterfacesEmptyWantOrphansEverything(t *testing.T) {
	got := orphanedInterfaces([]string{"awg5.conf"}, map[int]struct{}{})
	if want := []string{"awg5"}; !slices.Equal(got, want) {
		t.Errorf("orphanedInterfaces() = %v, want %v", got, want)
	}
}
