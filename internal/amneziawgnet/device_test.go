package amneziawgnet

import (
	"context"
	"fmt"
	"io"
	"net/netip"
	"testing"
	"time"

	awgconn "github.com/amnezia-vpn/amneziawg-go/v3/conn"
	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	"github.com/amnezia-vpn/amneziawg-go/v3/tun/netstack"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// TestNewDeviceHandshakeForwarderAndIdentity is Phase 1's real end-to-end
// proof, not just a compile check: a genuine amneziawg-go client (via that
// project's own tun/netstack.CreateNetTUN -- the client side doesn't need a
// forwarder or peer-identity resolution, only this package's server side
// does) completes a real 3-way handshake against a Device built by
// NewDevice, dials a destination that was never configured anywhere on the
// server, and the test verifies AttachTCPForwarder recovers that exact
// destination *and* PeerIndex.Lookup resolves the connection's source back
// to the right peer's Email -- Phase 1a/1b/1c working together, the same
// mechanism Phase 0's throwaway spike validated, now as a real, repo-owned,
// repeatable test instead of scratch code.
func TestNewDeviceHandshakeForwarderAndIdentity(t *testing.T) {
	serverPriv, serverPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate server keypair: %v", err)
	}
	clientPriv, clientPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate client keypair: %v", err)
	}

	const listenPort = 58712 // fixed loopback test port, matches the validated Phase 0 spike approach
	const wantEmail = "test-peer@example.com"

	inst := amneziawg.Instance{
		Id:            1,
		InterfaceName: "awgtest1",
		ListenPort:    listenPort,
		PrivateKey:    serverPriv,
		PublicKey:     serverPub,
		Address:       []string{"10.201.0.1/24"},
		MTU:           1420,
		Obfuscation: amneziawg.Obfuscation20{
			Jc: 4, Jmin: 40, Jmax: 70,
			S1: 20, S2: 30, S3: 20, S4: 20,
		},
		Peers: []amneziawg.Peer{{
			Email:      wantEmail,
			PublicKey:  clientPub,
			AllowedIPs: []string{"10.201.0.2/32"},
		}},
	}

	dev, err := NewDevice(inst, DeviceOptions{})
	if err != nil {
		t.Fatalf("NewDevice: %v", err)
	}
	defer dev.Close()

	idx := NewPeerIndex(inst.Peers)

	type recovered struct {
		email string
		ok    bool
		dest  netip.AddrPort
	}
	got := make(chan recovered, 1)

	// Never configured anywhere server-side: the forwarder must recover it
	// purely from the decapsulated packet, not from any routing table.
	wantDest := netip.MustParseAddrPort("10.201.9.9:9999")

	AttachTCPForwarder(dev.Stack, func(conn *gonet.TCPConn, dest netip.AddrPort) {
		defer conn.Close()
		srcAddrPort, parseErr := netip.ParseAddrPort(conn.RemoteAddr().String())
		var peer amneziawg.Peer
		var ok bool
		if parseErr == nil {
			peer, ok = idx.Lookup(srcAddrPort.Addr().Unmap())
		}
		got <- recovered{email: peer.Email, ok: ok, dest: dest}
		io.Copy(io.Discard, conn)
	})

	clientTun, clientNet, err := netstack.CreateNetTUN(
		[]netip.Addr{netip.MustParseAddr("10.201.0.2")},
		[]netip.Addr{netip.MustParseAddr("1.1.1.1")}, 1420)
	if err != nil {
		t.Fatalf("client CreateNetTUN: %v", err)
	}
	clientDev := device.NewDevice(clientTun, awgconn.NewDefaultBind(), device.NewLogger(device.LogLevelSilent, ""))
	defer clientDev.Close()

	clientPrivHex, err := wireguard.KeyToHex(clientPriv)
	if err != nil {
		t.Fatalf("client key to hex: %v", err)
	}
	serverPubHex, err := wireguard.KeyToHex(serverPub)
	if err != nil {
		t.Fatalf("server key to hex: %v", err)
	}
	// allowed_ip=0.0.0.0/0 on the client matches a real VPN client's own
	// config (route everything through the tunnel) -- it's also what makes
	// dialing an arbitrary, never-configured destination like wantDest
	// actually get routed to the server peer at all: a narrower AllowedIPs
	// here would make the client's own Device drop the packet as
	// non-matching before it ever reached the wire.
	clientConf := fmt.Sprintf(
		"private_key=%s\njc=4\njmin=40\njmax=70\ns1=20\ns2=30\ns3=20\ns4=20\npublic_key=%s\nendpoint=127.0.0.1:%d\nallowed_ip=0.0.0.0/0\n",
		clientPrivHex, serverPubHex, listenPort)
	if err := clientDev.IpcSet(clientConf); err != nil {
		t.Fatalf("client IpcSet: %v", err)
	}
	if err := clientDev.Up(); err != nil {
		t.Fatalf("client Up: %v", err)
	}

	// Retry the dial rather than guessing a fixed handshake delay: the
	// first attempts may race the handshake, later ones should succeed
	// once it completes.
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var lastErr error
	for {
		conn, dialErr := clientNet.DialContext(dialCtx, "tcp", wantDest.String())
		if dialErr == nil {
			conn.Close()
			break
		}
		lastErr = dialErr
		select {
		case <-dialCtx.Done():
			t.Fatalf("client dial never succeeded: %v", lastErr)
		case <-time.After(100 * time.Millisecond):
		}
	}

	select {
	case r := <-got:
		if !r.ok {
			t.Fatal("forwarder: peer identity lookup failed for the recovered connection")
		}
		if r.email != wantEmail {
			t.Errorf("resolved peer email = %q, want %q", r.email, wantEmail)
		}
		if r.dest != wantDest {
			t.Errorf("recovered destination = %v, want %v", r.dest, wantDest)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the forwarder to hand back the recovered connection")
	}
}
