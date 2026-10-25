package amneziawgkern

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
)

func testInstance() amneziawg.Instance {
	return amneziawg.Instance{
		Id:         1,
		ListenPort: 31884,
		PrivateKey: "cPrivateKeyBase64ValueForTestingOnlyPadding0=",
		Address:    []string{"10.8.1.1/24"},
		MTU:        1380,
		Obfuscation: amneziawg.Obfuscation31{
			Jc: 3, Jmin: 62, Jmax: 284,
			S1: 15, S2: 89, S3: 40, S4: 12,
			HeaderProtectionKey:    "hpkBase64ValueForTestingOnlyPaddingXXXXXXXX=",
			ContentPaddingAddition: "14-47",
			RekeyAfterTime:         "107-133",
			RandomTrailers:         true,
			DisableCookies:         true,
		},
		Peers: []amneziawg.Peer{
			{Email: "alice", PublicKey: "peerAPublicKeyBase64ValueForTestsPadding000=", AllowedIPs: []string{"10.8.1.2/32"}},
			{Email: "bob", PublicKey: "peerBPublicKeyBase64ValueForTestsPadding000=", AllowedIPs: []string{"10.8.1.3/32"}, PresharedKey: "pskBase64ValueForTestingOnlyPaddingXXXXXXX="},
		},
	}
}

// TestRenderEmitsOnlySetconfKeys: `awg setconf` rejects wg-quick keys (Address,
// MTU, hooks) outright, and a hook line would reintroduce a root-shell sink.
func TestRenderEmitsOnlySetconfKeys(t *testing.T) {
	conf, err := Render(testInstance())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{
		"PrivateKey = cPrivateKeyBase64ValueForTestingOnlyPadding0=\n",
		"ListenPort = 31884\n",
		"Jc = 3\n", "S3 = 40\n", "S4 = 12\n",
		"HeaderProtectionKey = hpkBase64ValueForTestingOnlyPaddingXXXXXXXX=\n",
		"ContentPaddingAddition = 14-47\n",
		"RandomTrailers = on\n", "DisableCookies = on\n",
		"# alice\nPublicKey = peerAPublicKeyBase64ValueForTestsPadding000=\nAllowedIPs = 10.8.1.2/32\n",
		"PresharedKey = pskBase64ValueForTestingOnlyPaddingXXXXXXX=\n",
	} {
		if !strings.Contains(conf, want) {
			t.Errorf("rendered config lacks %q:\n%s", want, conf)
		}
	}
	for _, line := range strings.Split(conf, "\n") {
		key, _, _ := strings.Cut(line, " = ")
		switch key {
		case "Address", "MTU", "DNS", "Table", "PreUp", "PostUp", "PreDown", "PostDown", "SaveConfig":
			t.Errorf("rendered config carries wg-quick key %q", line)
		}
	}
	if !validOwnedConfig(conf, ownedInterfaceName(1)) {
		t.Errorf("ownership grammar rejects the panel's own rendered config:\n%s", conf)
	}
}

// TestRenderFillsHeaderDefaults: the 3.x module and tools reject a missing H1-H4,
// so a blank one must fall back to the vanilla message type it replaces.
func TestRenderFillsHeaderDefaults(t *testing.T) {
	inst := testInstance()
	inst.Obfuscation.H2 = "1000-2000"
	conf, err := Render(inst)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"H1 = 1\n", "H2 = 1000-2000\n", "H3 = 3\n", "H4 = 4\n"} {
		if !strings.Contains(conf, want) {
			t.Errorf("rendered config lacks %q", want)
		}
	}
}

// TestRenderRejectsControlCharacters: a newline in a client value would add a
// directive of the caller's choosing to the file the kernel is configured from.
func TestRenderRejectsControlCharacters(t *testing.T) {
	cases := map[string]func(*amneziawg.Instance){
		"peer key":    func(i *amneziawg.Instance) { i.Peers[0].PublicKey = "x\n[Interface]\nListenPort = 1" },
		"psk":         func(i *amneziawg.Instance) { i.Peers[1].PresharedKey = "x\rPublicKey = y" },
		"email":       func(i *amneziawg.Instance) { i.Peers[0].Email = "a\nAllowedIPs = 0.0.0.0/0" },
		"allowed ips": func(i *amneziawg.Instance) { i.Peers[0].AllowedIPs = []string{"10.8.1.2/32\nEndpoint = 1.2.3.4:1"} },
		"i1":          func(i *amneziawg.Instance) { i.Obfuscation.I1 = "<b 0x01>\nJc = 1" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			inst := testInstance()
			mutate(&inst)
			if _, err := Render(inst); err == nil || !strings.Contains(err.Error(), "control characters") {
				t.Fatalf("Render accepted a control character, err = %v", err)
			}
		})
	}
}

// TestBuildSpecMasqueradesSubnetNotHostAddress: matching the server's own
// 10.8.1.1 would NAT nothing the clients send.
func TestBuildSpecMasqueradesSubnetNotHostAddress(t *testing.T) {
	s, err := buildSpec(Desired{Instance: testInstance(), EgressInterface: "ens3"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.nat, "ip saddr 10.8.1.0/24 oifname \"ens3\" masquerade") {
		t.Fatalf("NAT transaction does not masquerade the tunnel subnet:\n%s", s.nat)
	}
	if !strings.HasPrefix(s.nat, "add table ip xui_awg1\ndelete table ip xui_awg1\n") {
		t.Fatalf("NAT transaction does not replace its own table atomically:\n%s", s.nat)
	}
	if s.forward != (forwardRules{tunnel: "xui-awg1", egress: "ens3"}) {
		t.Fatalf("forward rules = %+v, want tunnel xui-awg1 -> ens3", s.forward)
	}
}

// TestBuildSpecRouteThroughXrayOnlyDeletesNAT: a legacy routeThroughXray row
// must not leave a masquerade rule behind from an earlier save.
func TestBuildSpecRouteThroughXrayOnlyDeletesNAT(t *testing.T) {
	s, err := buildSpec(Desired{Instance: testInstance(), EgressInterface: "eth0", RouteThroughXray: true})
	if err != nil {
		t.Fatal(err)
	}
	if s.nat != "add table ip xui_awg1\ndelete table ip xui_awg1\n" {
		t.Fatalf("NAT transaction = %q, want only the table delete", s.nat)
	}
	if s.forward != (forwardRules{}) {
		t.Fatalf("forward rules = %+v, want none without NAT egress", s.forward)
	}
}

// TestBuildSpecRejectsUnsafeInputs: each lands in an argv or an nft
// transaction, and the kernel interface name has a hard 15-byte limit.
func TestBuildSpecRejectsUnsafeInputs(t *testing.T) {
	cases := map[string]Desired{
		"egress with quote":   {Instance: testInstance(), EgressInterface: `eth0" accept; #`},
		"pinned listen":       {Instance: func() amneziawg.Instance { i := testInstance(); i.Listen = "203.0.113.10"; return i }()},
		"ipv6 address":        {Instance: func() amneziawg.Instance { i := testInstance(); i.Address = []string{"fd00::1/64"}; return i }(), EgressInterface: "eth0"},
		"no address":          {Instance: func() amneziawg.Instance { i := testInstance(); i.Address = nil; return i }(), EgressInterface: "eth0"},
		"no private key":      {Instance: func() amneziawg.Instance { i := testInstance(); i.PrivateKey = ""; return i }(), EgressInterface: "eth0"},
		"name over 15 bytes":  {Instance: func() amneziawg.Instance { i := testInstance(); i.Id = 123456789; return i }(), EgressInterface: "eth0"},
		"non-positive id":     {Instance: func() amneziawg.Instance { i := testInstance(); i.Id = 0; return i }(), EgressInterface: "eth0"},
		"unparsable address":  {Instance: func() amneziawg.Instance { i := testInstance(); i.Address = []string{"10.8.1.1"}; return i }(), EgressInterface: "eth0"},
		"egress with newline": {Instance: testInstance(), EgressInterface: "eth0\n"},
	}
	for name, d := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := buildSpec(d); err == nil {
				t.Fatal("buildSpec accepted it")
			}
		})
	}
}

// TestBuildSpecMTU: an unset MTU must still be S4-aware, or every full-size
// packet fragments; an explicit one must win.
func TestBuildSpecMTU(t *testing.T) {
	inst := testInstance()
	inst.MTU = 0
	s, err := buildSpec(Desired{Instance: inst, EgressInterface: "eth0"})
	if err != nil {
		t.Fatal(err)
	}
	if s.mtu != 1408 {
		t.Fatalf("default MTU = %d, want 1408 (1420 minus S4 = 12)", s.mtu)
	}
	inst.MTU = 1280
	if s, _ = buildSpec(Desired{Instance: inst, EgressInterface: "eth0"}); s.mtu != 1280 {
		t.Fatalf("explicit MTU = %d, want 1280", s.mtu)
	}
}

// TestStructuralFingerprintIgnoresPeersOnly: a peer edit must sync in place,
// while key, port, address or MTU changes need a rebuild.
func TestStructuralFingerprintIgnoresPeersOnly(t *testing.T) {
	base, err := buildSpec(Desired{Instance: testInstance(), EgressInterface: "eth0"})
	if err != nil {
		t.Fatal(err)
	}
	peerEdit := testInstance()
	peerEdit.Peers = peerEdit.Peers[:1]
	if s, _ := buildSpec(Desired{Instance: peerEdit, EgressInterface: "eth0"}); s.structFP != base.structFP {
		t.Error("a peer edit changed the structural fingerprint")
	}
	for name, mutate := range map[string]func(*amneziawg.Instance){
		"private key": func(i *amneziawg.Instance) { i.PrivateKey = "dPrivateKeyBase64ValueForTestingOnlyPadding0=" },
		"listen port": func(i *amneziawg.Instance) { i.ListenPort++ },
		"address":     func(i *amneziawg.Instance) { i.Address = []string{"10.9.1.1/24"} },
		"mtu":         func(i *amneziawg.Instance) { i.MTU = 1300 },
		"obfuscation": func(i *amneziawg.Instance) { i.Obfuscation.Jc = 7 },
	} {
		inst := testInstance()
		mutate(&inst)
		if s, _ := buildSpec(Desired{Instance: inst, EgressInterface: "eth0"}); s.structFP == base.structFP {
			t.Errorf("%s change kept the structural fingerprint", name)
		}
	}
}

// TestOwnershipGrammarRefusesForeignConfigs: only a file Render wrote may be
// adopted, synced or torn down.
func TestOwnershipGrammarRefusesForeignConfigs(t *testing.T) {
	name := ownedInterfaceName(1)
	conf, err := Render(testInstance())
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"unmarked":          strings.TrimPrefix(conf, ownershipMarker(name)+"\n"),
		"other marker":      strings.Replace(conf, ownershipMarker(name), ownershipMarker(ownedInterfaceName(2)), 1),
		"hook line":         strings.Replace(conf, "ListenPort", "PostUp = id\nListenPort", 1),
		"wg-quick address":  strings.Replace(conf, "ListenPort", "Address = 10.8.1.1/24\nListenPort", 1),
		"peer endpoint":     conf + "Endpoint = 1.2.3.4:1\n",
		"interface comment": strings.Replace(conf, "[Interface]\n", "[Interface]\n# note\n", 1),
		"second interface":  conf + "[Interface]\n",
		"crlf":              strings.ReplaceAll(conf, "\n", "\r\n"),
	}
	for label, body := range cases {
		if validOwnedConfig(body, name) {
			t.Errorf("%s: ownership grammar accepted a config Render never writes", label)
		}
	}
}
