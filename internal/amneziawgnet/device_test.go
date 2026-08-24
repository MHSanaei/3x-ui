package amneziawgnet

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strings"
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
		Obfuscation: amneziawg.Obfuscation31{
			Jc: 4, Jmin: 40, Jmax: 70,
			S1: 20, S2: 30, S3: 20, S4: 20,
		},
		Peers: []amneziawg.Peer{{
			Email:      wantEmail,
			PublicKey:  clientPub,
			AllowedIPs: []string{"10.201.0.2/32"},
		}},
	}

	dev, err := newUnconfiguredDevice(inst, DeviceOptions{})
	if err != nil {
		t.Fatalf("newUnconfiguredDevice: %v", err)
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

	// Configure (IpcSet) must come after AttachTCPForwarder -- see
	// newUnconfiguredDevice's doc comment: IpcSet is what starts the peer's
	// receive goroutine, which must never be able to run before the
	// forwarder is registered on the stack.
	if err := dev.Configure(inst, DeviceOptions{}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

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

// TestBuildUAPIConfigHeaderProtectionAndContentPaddingLines is a cheap,
// network-free companion to the real round-trip test below: confirms the 2
// AWG 3.0 UAPI lines only appear when set, and that a malformed
// HeaderProtectionKey surfaces a clear, wrapped error instead of silently
// producing a UAPI string amneziawg-go's own IpcSet would reject uselessly.
func TestBuildUAPIConfigHeaderProtectionAndContentPaddingLines(t *testing.T) {
	priv, _, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	inst := amneziawg.Instance{
		PrivateKey: priv,
		Obfuscation: amneziawg.Obfuscation31{
			S1: 20, S2: 20, S3: 20, S4: 20,
		},
	}

	conf, err := buildUAPIConfig(inst, DeviceOptions{})
	if err != nil {
		t.Fatalf("buildUAPIConfig with empty options: %v", err)
	}
	if strings.Contains(conf, "header_protection_key=") || strings.Contains(conf, "content_padding_addition=") {
		t.Fatalf("empty DeviceOptions must not emit AWG 3.0 lines, got:\n%s", conf)
	}

	key, err := wireguard.GenerateWireguardPSK()
	if err != nil {
		t.Fatalf("generate header protection key: %v", err)
	}
	conf, err = buildUAPIConfig(inst, DeviceOptions{HeaderProtectionKey: key, ContentPaddingAddition: "20-40"})
	if err != nil {
		t.Fatalf("buildUAPIConfig with AWG 3.0 options: %v", err)
	}
	if !strings.Contains(conf, "header_protection_key=") {
		t.Errorf("expected a header_protection_key= line, got:\n%s", conf)
	}
	if !strings.Contains(conf, "content_padding_addition=20-40\n") {
		t.Errorf("expected a content_padding_addition=20-40 line, got:\n%s", conf)
	}

	if _, err := buildUAPIConfig(inst, DeviceOptions{HeaderProtectionKey: "not-a-valid-base64-key"}); err == nil {
		t.Fatal("a malformed HeaderProtectionKey must be rejected, not silently passed through")
	}
}

// TestNewDeviceHeaderProtectionAndContentPaddingRoundTrip is the real proof
// behind AmneziaWG 3.0's admin-facing HeaderProtectionKey/
// ContentPaddingAddition fields: a genuine amneziawg-go client, configured
// with matching header_protection_key/content_padding_addition UAPI lines
// (S1-S4 all >= 12, the hard requirement amneziawg-go's own IpcSet enforces
// for header protection), completes a real handshake against a Device built
// via NewDevice/DeviceOptions and exchanges real application data both
// directions through it. This is more than a handshake-completed check --
// it also confirms actual payload bytes survive content padding on both the
// send and receive sides, the specific area a third-party AmneziaWG
// installer project's docs flagged a past interop concern for (see the
// migration plan's own risk note); it is not a substitute for real-VPS
// verification against the official client, but it is the cheapest
// available local check against a regression in either engine's own padding
// handling.
func TestNewDeviceHeaderProtectionAndContentPaddingRoundTrip(t *testing.T) {
	serverPriv, serverPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate server keypair: %v", err)
	}
	clientPriv, clientPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate client keypair: %v", err)
	}
	headerProtectionKey, err := wireguard.GenerateWireguardPSK()
	if err != nil {
		t.Fatalf("generate header protection key: %v", err)
	}

	const listenPort = 58713 // fixed loopback test port, distinct from the handshake test above
	const contentPaddingAddition = "20-40"

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
			S1: 20, S2: 30, S3: 20, S4: 20, // all >= 12, required for header protection
		},
		Peers: []amneziawg.Peer{{
			Email:      "hp-peer@example.com",
			PublicKey:  clientPub,
			AllowedIPs: []string{"10.202.0.2/32"},
		}},
	}

	opts := DeviceOptions{
		HeaderProtectionKey:    headerProtectionKey,
		ContentPaddingAddition: contentPaddingAddition,
	}
	dev, err := newUnconfiguredDevice(inst, opts)
	if err != nil {
		t.Fatalf("newUnconfiguredDevice: %v", err)
	}
	defer dev.Close()

	const wantRequest = "hello from client"
	const wantReply = "hello from server"
	serverDone := make(chan error, 1)
	AttachTCPForwarder(dev.Stack, func(conn *gonet.TCPConn, dest netip.AddrPort) {
		defer conn.Close()
		buf := make([]byte, len(wantRequest))
		if _, err := io.ReadFull(conn, buf); err != nil {
			serverDone <- fmt.Errorf("server read: %w", err)
			return
		}
		if string(buf) != wantRequest {
			serverDone <- fmt.Errorf("server got %q, want %q", buf, wantRequest)
			return
		}
		if _, err := conn.Write([]byte(wantReply)); err != nil {
			serverDone <- fmt.Errorf("server write: %w", err)
			return
		}
		serverDone <- nil
	})

	// Configure (IpcSet) must come after AttachTCPForwarder -- see
	// newUnconfiguredDevice's doc comment.
	if err := dev.Configure(inst, opts); err != nil {
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
	headerProtectionKeyHex, err := wireguard.KeyToHex(headerProtectionKey)
	if err != nil {
		t.Fatalf("header protection key to hex: %v", err)
	}
	clientConf := fmt.Sprintf(
		"private_key=%s\njc=4\njmin=40\njmax=70\ns1=20\ns2=30\ns3=20\ns4=20\nheader_protection_key=%s\ncontent_padding_addition=%s\npublic_key=%s\nendpoint=127.0.0.1:%d\nallowed_ip=0.0.0.0/0\n",
		clientPrivHex, headerProtectionKeyHex, contentPaddingAddition, serverPubHex, listenPort)
	if err := clientDev.IpcSet(clientConf); err != nil {
		t.Fatalf("client IpcSet: %v", err)
	}
	if err := clientDev.Up(); err != nil {
		t.Fatalf("client Up: %v", err)
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var conn net.Conn
	for {
		c, dialErr := clientNet.DialContext(dialCtx, "tcp", "10.202.9.9:9999")
		if dialErr == nil {
			conn = c
			break
		}
		select {
		case <-dialCtx.Done():
			t.Fatalf("client dial never succeeded: %v", dialErr)
		case <-time.After(100 * time.Millisecond):
		}
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(wantRequest)); err != nil {
		t.Fatalf("client write: %v", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	reply := make([]byte, len(wantReply))
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("client read reply: %v", err)
	}
	if string(reply) != wantReply {
		t.Fatalf("client got reply %q, want %q", reply, wantReply)
	}

	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("server side: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the server side to finish")
	}
}

func TestBuildUAPIConfigRandomTrailersAndDisableCookiesLines(t *testing.T) {
	priv, _, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}
	inst := amneziawg.Instance{PrivateKey: priv}

	// Unlike HeaderProtectionKey/ContentPaddingAddition, these two lines
	// must always be present -- see DeviceOptions.RandomTrailers's own doc
	// comment on why an absent line (instead of an explicit "false") would
	// break the reconfigure-in-place diff for a true->false edit.
	conf, err := buildUAPIConfig(inst, DeviceOptions{})
	if err != nil {
		t.Fatalf("buildUAPIConfig with empty options: %v", err)
	}
	if !strings.Contains(conf, "random_trailers=false\n") {
		t.Errorf("expected an explicit random_trailers=false line even when unset, got:\n%s", conf)
	}
	if !strings.Contains(conf, "disable_cookies=false\n") {
		t.Errorf("expected an explicit disable_cookies=false line even when unset, got:\n%s", conf)
	}

	conf, err = buildUAPIConfig(inst, DeviceOptions{RandomTrailers: true, DisableCookies: true})
	if err != nil {
		t.Fatalf("buildUAPIConfig with both enabled: %v", err)
	}
	if !strings.Contains(conf, "random_trailers=true\n") {
		t.Errorf("expected a random_trailers=true line, got:\n%s", conf)
	}
	if !strings.Contains(conf, "disable_cookies=true\n") {
		t.Errorf("expected a disable_cookies=true line, got:\n%s", conf)
	}
}

// TestNewDeviceRandomTrailersAndDisableCookiesRoundTrip is the real proof
// behind AmneziaWG 3.1's two new device-wide toggles: a genuine amneziawg-go
// client with matching random_trailers=true/disable_cookies=true UAPI lines
// completes a real handshake against a Device built via NewDevice/
// DeviceOptions and exchanges real application data both directions through
// it. This specifically exercises amneziawg-go's receive.go size-matching
// path for RandomTrailers (device_test.go's HeaderProtection test doesn't
// enable it), which only accepts a message when
// `size == expectedSize || randomTrailers && size > expectedSize` -- proof
// that setting it on both ends really does interoperate, not just that
// IpcSet accepts the value.
func TestNewDeviceRandomTrailersAndDisableCookiesRoundTrip(t *testing.T) {
	serverPriv, serverPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate server keypair: %v", err)
	}
	clientPriv, clientPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate client keypair: %v", err)
	}

	const listenPort = 58721 // fixed loopback test port, distinct from every other test in this package

	inst := amneziawg.Instance{
		Id:            3,
		InterfaceName: "awgtest3",
		ListenPort:    listenPort,
		PrivateKey:    serverPriv,
		PublicKey:     serverPub,
		Address:       []string{"10.203.0.1/24"},
		MTU:           1420,
		Peers: []amneziawg.Peer{{
			Email:      "trailer-peer@example.com",
			PublicKey:  clientPub,
			AllowedIPs: []string{"10.203.0.2/32"},
		}},
	}

	opts := DeviceOptions{RandomTrailers: true, DisableCookies: true}
	dev, err := newUnconfiguredDevice(inst, opts)
	if err != nil {
		t.Fatalf("newUnconfiguredDevice: %v", err)
	}
	defer dev.Close()

	const wantRequest = "hello from client, with a trailer"
	const wantReply = "hello from server, with a trailer"
	serverDone := make(chan error, 1)
	AttachTCPForwarder(dev.Stack, func(conn *gonet.TCPConn, dest netip.AddrPort) {
		defer conn.Close()
		buf := make([]byte, len(wantRequest))
		if _, err := io.ReadFull(conn, buf); err != nil {
			serverDone <- fmt.Errorf("server read: %w", err)
			return
		}
		if string(buf) != wantRequest {
			serverDone <- fmt.Errorf("server got %q, want %q", buf, wantRequest)
			return
		}
		if _, err := conn.Write([]byte(wantReply)); err != nil {
			serverDone <- fmt.Errorf("server write: %w", err)
			return
		}
		serverDone <- nil
	})

	// Configure (IpcSet) must come after AttachTCPForwarder -- see
	// newUnconfiguredDevice's doc comment.
	if err := dev.Configure(inst, opts); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	clientTun, clientNet, err := netstack.CreateNetTUN(
		[]netip.Addr{netip.MustParseAddr("10.203.0.2")},
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
		"private_key=%s\nrandom_trailers=true\ndisable_cookies=true\npublic_key=%s\nendpoint=127.0.0.1:%d\nallowed_ip=0.0.0.0/0\n",
		clientPrivHex, serverPubHex, listenPort)
	if err := clientDev.IpcSet(clientConf); err != nil {
		t.Fatalf("client IpcSet: %v", err)
	}
	if err := clientDev.Up(); err != nil {
		t.Fatalf("client Up: %v", err)
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var conn net.Conn
	for {
		c, dialErr := clientNet.DialContext(dialCtx, "tcp", "10.203.9.9:9999")
		if dialErr == nil {
			conn = c
			break
		}
		select {
		case <-dialCtx.Done():
			t.Fatalf("client dial never succeeded: %v", dialErr)
		case <-time.After(100 * time.Millisecond):
		}
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(wantRequest)); err != nil {
		t.Fatalf("client write: %v", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	reply := make([]byte, len(wantReply))
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("client read reply: %v", err)
	}
	if string(reply) != wantReply {
		t.Fatalf("client got reply %q, want %q", reply, wantReply)
	}

	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("server side: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the server side to finish")
	}
}
