package amneziawgnet

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"

	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// clientDeviceTestInstance builds a minimal valid client instance with one
// peer and a non-zero keepalive -- the exact shape the outbound form seeds.
func clientDeviceTestInstance(t *testing.T) amneziawg.OutboundInstance {
	t.Helper()
	priv, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatal(err)
	}
	return amneziawg.OutboundInstance{
		Tag:        "awg-out-test",
		Address:    []string{"10.8.0.2/32"},
		MTU:        1420,
		PrivateKey: priv,
		Peers: []amneziawg.OutboundPeer{{
			PublicKey:  pub,
			AllowedIPs: []string{"0.0.0.0/0", "::/0"},
			Endpoint:   "203.0.113.7:51820",
			KeepAlive:  25,
		}},
	}
}

func TestBuildClientUAPIConfig_KeepAliveKeyIsValidUAPIPeerKey(t *testing.T) {
	inst := clientDeviceTestInstance(t)
	conf, err := buildClientUAPIConfig(inst, DeviceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := "persistent_keepalive_interval=25\n"
	if !strings.Contains(conf, want) {
		t.Fatalf("UAPI config missing %q:\n%s", want, conf)
	}
	if strings.Contains(conf, "persistent_keepalive_seconds") {
		t.Fatalf("UAPI config contains invalid peer key persistent_keepalive_seconds:\n%s", conf)
	}
}

func TestBuildClientUAPIConfig_ZeroKeepAliveOmitsLine(t *testing.T) {
	inst := clientDeviceTestInstance(t)
	inst.Peers[0].KeepAlive = 0
	conf, err := buildClientUAPIConfig(inst, DeviceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(conf, "persistent_keepalive") {
		t.Fatalf("zero KeepAlive must not emit a keepalive line:\n%s", conf)
	}
}

// amneziawg-go reads an absent UAPI line as "keep the current value", and
// ensureLocked reconfigures in place, so a cleared key must be sent as zero.
func TestBuildClientUAPIConfig_ClearedHeaderProtectionKeyIsSentAsZero(t *testing.T) {
	inst := clientDeviceTestInstance(t)
	inst.Obfuscation = amneziawg.Obfuscation31{S1: 20, S2: 20, S3: 20, S4: 20}

	key, err := wgutil.GenerateWireguardPSK()
	if err != nil {
		t.Fatal(err)
	}
	withKey, err := buildClientUAPIConfig(inst, DeviceOptions{HeaderProtectionKey: key})
	if err != nil {
		t.Fatal(err)
	}
	keyHex, err := wgutil.KeyToHex(key)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(withKey, "header_protection_key="+keyHex+"\n") {
		t.Fatalf("a set key must be emitted verbatim, got:\n%s", withKey)
	}

	cleared, err := buildClientUAPIConfig(inst, DeviceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	zero := "header_protection_key=" + strings.Repeat("0", 64) + "\n"
	if !strings.Contains(cleared, zero) {
		t.Fatalf("an unset key must be emitted as the all-zero key, got:\n%s", cleared)
	}
}

// With no explicit MTU the netstack is built from S4, so an S4-only edit must
// move the fingerprint or ensureLocked reconfigures in place and keeps the old.
func TestOutboundFingerprintTracksTheS4DerivedMTU(t *testing.T) {
	tests := []struct {
		name       string
		mtu        int
		wantChange bool
	}{
		{"derived MTU", 0, true},
		{"explicit MTU", 1420, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := clientDeviceTestInstance(t)
			inst.MTU = tt.mtu
			inst.Obfuscation.S4 = 12
			before := outboundFingerprint(inst)
			inst.Obfuscation.S4 = 28
			after := outboundFingerprint(inst)
			if changed := before != after; changed != tt.wantChange {
				t.Fatalf("fingerprint changed = %v, want %v (%q -> %q)", changed, tt.wantChange, before, after)
			}
		})
	}
}
