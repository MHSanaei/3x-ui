package amneziawgnet

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// TestPeerIndexLookupMatchesDeviceAllowedIPs catches Lookup attributing a
// prefix two peers both carry to a peer the device accepts no such source from.
func TestPeerIndexLookupMatchesDeviceAllowedIPs(t *testing.T) {
	serverPriv, serverPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate server keypair: %v", err)
	}
	var peers []amneziawg.Peer
	for _, p := range []amneziawg.Peer{
		{Email: "first@example.com", AllowedIPs: []string{"10.202.0.2/32", "fd00:202::2/128", "0.0.0.0/0", "::/0"}},
		{Email: "last@example.com", AllowedIPs: []string{"10.202.0.3/32", "fd00:202::3/128", "0.0.0.0/0", "::/0"}},
	} {
		if _, p.PublicKey, err = wireguard.GenerateWireguardKeypair(); err != nil {
			t.Fatalf("generate peer keypair: %v", err)
		}
		peers = append(peers, p)
	}

	dev, err := NewDevice(amneziawg.Instance{
		Id:         1,
		PrivateKey: serverPriv,
		PublicKey:  serverPub,
		Address:    []string{"10.202.0.1/24"},
		MTU:        1420,
		Peers:      peers,
	}, DeviceOptions{})
	if err != nil {
		t.Fatalf("NewDevice: %v", err)
	}
	defer dev.Close()

	dump, err := dev.IpcGet()
	if err != nil {
		t.Fatalf("IpcGet: %v", err)
	}
	_, states := parseUAPIDump(dump)

	idx := NewPeerIndex(peers)
	for _, tc := range []struct{ addr, prefix string }{
		{"10.99.0.1", "0.0.0.0/0"},
		{"fd00:99::1", "::/0"},
		{"10.202.0.2", "10.202.0.2/32"},
		{"10.202.0.3", "10.202.0.3/32"},
		{"fd00:202::2", "fd00:202::2/128"},
		{"fd00:202::3", "fd00:202::3/128"},
	} {
		t.Run(tc.addr, func(t *testing.T) {
			got, ok := idx.Lookup(netip.MustParseAddr(tc.addr))
			if !ok {
				t.Fatalf("Lookup(%s) found no peer", tc.addr)
			}
			gotHex, err := wireguard.KeyToHex(got.PublicKey)
			if err != nil {
				t.Fatalf("KeyToHex: %v", err)
			}
			if !slices.Contains(states[gotHex].allowedIPs, tc.prefix) {
				t.Errorf("Lookup(%s) = %s, but the device holds %s for another peer", tc.addr, got.Email, tc.prefix)
			}
		})
	}
}
