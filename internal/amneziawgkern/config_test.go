package amneziawgkern

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
)

func testInstance() amneziawg.Instance {
	return amneziawg.Instance{
		Id:            1,
		InterfaceName: "awg1",
		ListenPort:    31884,
		PrivateKey:    "cPrivateKeyBase64ValueForTestingOnlyPadding0=",
		Address:       []string{"10.8.1.1/24"},
		MTU:           1380,
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

// TestRenderEmitsInterfaceAndPeers locks the field set awg-quick has to see: a missing ListenPort or
// MTU silently changes which port clients reach and whether full-size packets fragment.
func TestRenderEmitsInterfaceAndPeers(t *testing.T) {
	t.Parallel()

	got, err := Render(testInstance(), "eth0", false)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{
		"[Interface]",
		"PrivateKey = cPrivateKeyBase64ValueForTestingOnlyPadding0=",
		"Address = 10.8.1.1/24",
		"ListenPort = 31884",
		"MTU = 1380",
		"Jc = 3", "Jmin = 62", "Jmax = 284",
		"S1 = 15", "S2 = 89", "S3 = 40", "S4 = 12",
		"HeaderProtectionKey = hpkBase64ValueForTestingOnlyPaddingXXXXXXXX=",
		"ContentPaddingAddition = 14-47",
		"RekeyAfterTime = 107-133",
		"RandomTrailers = on",
		"DisableCookies = on",
		"PublicKey = peerAPublicKeyBase64ValueForTestsPadding000=",
		"AllowedIPs = 10.8.1.2/32",
		"PresharedKey = pskBase64ValueForTestingOnlyPaddingXXXXXXX=",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered config missing %q\n--- got ---\n%s", want, got)
		}
	}
	if n := strings.Count(got, "[Peer]"); n != 2 {
		t.Errorf("got %d [Peer] sections, want 2", n)
	}
}

// TestRenderFillsHeaderDefaults is the interop guard: the panel's own client emitter writes H1..H4 =
// 1..4 when the stored values are blank, so a server config that omits them or writes an empty value
func TestRenderFillsHeaderDefaults(t *testing.T) {
	t.Parallel()

	got, err := Render(testInstance(), "eth0", false)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for i, want := range []string{"H1 = 1", "H2 = 2", "H3 = 3", "H4 = 4"} {
		if !strings.Contains(got, want) {
			t.Errorf("blank H%d must fall back to %q\n--- got ---\n%s", i+1, want, got)
		}
	}
}

// TestRenderMasqueradesSubnetNotHostAddress catches the easy mistake of masquerading 10.8.1.1/24
// verbatim: nftables would then match only the server's own tunnel address and no peer traffic would
func TestRenderMasqueradesSubnetNotHostAddress(t *testing.T) {
	t.Parallel()

	got, err := Render(testInstance(), "eth0", false)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(got, "ip saddr 10.8.1.0/24 oifname \"eth0\" masquerade") {
		t.Errorf("expected masquerade on the tunnel subnet\n--- got ---\n%s", got)
	}
	if strings.Contains(got, "ip saddr 10.8.1.1/24") {
		t.Errorf("masquerade must not match the server's own address only\n--- got ---\n%s", got)
	}
	if !strings.Contains(got, "net.ipv4.ip_forward=1") {
		t.Errorf("NAT mode must enable forwarding\n--- got ---\n%s", got)
	}
}

// TestRenderSkipsNATWhenRoutingThroughXray preserves the renderer's no-NAT result.
// Preflight rejects this setting before a kernel interface can start.
func TestRenderSkipsNATWhenRoutingThroughXray(t *testing.T) {
	t.Parallel()

	got, err := Render(testInstance(), "eth0", true)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(got, "masquerade") || strings.Contains(got, "PostUp") {
		t.Errorf("routeThroughXray must not emit NAT rules\n--- got ---\n%s", got)
	}
}

// TestNATTableIsPerInbound: a shared table name would make one inbound's
// PostDown delete every other inbound's NAT.
func TestNATTableIsPerInbound(t *testing.T) {
	t.Parallel()

	a := testInstance()
	b := testInstance()
	b.Id = 7

	ra, err := Render(a, "eth0", false)
	if err != nil {
		t.Fatalf("Render(a): %v", err)
	}
	rb, err := Render(b, "eth0", false)
	if err != nil {
		t.Fatalf("Render(b): %v", err)
	}
	if !strings.Contains(ra, "nft delete table ip awgnat1") {
		t.Errorf("inbound 1 should tear down its own table\n%s", ra)
	}
	if !strings.Contains(rb, "nft delete table ip awgnat7") {
		t.Errorf("inbound 7 should tear down its own table\n%s", rb)
	}
}

func TestRenderRejectsIncompleteInstance(t *testing.T) {
	t.Parallel()

	noKey := testInstance()
	noKey.PrivateKey = ""
	if _, err := Render(noKey, "eth0", false); err == nil {
		t.Error("Render must reject an instance with no private key")
	}

	noAddr := testInstance()
	noAddr.Address = nil
	if _, err := Render(noAddr, "eth0", false); err == nil {
		t.Error("Render must reject an instance with no address")
	}

	v6Only := testInstance()
	v6Only.Address = []string{"fd00::1/64"}
	if _, err := Render(v6Only, "eth0", false); err == nil {
		t.Error("Render must reject NAT mode with no IPv4 subnet to translate")
	}
}

func TestRenderRejectsPinnedListenAddress(t *testing.T) {
	inst := testInstance()
	inst.Listen = "203.0.113.10"
	if _, err := Render(inst, "eth0", false); err == nil || !strings.Contains(err.Error(), "cannot bind to listen address") {
		t.Fatalf("Render must refuse to widen a pinned listen address to a wildcard kernel bind, got %v", err)
	}
}

func TestConfPathUsesInterfaceName(t *testing.T) {
	t.Parallel()

	if got, want := ConfPath("awg1"), "/etc/amnezia/amneziawg/awg1.conf"; got != want {
		t.Errorf("ConfPath = %q, want %q", got, want)
	}
}

// TestDetectDefaultInterfaceSkipsVirtual only asserts the naming contract -- the real host's interface
// list is not something a test can arrange -- but a tunnel or bridge device chosen as the NAT egress
func TestDetectDefaultInterfaceSkipsVirtual(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"awg1", "wg0", "docker0", "br-abc123", "veth9f2"} {
		if !hasSkippedPrefix(name) {
			t.Errorf("%q must never be picked as the NAT egress interface", name)
		}
	}
	for _, name := range []string{"eth0", "ens3", "enp1s0"} {
		if hasSkippedPrefix(name) {
			t.Errorf("%q is a normal host NIC and must remain eligible", name)
		}
	}
}

// TestRenderFallsBackToS4AwareMTU: an inbound with no explicit MTU must not be left on the netdev
// default, which knows nothing about the S4 junk prepended to every transport packet.
func TestRenderFallsBackToS4AwareMTU(t *testing.T) {
	t.Parallel()

	inst := testInstance()
	inst.MTU = 0
	inst.Obfuscation.S4 = 27

	got, err := Render(inst, "eth0", false)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "MTU = 1393" // 1420 - 27
	if !strings.Contains(got, want) {
		t.Errorf("expected %q for S4=27 with no explicit MTU\n--- got ---\n%s", want, got)
	}
	if strings.Contains(got, "MTU = 1420") {
		t.Error("the plain 1420 default ignores S4 and fragments full-size packets")
	}
}

// TestRenderKeepsExplicitMTU: the admin's value wins over the default.
func TestRenderKeepsExplicitMTU(t *testing.T) {
	t.Parallel()

	got, err := Render(testInstance(), "eth0", false)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(got, "MTU = 1380") {
		t.Errorf("explicit MTU must be preserved\n--- got ---\n%s", got)
	}
}
