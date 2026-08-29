package amneziawg

import (
	"encoding/json"
	"testing"

	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// validOutboundJSON is a fully valid amneziawg outbound settings payload.
func validOutboundJSON(t *testing.T) []byte {
	t.Helper()
	raw := map[string]any{
		"mtu":       1420,
		"secretKey": validPrivKey(t),
		"address":   []string{"10.8.0.2/32"},
		"jc":        4,
		"jmin":      40,
		"jmax":      100,
		"s1":        15,
		"s2":        80,
		"s3":        12,
		"s4":        12,
		"h1":        "100-800",
		"h2":        "900-1600",
		"h3":        "1700-2400",
		"h4":        "2500-3200",
		"peers": []map[string]any{{
			"publicKey":  validPubKey(t),
			"allowedIPs": []string{"0.0.0.0/0", "::/0"},
			"endpoint":   "203.0.113.7:51820",
			"keepAlive":  25,
		}},
	}
	bs, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	return bs
}

func validPubKey(t *testing.T) string {
	t.Helper()
	_, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatal(err)
	}
	return pub
}

func validPrivKey(t *testing.T) string {
	t.Helper()
	priv, _, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatal(err)
	}
	return priv
}

func TestInstanceFromOutbound_OK(t *testing.T) {
	wrapped, err := json.Marshal(map[string]any{
		"protocol": "amneziawg",
		"tag":      "awg-out-test",
		"settings": json.RawMessage(validOutboundJSON(t)),
	})
	if err != nil {
		t.Fatal(err)
	}
	inst, ok := InstanceFromOutbound("awg-out-test", wrapped)
	if !ok {
		t.Fatal("InstanceFromOutbound returned false for a valid outbound")
	}
	if inst.Tag != "awg-out-test" {
		t.Fatalf("Tag = %q, want awg-out-test", inst.Tag)
	}
	if len(inst.Peers) != 1 {
		t.Fatalf("len(Peers) = %d, want 1", len(inst.Peers))
	}
	p := inst.Peers[0]
	if p.Endpoint != "203.0.113.7:51820" {
		t.Fatalf("Endpoint = %q", p.Endpoint)
	}
	if p.KeepAlive != 25 {
		t.Fatalf("KeepAlive = %d, want 25", p.KeepAlive)
	}
	if len(p.AllowedIPs) != 2 {
		t.Fatalf("AllowedIPs = %v", p.AllowedIPs)
	}
	if inst.MTU != 1420 {
		t.Fatalf("MTU = %d, want 1420", inst.MTU)
	}
	if inst.Obfuscation.Jc != 4 || inst.Obfuscation.S1 != 15 {
		t.Fatalf("Obfuscation not carried: %+v", inst.Obfuscation)
	}
}

func TestInstanceFromOutbound_RejectsIncompletePeer(t *testing.T) {
	m := validOutboundMapT(t)
	m["address"] = []any{}
	bs, _ := json.Marshal(m)
	wrapped, _ := json.Marshal(map[string]any{"protocol": "amneziawg", "settings": json.RawMessage(bs)})
	if _, ok := InstanceFromOutbound("t", wrapped); ok {
		t.Fatal("expected false when address list is empty")
	}

	m2 := validOutboundMapT(t)
	m2["peers"].([]any)[0].(map[string]any)["endpoint"] = ""
	bs2, _ := json.Marshal(m2)
	wrapped2, _ := json.Marshal(map[string]any{"protocol": "amneziawg", "settings": json.RawMessage(bs2)})
	// The only peer is incomplete -> skipped -> zero usable peers -> false,
	// mirroring InstanceFromInbound's "nothing to serve" contract.
	if _, ok := InstanceFromOutbound("t", wrapped2); ok {
		t.Fatal("outbound whose only peer lacks an endpoint must be unusable")
	}

	// With a second, complete peer the instance stays usable and only the
	// broken entry disappears.
	m3 := validOutboundMapT(t)
	brokenPeer := validOutboundMapT(t)["peers"].([]any)[0].(map[string]any)
	brokenPeer["endpoint"] = ""
	m3["peers"] = []any{brokenPeer, validOutboundMapT(t)["peers"].([]any)[0]}
	bs3, _ := json.Marshal(m3)
	wrapped3, _ := json.Marshal(map[string]any{"protocol": "amneziawg", "settings": json.RawMessage(bs3)})
	inst, ok := InstanceFromOutbound("t", wrapped3)
	if !ok {
		t.Fatal("one good peer should keep the outbound usable")
	}
	if len(inst.Peers) != 1 {
		t.Fatalf("broken peer must be dropped; got %d peers", len(inst.Peers))
	}
}

func TestValidateAmneziaWGOutbound_AcceptsValidAndRejectsBroken(t *testing.T) {
	if err := ValidateAmneziaWGOutbound("t", wrapOutboundSettings(validOutboundJSON(t))); err != nil {
		t.Fatalf("valid outbound rejected: %v", err)
	}

	cases := []struct {
		name   string
		breakF func(m map[string]any)
	}{
		{"bad endpoint no port", func(m map[string]any) { peer(m)["endpoint"] = "203.0.113.7" }},
		{"endpoint control char", func(m map[string]any) { peer(m)["endpoint"] = "host:51820\nPostUp=x" }},
		{"empty allowedIPs", func(m map[string]any) { peer(m)["allowedIPs"] = []string{} }},
		{"no peers", func(m map[string]any) { m["peers"] = []any{} }},
		{"bad allowedIP", func(m map[string]any) { peer(m)["allowedIPs"] = []string{"not-a-prefix"} }},
		{"no address", func(m map[string]any) { m["address"] = []any{} }},
		{"bad jc/jmin order", func(m map[string]any) { m["jmin"] = 200; m["jmax"] = 100 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := validOutboundMapT(t)
			tc.breakF(m)
			bs, _ := json.Marshal(m)
			if err := ValidateAmneziaWGOutbound("t", wrapOutboundSettings(bs)); err == nil {
				t.Fatalf("%s: expected error, got nil", tc.name)
			}
		})
	}
}

// wrapOutboundSettings embeds a settings payload the way the template stores
// it: as the nested "settings" of an amneziawg outbound row.
func wrapOutboundSettings(settings json.RawMessage) []byte {
	bs, err := json.Marshal(map[string]any{
		"protocol": "amneziawg",
		"tag":      "t",
		"settings": settings,
	})
	if err != nil {
		panic(err)
	}
	return bs
}

func peer(m map[string]any) map[string]any {
	return m["peers"].([]any)[0].(map[string]any)
}

func validOutboundMapT(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(validOutboundJSON(t), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestIsAmneziaWGOutbound(t *testing.T) {
	yes := []byte(`{"protocol":"amneziawg","tag":"x"}`)
	if !IsAmneziaWGOutbound(yes) {
		t.Fatal("amneziawg protocol not detected")
	}
	no := []byte(`{"protocol":"freedom","tag":"x"}`)
	if IsAmneziaWGOutbound(no) {
		t.Fatal("freedom misdetected as amneziawg")
	}
	if IsAmneziaWGOutbound([]byte(`{broken`)) {
		t.Fatal("garbage misdetected as amneziawg")
	}
}

// A blank line terminates IpcSetOperation, silently truncating the peer set;
// validation rejects a trailing newline, parsing normalizes it away.
func TestValidateAmneziaWGOutbound_RejectsAllowedIPWithNewline(t *testing.T) {
	m := validOutboundMapT(t)
	peer(m)["allowedIPs"] = []string{"0.0.0.0/0\n", "::/0"}
	bs, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateAmneziaWGOutbound("t", wrapOutboundSettings(bs)); err == nil {
		t.Fatal("allowedIP with trailing newline: expected error, got nil")
	}
}

func TestInstanceFromOutbound_NormalizesAllowedIPs(t *testing.T) {
	m := validOutboundMapT(t)
	peer(m)["allowedIPs"] = []string{" 0.0.0.0/0\n", "::/0"}
	bs, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	inst, ok := InstanceFromOutbound("t", wrapOutboundSettings(bs))
	if !ok {
		t.Fatal("InstanceFromOutbound returned false for trimmable allowedIPs")
	}
	got := inst.Peers[0].AllowedIPs
	want := []string{"0.0.0.0/0", "::/0"}
	if len(got) != len(want) {
		t.Fatalf("AllowedIPs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("AllowedIPs[%d] = %q, want %q (newline must not survive)", i, got[i], want[i])
		}
	}
}

func TestInstanceFromOutbound_RejectsUnparseableAllowedIP(t *testing.T) {
	m := validOutboundMapT(t)
	peer(m)["allowedIPs"] = []string{"not-a-prefix"}
	bs, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := InstanceFromOutbound("t", wrapOutboundSettings(bs)); ok {
		t.Fatal("unparseable allowedIP must make InstanceFromOutbound return false")
	}
}

func TestValidateAmneziaWGOutbound_RejectsControlCharInIParams(t *testing.T) {
	for _, field := range []string{"i1", "i2", "i3", "i4", "i5"} {
		t.Run(field, func(t *testing.T) {
			m := validOutboundMapT(t)
			m[field] = "<r 64>\nPostUp=x"
			bs, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateAmneziaWGOutbound("t", wrapOutboundSettings(bs)); err == nil {
				t.Fatalf("%s with embedded newline: expected error, got nil", field)
			}
		})
	}
}

func TestValidateAmneziaWGOutbound_RejectsEmptyTag(t *testing.T) {
	raw := []byte(`{"protocol":"amneziawg","tag":"","settings":{"secretKey":"x"}}`)
	for _, tag := range []string{"", "   "} {
		if err := ValidateAmneziaWGOutbound(tag, raw); err == nil {
			t.Fatalf("tag %q accepted", tag)
		}
	}
}

func TestValidateAmneziaWGOutbound_DNSField(t *testing.T) {
	valid := map[string]string{
		"":                          "",
		"1.1.1.1":                   "1.1.1.1:53",
		"8.8.8.8:53":                "8.8.8.8:53",
		"2606:4700:4700::1111":      "[2606:4700:4700::1111]:53",
		"[2606:4700:4700::1111]:53": "[2606:4700:4700::1111]:53",
	}
	for d, expected := range valid {
		m := validOutboundMapT(t)
		if d != "" {
			m["dns"] = d
		}
		bs, _ := json.Marshal(m)
		if err := ValidateAmneziaWGOutbound("t", wrapOutboundSettings(bs)); err != nil {
			t.Fatalf("valid dns %q rejected: %v", d, err)
		}
		inst, ok := InstanceFromOutbound("t", wrapOutboundSettings(bs))
		if !ok || inst.DNS != expected {
			t.Fatalf("InstanceFromOutbound dns=%q, want %q", inst.DNS, expected)
		}
	}
	invalid := []string{"not-an-ip", "1.1.1.1\nPostUp=x", "999.999.999.999"}
	for _, d := range invalid {
		m := validOutboundMapT(t)
		m["dns"] = d
		bs, _ := json.Marshal(m)
		if err := ValidateAmneziaWGOutbound("t", wrapOutboundSettings(bs)); err == nil {
			t.Fatalf("invalid dns %q accepted", d)
		}
	}
}
