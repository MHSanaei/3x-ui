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
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// TestRandomTrailersThroughputRepro and TestRandomTrailersThroughputReproViaSocks5
// are standalone diagnostics, not regression tests: reproduce (or rule out)
// the ~10x live-connection throughput collapse a real box measured, tied
// specifically to RandomTrailers. Every other field matches the live
// isolation exactly (MTU 1280, S4 22, ContentPaddingAddition 22-95,
// DisableCookies true).
//
// The first uses only amneziawg-go and this package's own netstack forwarder
// -- no SOCKS5 relay, no Xray, no real network interface. The second adds
// the one remaining piece of the real path: a genuine SOCKS5 hop through
// relay.go's own RelayTCP, against a minimal test SOCKS5 server standing in
// for Xray's inbound+outbound. Comparing the two localizes the cause to
// whichever layer actually shows the gap.
const reproPayloadSize = 30 * 1024 * 1024 // 30MB

func TestRandomTrailersThroughputRepro(t *testing.T) {
	if testing.Short() {
		t.Skip("pushes 60MB through two real handshakes; skip in -short")
	}
	var elapsed [2]time.Duration
	for i, rt := range []bool{false, true} {
		rt := rt
		t.Run(fmt.Sprintf("RandomTrailers=%v", rt), func(t *testing.T) {
			elapsed[i] = runThroughputRepro(t, rt, 58800+i, directWriteHandler)
			mbps := float64(reproPayloadSize) / elapsed[i].Seconds() / (1024 * 1024) * 8
			t.Logf("RandomTrailers=%v: %d bytes in %v = %.1f Mbit/s", rt, reproPayloadSize, elapsed[i], mbps)
		})
	}
	t.Logf("ratio, direct (false/true): %.2fx", elapsed[1].Seconds()/elapsed[0].Seconds())
}

func TestRandomTrailersThroughputReproViaSocks5(t *testing.T) {
	if testing.Short() {
		t.Skip("pushes 60MB through two real handshakes plus a real SOCKS5 hop; skip in -short")
	}
	const email = "repro@example.com"
	const password = "test-pass"
	var elapsed [2]time.Duration
	for i, rt := range []bool{false, true} {
		rt := rt
		t.Run(fmt.Sprintf("RandomTrailers=%v", rt), func(t *testing.T) {
			socksAddr := startTestSocks5Server(t, email, password, func(conn net.Conn) {
				defer conn.Close()
				_, _ = conn.Write(make([]byte, reproPayloadSize))
			})
			relay := SocksRelay{Addr: socksAddr, Password: password}
			elapsed[i] = runThroughputRepro(t, rt, 58810+i, func(conn *gonet.TCPConn, dest netip.AddrPort) {
				relay.RelayTCP(conn, email, dest)
			})
			mbps := float64(reproPayloadSize) / elapsed[i].Seconds() / (1024 * 1024) * 8
			t.Logf("RandomTrailers=%v (via SOCKS5): %d bytes in %v = %.1f Mbit/s", rt, reproPayloadSize, elapsed[i], mbps)
		})
	}
	t.Logf("ratio, via SOCKS5 (false/true): %.2fx", elapsed[1].Seconds()/elapsed[0].Seconds())
}

func directWriteHandler(conn *gonet.TCPConn, dest netip.AddrPort) {
	defer conn.Close()
	_, _ = conn.Write(make([]byte, reproPayloadSize))
}

// startTestSocks5Server starts a minimal SOCKS5 server (RFC 1928 handshake +
// RFC 1929 username/password auth, matching what golang.org/x/net/proxy's
// client sends) that accepts exactly one connection, requires
// wantUser/wantPass, accepts a CONNECT to any destination without actually
// dialing it, and hands the raw connection to serve -- standing in for
// Xray's SOCKS5 inbound+outbound so relay.go's RelayTCP runs against a real
// socket exactly as it does in production.
func startTestSocks5Server(t *testing.T, wantUser, wantPass string, serve func(net.Conn)) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("socks5 test server listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		if err := negotiateSocks5(conn, wantUser, wantPass); err != nil {
			conn.Close()
			return
		}
		serve(conn)
	}()
	return ln.Addr().String()
}

func negotiateSocks5(conn net.Conn, wantUser, wantPass string) error {
	buf := make([]byte, 262)
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return err
	}
	if buf[0] != 0x05 {
		return fmt.Errorf("bad SOCKS version %d", buf[0])
	}
	if _, err := io.ReadFull(conn, buf[:int(buf[1])]); err != nil {
		return err
	}
	if _, err := conn.Write([]byte{0x05, 0x02}); err != nil { // select username/password
		return err
	}

	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return err
	}
	ulen := int(buf[1])
	if _, err := io.ReadFull(conn, buf[:ulen]); err != nil {
		return err
	}
	user := string(buf[:ulen])
	if _, err := io.ReadFull(conn, buf[:1]); err != nil {
		return err
	}
	plen := int(buf[0])
	if _, err := io.ReadFull(conn, buf[:plen]); err != nil {
		return err
	}
	pass := string(buf[:plen])
	status := byte(0x00)
	if user != wantUser || pass != wantPass {
		status = 0x01
	}
	if _, err := conn.Write([]byte{0x01, status}); err != nil {
		return err
	}
	if status != 0x00 {
		return fmt.Errorf("socks5 test server: auth rejected for user %q", user)
	}

	if _, err := io.ReadFull(conn, buf[:4]); err != nil {
		return err
	}
	if buf[1] != 0x01 {
		return fmt.Errorf("unsupported SOCKS5 command %d", buf[1])
	}
	var addrLen int
	switch buf[3] {
	case 0x01:
		addrLen = 4
	case 0x03:
		if _, err := io.ReadFull(conn, buf[:1]); err != nil {
			return err
		}
		addrLen = int(buf[0])
	case 0x04:
		addrLen = 16
	default:
		return fmt.Errorf("unsupported SOCKS5 address type %d", buf[3])
	}
	if _, err := io.ReadFull(conn, buf[:addrLen+2]); err != nil { // addr + port
		return err
	}

	_, err := conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	return err
}

func runThroughputRepro(t *testing.T, randomTrailers bool, listenPort int, handler func(conn *gonet.TCPConn, dest netip.AddrPort)) time.Duration {
	t.Helper()
	serverPriv, serverPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate server keypair: %v", err)
	}
	clientPriv, clientPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate client keypair: %v", err)
	}

	inst := amneziawg.Instance{
		Id:            1,
		InterfaceName: "awgrepro1",
		ListenPort:    listenPort,
		PrivateKey:    serverPriv,
		PublicKey:     serverPub,
		Address:       []string{"10.202.0.1/24"},
		MTU:           1280,
		Obfuscation: amneziawg.Obfuscation20{
			Jc: 3, Jmin: 88, Jmax: 156,
			S1: 30, S2: 138, S3: 13, S4: 22,
		},
		Peers: []amneziawg.Peer{{
			Email:      "repro@example.com",
			PublicKey:  clientPub,
			AllowedIPs: []string{"10.202.0.2/32"},
		}},
	}
	opts := DeviceOptions{
		ContentPaddingAddition: "22-95",
		RandomTrailers:         randomTrailers,
		DisableCookies:         true,
	}

	dev, err := newUnconfiguredDevice(inst, opts)
	if err != nil {
		t.Fatalf("newUnconfiguredDevice: %v", err)
	}
	defer dev.Close()

	AttachTCPForwarder(dev.Stack, handler)

	if err := dev.Configure(inst, opts); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	clientTun, clientNet, err := netstack.CreateNetTUN(
		[]netip.Addr{netip.MustParseAddr("10.202.0.2")},
		[]netip.Addr{netip.MustParseAddr("1.1.1.1")}, 1280)
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
	rt := "false"
	if randomTrailers {
		rt = "true"
	}
	// allowed_ip=0.0.0.0/0 on the client matches a real VPN client's own
	// config, same reasoning as the identity-recovery test next to this one.
	clientConf := fmt.Sprintf(
		"private_key=%s\njc=3\njmin=88\njmax=156\ns1=30\ns2=138\ns3=13\ns4=22\ncontent_padding_addition=22-95\nrandom_trailers=%s\ndisable_cookies=true\npublic_key=%s\nendpoint=127.0.0.1:%d\nallowed_ip=0.0.0.0/0\n",
		clientPrivHex, rt, serverPubHex, listenPort)
	if err := clientDev.IpcSet(clientConf); err != nil {
		t.Fatalf("client IpcSet: %v", err)
	}
	if err := clientDev.Up(); err != nil {
		t.Fatalf("client Up: %v", err)
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var conn net.Conn
	var lastErr error
	for {
		c, dialErr := clientNet.DialContext(dialCtx, "tcp", "10.202.9.9:9999")
		if dialErr == nil {
			conn = c
			break
		}
		lastErr = dialErr
		select {
		case <-dialCtx.Done():
			t.Fatalf("client dial never succeeded: %v", lastErr)
		case <-time.After(100 * time.Millisecond):
		}
	}
	defer conn.Close()

	readStart := time.Now()
	n, err := io.Copy(io.Discard, conn)
	if err != nil && err != io.EOF {
		t.Fatalf("client read: %v", err)
	}
	if n != reproPayloadSize {
		t.Fatalf("client received %d bytes, want %d", n, reproPayloadSize)
	}
	return time.Since(readStart)
}
