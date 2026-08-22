package amneziawgnet

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	awgconn "github.com/amnezia-vpn/amneziawg-go/v3/conn"
	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	"github.com/amnezia-vpn/amneziawg-go/v3/tun/netstack"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// TestNewDeviceUDPHandlerAndReply is the UDP counterpart of
// TestNewDeviceHandshakeForwarderAndIdentity: this package's own udp.go was
// refactored from the Phase 0 spike's bake-the-dial-in version to a generic
// handler-plus-reply-injection design (see AttachUDPHandler/WriteUDPReply's
// doc comments), a real behavior change worth its own verification rather
// than assuming the port preserved correctness -- UDP was flagged as "the
// harder half" in the migration plan's own risk list, precisely because
// gVisor has no udp.NewForwarder and the reply path has to be constructed
// by hand.
func TestNewDeviceUDPHandlerAndReply(t *testing.T) {
	serverPriv, serverPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate server keypair: %v", err)
	}
	clientPriv, clientPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate client keypair: %v", err)
	}

	const listenPort = 58713 // distinct from the TCP test's port
	const wantEmail = "udp-test-peer@example.com"
	const echoPayload = "hello-from-client"

	inst := amneziawg.Instance{
		Id:            2,
		InterfaceName: "awgtest2",
		ListenPort:    listenPort,
		PrivateKey:    serverPriv,
		PublicKey:     serverPub,
		Address:       []string{"10.202.0.1/24"},
		MTU:           1420,
		Obfuscation: amneziawg.Obfuscation31{
			Jc: 4, Jmin: 40, Jmax: 70,
			S1: 20, S2: 30, S3: 20, S4: 20,
		},
		Peers: []amneziawg.Peer{{
			Email:      wantEmail,
			PublicKey:  clientPub,
			AllowedIPs: []string{"10.202.0.2/32"},
		}},
	}

	dev, err := newUnconfiguredDevice(inst, DeviceOptions{})
	if err != nil {
		t.Fatalf("newUnconfiguredDevice: %v", err)
	}
	defer dev.Close()

	idx := NewPeerIndex(inst.Peers)
	// Never configured anywhere server-side, same idea as the TCP test.
	wantDest := netip.MustParseAddrPort("10.202.9.9:5353")

	identityErrCh := make(chan error, 8)
	AttachUDPHandler(dev.Stack, func(src, dst netip.AddrPort, payload []byte) {
		if peer, ok := idx.Lookup(src.Addr()); !ok || peer.Email != wantEmail {
			identityErrCh <- fmt.Errorf("peer identity lookup for src %v: ok=%v email=%q, want %q", src, ok, peer.Email, wantEmail)
			return
		}
		if dst != wantDest {
			identityErrCh <- fmt.Errorf("recovered dest = %v, want %v", dst, wantDest)
			return
		}
		// Echo the payload back, posing as a reply from the destination the
		// client dialed -- exactly what a real relay's downstream reply
		// would look like from the tunnel's point of view.
		if err := WriteUDPReply(dev.Stack, dst, src, payload); err != nil {
			identityErrCh <- fmt.Errorf("WriteUDPReply: %w", err)
		}
	})

	// Configure (IpcSet) must come after AttachUDPHandler -- see
	// newUnconfiguredDevice's doc comment.
	if err := dev.Configure(inst, DeviceOptions{}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	clientTun, clientNet, err := netstack.CreateNetTUN(
		[]netip.Addr{netip.MustParseAddr("10.202.0.2")},
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
	clientConf := fmt.Sprintf(
		"private_key=%s\njc=4\njmin=40\njmax=70\ns1=20\ns2=30\ns3=20\ns4=20\npublic_key=%s\nendpoint=127.0.0.1:%d\nallowed_ip=0.0.0.0/0\n",
		clientPrivHex, serverPubHex, listenPort)
	if err := clientDev.IpcSet(clientConf); err != nil {
		t.Fatalf("client IpcSet: %v", err)
	}
	if err := clientDev.Up(); err != nil {
		t.Fatalf("client Up: %v", err)
	}

	conn, err := clientNet.DialUDPAddrPort(netip.AddrPort{}, wantDest)
	if err != nil {
		t.Fatalf("client DialUDPAddrPort: %v", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(5 * time.Second)
	var buf [256]byte
	for {
		select {
		case err := <-identityErrCh:
			t.Fatal(err)
		default:
		}

		_ = conn.SetWriteDeadline(time.Now().Add(200 * time.Millisecond))
		if _, err := conn.Write([]byte(echoPayload)); err != nil {
			if time.Now().After(deadline) {
				t.Fatalf("client write never succeeded: %v", err)
			}
			continue
		}

		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, err := conn.Read(buf[:])
		if err != nil {
			if time.Now().After(deadline) {
				t.Fatalf("client never received a reply: %v", err)
			}
			continue
		}
		if got := string(buf[:n]); got != echoPayload {
			t.Fatalf("echoed payload = %q, want %q", got, echoPayload)
		}
		return
	}
}
