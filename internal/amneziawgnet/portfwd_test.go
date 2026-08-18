package amneziawgnet

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	awgconn "github.com/amnezia-vpn/amneziawg-go/v3/conn"
	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	"github.com/amnezia-vpn/amneziawg-go/v3/tun/netstack"
	"gvisor.dev/gvisor/pkg/tcpip/stack"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

func peerWithPortsAndIPs(email, forwardedPorts string, ips ...string) amneziawg.Peer {
	return amneziawg.Peer{Email: email, PublicKey: "pub-" + email, AllowedIPs: ips, ForwardedPorts: forwardedPorts}
}

// --- desiredPeerTargets ---

func TestDesiredPeerTargetsPrefersIPv4(t *testing.T) {
	inst := amneziawg.Instance{IPv6Enabled: true, Peers: []amneziawg.Peer{
		peerWithPortsAndIPs("a@x", "", "10.8.1.2/32", "fd86::2/128"),
	}}
	got := desiredPeerTargets(inst)
	addr, ok := got["a@x"]
	if !ok || addr.String() != "10.8.1.2" {
		t.Fatalf("desiredPeerTargets = %v, want a@x -> 10.8.1.2", got)
	}
}

func TestDesiredPeerTargetsFallsBackToIPv6WhenEnabled(t *testing.T) {
	inst := amneziawg.Instance{IPv6Enabled: true, Peers: []amneziawg.Peer{
		peerWithPortsAndIPs("a@x", "", "fd86::2/128"),
	}}
	got := desiredPeerTargets(inst)
	addr, ok := got["a@x"]
	if !ok || addr.String() != "fd86::2" {
		t.Fatalf("desiredPeerTargets = %v, want a@x -> fd86::2", got)
	}
}

func TestDesiredPeerTargetsSkipsIPv6OnlyWhenIPv6Disabled(t *testing.T) {
	inst := amneziawg.Instance{IPv6Enabled: false, Peers: []amneziawg.Peer{
		peerWithPortsAndIPs("a@x", "", "fd86::2/128"),
	}}
	if got := desiredPeerTargets(inst); len(got) != 0 {
		t.Fatalf("desiredPeerTargets = %v, want empty (IPv6-only peer, IPv6 disabled)", got)
	}
}

func TestDesiredPeerTargetsSkipsPeerWithoutEmailOrAddress(t *testing.T) {
	inst := amneziawg.Instance{IPv6Enabled: true, Peers: []amneziawg.Peer{
		peerWithPortsAndIPs("", "", "10.8.1.2/32"), // no email
		peerWithPortsAndIPs("b@x", ""),             // no AllowedIPs at all
	}}
	if got := desiredPeerTargets(inst); len(got) != 0 {
		t.Fatalf("desiredPeerTargets = %v, want empty", got)
	}
}

// --- desiredPortForwardKeys ---

func TestDesiredPortForwardKeysEmptyWhenNoForwardedPorts(t *testing.T) {
	inst := amneziawg.Instance{Peers: []amneziawg.Peer{
		peerWithPortsAndIPs("a@x", "", "10.8.1.2/32"),
	}}
	if got := desiredPortForwardKeys(inst); len(got) != 0 {
		t.Fatalf("desiredPortForwardKeys = %v, want empty", got)
	}
}

func TestDesiredPortForwardKeysEmptyWhenNoResolvableTarget(t *testing.T) {
	// ForwardedPorts is set, but the peer has no AllowedIPs to resolve a
	// target from -- must not produce keys for a peer nothing can dial.
	inst := amneziawg.Instance{Peers: []amneziawg.Peer{
		{Email: "a@x", ForwardedPorts: "8080"},
	}}
	if got := desiredPortForwardKeys(inst); len(got) != 0 {
		t.Fatalf("desiredPortForwardKeys = %v, want empty", got)
	}
}

func TestDesiredPortForwardKeysOneTCPAndUDPKeyPerPort(t *testing.T) {
	inst := amneziawg.Instance{Peers: []amneziawg.Peer{
		peerWithPortsAndIPs("a@x", "8080,8081", "10.8.1.2/32"),
	}}
	got := desiredPortForwardKeys(inst)
	if len(got) != 4 {
		t.Fatalf("desiredPortForwardKeys = %v, want 4 entries (2 ports x 2 protocols)", got)
	}
	for _, port := range []int{8080, 8081} {
		for _, proto := range []portForwardProto{tcpForward, udpForward} {
			key := portForwardKey{email: "a@x", port: port, proto: proto}
			if _, ok := got[key]; !ok {
				t.Errorf("desiredPortForwardKeys missing %+v", key)
			}
		}
	}
}

func TestDesiredPortForwardKeysMultiplePeersDoNotMix(t *testing.T) {
	inst := amneziawg.Instance{Peers: []amneziawg.Peer{
		peerWithPortsAndIPs("a@x", "8080", "10.8.1.2/32"),
		peerWithPortsAndIPs("b@x", "8080", "10.8.1.3/32"), // same port, different peer
	}}
	got := desiredPortForwardKeys(inst)
	if len(got) != 4 {
		t.Fatalf("desiredPortForwardKeys = %v, want 4 entries (2 peers x 2 protocols, same port kept separate per email)", got)
	}
}

// --- PortForwardSet.Reconcile: real stack, no handshake needed (dialing
// isn't exercised by these -- only the host-facing listener lifecycle) ---

func newTestStack(t *testing.T, addr string) *stack.Stack {
	t.Helper()
	tunDev, gstack, err := createNetTUNWithStack([]netip.Addr{netip.MustParseAddr(addr)}, 1420)
	if err != nil {
		t.Fatalf("createNetTUNWithStack: %v", err)
	}
	t.Cleanup(func() { tunDev.Close() })
	return gstack
}

func dialLoopback(t *testing.T, network string, port int) {
	t.Helper()
	conn, err := net.DialTimeout(network, fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("dial 127.0.0.1:%d (%s): %v", port, network, err)
	}
	conn.Close()
}

func TestPortForwardSetReconcileOpensAndClosesListeners(t *testing.T) {
	gs := newTestStack(t, "10.211.0.1")
	set := NewPortForwardSet(gs, 501)

	const port = 58910
	inst := amneziawg.Instance{Peers: []amneziawg.Peer{
		peerWithPortsAndIPs("a@x", fmt.Sprintf("%d", port), "10.211.0.2/32"),
	}}

	set.Reconcile(inst)
	set.mu.Lock()
	n := len(set.listeners)
	set.mu.Unlock()
	if n != 2 {
		t.Fatalf("listeners after Reconcile = %d, want 2 (tcp+udp)", n)
	}
	dialLoopback(t, "tcp", port) // proves a real host listener is actually bound

	set.mu.Lock()
	tcpBefore := set.listeners[portForwardKey{email: "a@x", port: port, proto: tcpForward}]
	set.mu.Unlock()

	// Reconciling again with an unchanged instance must not close and
	// reopen an unaffected listener.
	set.Reconcile(inst)
	set.mu.Lock()
	tcpAfter := set.listeners[portForwardKey{email: "a@x", port: port, proto: tcpForward}]
	set.mu.Unlock()
	if tcpBefore != tcpAfter {
		t.Error("Reconcile with an unchanged instance replaced an unaffected listener")
	}

	// Peer removed entirely -> both listeners close.
	set.Reconcile(amneziawg.Instance{})
	set.mu.Lock()
	n = len(set.listeners)
	set.mu.Unlock()
	if n != 0 {
		t.Fatalf("listeners after removal Reconcile = %d, want 0", n)
	}
	if _, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second); err == nil {
		t.Error("port still accepting connections after the listener should have closed")
	}
}

func TestPortForwardSetReconcileSurvivesPreBoundPort(t *testing.T) {
	gs := newTestStack(t, "10.211.1.1")
	set := NewPortForwardSet(gs, 502)

	const collidingPort = 58911
	const okPort = 58912
	blocker, err := net.Listen("tcp", fmt.Sprintf(":%d", collidingPort))
	if err != nil {
		t.Fatalf("pre-bind test port: %v", err)
	}
	defer blocker.Close()

	inst := amneziawg.Instance{Peers: []amneziawg.Peer{
		peerWithPortsAndIPs("a@x", fmt.Sprintf("%d,%d", collidingPort, okPort), "10.211.1.2/32"),
	}}

	// Must not panic despite one of the two ports being unbindable, and the
	// other port (and its UDP counterpart on the colliding port) must still
	// open normally.
	set.Reconcile(inst)
	set.mu.Lock()
	n := len(set.listeners)
	_, tcpCollidingOpen := set.listeners[portForwardKey{email: "a@x", port: collidingPort, proto: tcpForward}]
	_, udpCollidingOpen := set.listeners[portForwardKey{email: "a@x", port: collidingPort, proto: udpForward}]
	set.mu.Unlock()
	if n != 3 {
		t.Fatalf("listeners after Reconcile with one pre-bound port = %d, want 3 (4 desired minus the 1 that couldn't bind)", n)
	}
	if tcpCollidingOpen {
		t.Error("TCP listener on the pre-bound port opened despite the real bind conflict")
	}
	if !udpCollidingOpen {
		t.Error("UDP listener on the colliding port's own number should still open (TCP and UDP binds are independent)")
	}
	dialLoopback(t, "tcp", okPort)

	set.Close()
}

// --- Real round trip: a genuine amneziawg-go client handshakes against a
// real server Device, PortForwardSet opens a real host listener, and a real
// external-side dial (this test's own process) round-trips bytes through
// the actual encrypted tunnel to a service listening on the client's own
// netstack -- proving the full path, not just the listener bookkeeping
// above. Modeled closely on device_test.go's
// TestNewDeviceHandshakeForwarderAndIdentity.
func TestPortForwardRoundTripTCPAndUDP(t *testing.T) {
	serverPriv, serverPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate server keypair: %v", err)
	}
	clientPriv, clientPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate client keypair: %v", err)
	}

	const listenPort = 58920 // fixed loopback test port, matches this package's existing test convention
	const tcpPort = 58921
	const udpPort = 58922
	const clientAddr = "10.202.0.2"

	inst := amneziawg.Instance{
		Id:            5,
		InterfaceName: "awgtest5",
		ListenPort:    listenPort,
		PrivateKey:    serverPriv,
		PublicKey:     serverPub,
		Address:       []string{"10.202.0.1/24"},
		MTU:           1420,
		Obfuscation: amneziawg.Obfuscation31{
			Jc: 4, Jmin: 40, Jmax: 70,
			S1: 20, S2: 30, S3: 20, S4: 20,
		},
		Peers: []amneziawg.Peer{
			{
				Email:          "client@test",
				PublicKey:      clientPub,
				AllowedIPs:     []string{clientAddr + "/32"},
				ForwardedPorts: fmt.Sprintf("%d,%d", tcpPort, udpPort),
			},
		},
	}

	dev, err := NewDevice(inst, DeviceOptions{})
	if err != nil {
		t.Fatalf("NewDevice: %v", err)
	}
	defer dev.Close()

	set := NewPortForwardSet(dev.Stack, inst.Id)
	set.Reconcile(inst)
	defer set.Close()

	// Real amneziawg-go client, same recipe as device_test.go.
	clientTun, clientNet, err := netstack.CreateNetTUN(
		[]netip.Addr{netip.MustParseAddr(clientAddr)},
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

	// Prime the handshake before exercising the actual port forwards below.
	// The server only learns the client's real (roaming) endpoint from a
	// packet the client sends it -- buildUAPIConfig never configures an
	// endpoint= for a peer server-side (see device.go), and the server has
	// no route to initiate a handshake toward an endpoint it doesn't know --
	// so without this, relayTCPForward's own dial toward the client races a
	// handshake that can never even start server-side and fails outright.
	// A throwaway client dial toward nothing in particular is enough:
	// queuing any outbound packet triggers amneziawg-go's own automatic
	// handshake initiation regardless of whether the dial itself ever
	// succeeds (nothing server-side is listening for it), so this loop
	// deliberately ignores the dial's own outcome and just gives the
	// handshake a few real attempts to complete in the background.
	primeCtx, primeCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer primeCancel()
	for {
		if conn, dialErr := clientNet.DialContext(primeCtx, "tcp", "10.202.9.9:9999"); dialErr == nil {
			conn.Close()
		}
		select {
		case <-primeCtx.Done():
			goto primed
		case <-time.After(200 * time.Millisecond):
		}
	}
primed:

	// A real service on the client's own netstack -- what a real forwarded
	// port is ultimately supposed to reach.
	tcpSvc, err := clientNet.ListenTCPAddrPort(netip.MustParseAddrPort(fmt.Sprintf("%s:%d", clientAddr, tcpPort)))
	if err != nil {
		t.Fatalf("client ListenTCP: %v", err)
	}
	defer tcpSvc.Close()
	go func() {
		for {
			c, err := tcpSvc.Accept()
			if err != nil {
				return
			}
			go func() { io.Copy(c, c); c.Close() }()
		}
	}()

	udpSvc, err := clientNet.ListenUDPAddrPort(netip.MustParseAddrPort(fmt.Sprintf("%s:%d", clientAddr, udpPort)))
	if err != nil {
		t.Fatalf("client ListenUDP: %v", err)
	}
	defer udpSvc.Close()
	go func() {
		buf := make([]byte, 1500)
		for {
			n, addr, err := udpSvc.ReadFrom(buf)
			if err != nil {
				return
			}
			udpSvc.WriteTo(buf[:n], addr)
		}
	}()

	// Retry the TCP dial rather than guessing a fixed handshake delay --
	// the handshake happens lazily on first real traffic.
	const wantTCP = "port-forward tcp round trip"
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var tcpConn net.Conn
	var lastErr error
	for {
		tcpConn, lastErr = net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", tcpPort), time.Second)
		if lastErr == nil {
			break
		}
		select {
		case <-dialCtx.Done():
			t.Fatalf("external TCP dial never succeeded: %v", lastErr)
		case <-time.After(150 * time.Millisecond):
		}
	}
	defer tcpConn.Close()
	if _, err := tcpConn.Write([]byte(wantTCP)); err != nil {
		t.Fatalf("write to forwarded TCP port: %v", err)
	}
	tcpConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	gotTCP := make([]byte, len(wantTCP))
	if _, err := io.ReadFull(tcpConn, gotTCP); err != nil {
		t.Fatalf("read echo from forwarded TCP port: %v", err)
	}
	if string(gotTCP) != wantTCP {
		t.Errorf("TCP round trip = %q, want %q", gotTCP, wantTCP)
	}

	// UDP: the tunnel is already up (handshake completed above), so this
	// can dial straight away.
	const wantUDP = "port-forward udp round trip"
	udpConn, err := net.DialTimeout("udp", fmt.Sprintf("127.0.0.1:%d", udpPort), time.Second)
	if err != nil {
		t.Fatalf("external UDP dial: %v", err)
	}
	defer udpConn.Close()
	if _, err := udpConn.Write([]byte(wantUDP)); err != nil {
		t.Fatalf("write to forwarded UDP port: %v", err)
	}
	udpConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	gotUDP := make([]byte, len(wantUDP))
	if _, err := io.ReadFull(udpConn, gotUDP); err != nil {
		t.Fatalf("read echo from forwarded UDP port: %v", err)
	}
	if string(gotUDP) != wantUDP {
		t.Errorf("UDP round trip = %q, want %q", gotUDP, wantUDP)
	}
}
