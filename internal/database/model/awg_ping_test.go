package model

import (
	"net/netip"
	"testing"
)

func TestAWGPingPolicy(t *testing.T) {
	p := &AWGPingPolicy{Enabled: true, Allow: []string{"192.0.2.7/24"}, Deny: []string{"192.0.2.8"}}
	if err := p.Normalize(); err != nil {
		t.Fatal(err)
	}
	if p.Allow[0] != "192.0.2.0/24" || p.Deny[0] != "192.0.2.8/32" {
		t.Fatalf("noncanonical: %#v", p)
	}
	for _, c := range []struct {
		ip   string
		want bool
	}{{"192.0.2.7", true}, {"192.0.2.8", false}, {"198.51.100.1", false}, {"127.0.0.1", false}, {"224.0.0.1", false}, {"169.254.1.2", false}, {"255.255.255.255", false}} {
		if got := p.Allows(netip.MustParseAddr(c.ip)); got != c.want {
			t.Errorf("%s: got %v want %v", c.ip, got, c.want)
		}
	}
	p.Allow = nil
	if p.Allows(netip.MustParseAddr("192.0.2.7")) {
		t.Fatal("empty allow grants access")
	}
	for _, bad := range []string{"example.com", "::/0", "::ffff:192.0.2.1", "192.0.2.1/33", ""} {
		if err := (&AWGPingPolicy{Allow: []string{bad}}).Normalize(); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestAWGPingRecordRoundTrip(t *testing.T) {
	c := Client{Email: "synthetic", AWGPing: &AWGPingPolicy{Enabled: true, Allow: []string{"192.0.2.0/24"}}}
	got := c.ToRecord().ToClient()
	if got.AWGPing == nil || !got.AWGPing.Enabled || got.AWGPing.Allow[0] != "192.0.2.0/24" {
		t.Fatalf("policy lost: %#v", got.AWGPing)
	}
	got.AWGPing.Allow[0] = "198.51.100.0/24"
	if c.AWGPing.Allow[0] != "192.0.2.0/24" {
		t.Fatal("policy slices alias")
	}
}

func TestAWGPingRejectsRecognizableBroadcastAndReservedDestinations(t *testing.T) {
	p := &AWGPingPolicy{Enabled: true, Allow: []string{"0.0.0.0/0", "192.0.2.0/24"}}
	for _, ip := range []string{"192.0.2.255", "255.255.255.255", "240.0.0.1", "224.0.0.1", "0.1.2.3"} {
		if p.Allows(netip.MustParseAddr(ip)) {
			t.Fatalf("accepted non-unicast/broadcast %s", ip)
		}
	}
}
