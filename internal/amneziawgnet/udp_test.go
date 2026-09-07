package amneziawgnet

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	awgconn "github.com/amnezia-vpn/amneziawg-go/v3/conn"
	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	"github.com/amnezia-vpn/amneziawg-go/v3/tun/netstack"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"

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

// udpDatagram builds a complete IPv4/UDP packet, the shape stackTun.Write
// expects from amneziawg-go after decryption.
func udpDatagram(src, dst netip.AddrPort, payload []byte) []byte {
	total := header.IPv4MinimumSize + header.UDPMinimumSize + len(payload)
	p := make([]byte, total)
	ip := header.IPv4(p)
	ip.Encode(&header.IPv4Fields{
		TotalLength: uint16(total),
		TTL:         64,
		Protocol:    uint8(header.UDPProtocolNumber),
		SrcAddr:     tcpip.AddrFromSlice(src.Addr().AsSlice()),
		DstAddr:     tcpip.AddrFromSlice(dst.Addr().AsSlice()),
	})
	ip.SetChecksum(^ip.CalculateChecksum())
	u := header.UDP(p[header.IPv4MinimumSize:])
	u.Encode(&header.UDPFields{
		SrcPort: src.Port(),
		DstPort: dst.Port(),
		Length:  uint16(header.UDPMinimumSize + len(payload)),
	})
	copy(p[header.IPv4MinimumSize+header.UDPMinimumSize:], payload)
	return p
}

// TestAttachUDPHandlerDoesNotStrandPacketBuffers drives a real datagram all the
// way through the stack: Range.ToSlice already copies, so cloning pkt only leaks.
func TestAttachUDPHandlerDoesNotStrandPacketBuffers(t *testing.T) {
	tun, gstack, err := createNetTUNWithStack([]netip.Addr{netip.MustParseAddr("10.77.0.1")}, 1420)
	if err != nil {
		t.Fatalf("createNetTUNWithStack: %v", err)
	}
	defer tun.Close()

	src := netip.MustParseAddrPort("10.77.0.2:40000")
	dst := netip.MustParseAddrPort("10.77.9.9:5353")
	payload := make([]byte, 512)
	var delivered int
	AttachUDPHandler(gstack, func(gotSrc, gotDst netip.AddrPort, got []byte) {
		if gotSrc != src || gotDst != dst || len(got) != len(payload) {
			t.Errorf("handler got (%v -> %v, %d bytes), want (%v -> %v, %d bytes)", gotSrc, gotDst, len(got), src, dst, len(payload))
		}
		delivered++
	})

	bufs := [][]byte{udpDatagram(src, dst, payload)}
	st := tun.(*stackTun)
	allocs := testing.AllocsPerRun(500, func() {
		if _, err := st.Write(bufs, 0); err != nil {
			t.Fatalf("Write: %v", err)
		}
	})
	if delivered == 0 {
		t.Fatal("handler never ran: the datagram never reached the UDP transport handler")
	}
	// 2 once nothing is stranded (1 is ToSlice itself), 8 with the leaked
	// clone plus the un-released packet buffer; -race adds ~1.
	if allocs > 4 {
		t.Fatalf("UDP delivery allocates %v times per datagram, want <=4: pooled packet buffers are being stranded", allocs)
	}
}
