package amneziawg

import (
	"encoding/json"
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
		name                            string
		up                              bool
		curStruct, curPortFwd, curPeers string
		newStruct, newPortFwd, newPeers string
		want                            ensureAction
	}{
		{"down forces restart even if identical", false, "s", "f", "p", "s", "f", "p", ensureRestart},
		{"structural change forces restart", true, "s1", "f", "p", "s2", "f", "p", ensureRestart},
		{"port-forward change forces restart", true, "s", "f1", "p", "s", "f2", "p", ensureRestart},
		{"peers-only change reloads", true, "s", "f", "p1", "s", "f", "p2", ensureReload},
		{"identical up interface is a noop", true, "s", "f", "p", "s", "f", "p", ensureNoop},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ensureActionFor(c.up, c.curStruct, c.curPortFwd, c.curPeers, c.newStruct, c.newPortFwd, c.newPeers)
			if got != c.want {
				t.Errorf("ensureActionFor() = %v, want %v", got, c.want)
			}
		})
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
