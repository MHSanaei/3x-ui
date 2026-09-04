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
