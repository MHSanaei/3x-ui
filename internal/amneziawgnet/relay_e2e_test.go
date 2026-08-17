package amneziawgnet

import (
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	awgconn "github.com/amnezia-vpn/amneziawg-go/v3/conn"
	"github.com/amnezia-vpn/amneziawg-go/v3/device"
	"github.com/amnezia-vpn/amneziawg-go/v3/tun/netstack"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// TestSocksRelayAgainstRealXray is Phase 2's real end-to-end proof: a
// genuine amneziawg-go client completes a real handshake against a Device
// built by NewDevice, dials a real TCP echo server and sends a real UDP
// echo datagram, and this package's own AttachTCPForwarder/AttachUDPHandler
// handlers relay both through RelayTCP/UDPRelay into an *actual xray-core
// process* (not a mock) running a SOCKS5 inbound built by
// SocksInboundSettings. Verifies real data round-trips on both protocols,
// then greps the real process's own debug log for
// "user>>>{email}>>>traffic>>>{up,down}link" -- the same proof Finding 3 of
// the migration plan established manually in Phase 0, now permanent,
// repo-owned test infrastructure. The UDP half in particular is the first
// real test of this package's hand-rolled SOCKS5 UDP ASSOCIATE client
// (relay.go) against an independent, authoritative implementation of the
// protocol rather than a mock this same session wrote.
//
// Skipped unless XRAY_E2E_BINARY points at an xray executable built from
// the same xray-core version as go.mod, matching internal/xray's own
// TestXrayAPI_E2E convention:
//
//	go install github.com/xtls/xray-core/main@<version from go.mod>
//	XRAY_E2E_BINARY=$GOBIN/main go test ./internal/amneziawgnet -run TestSocksRelayAgainstRealXray -v
func TestSocksRelayAgainstRealXray(t *testing.T) {
	bin := os.Getenv("XRAY_E2E_BINARY")
	if bin == "" {
		t.Skip("set XRAY_E2E_BINARY to an xray binary to run this test")
	}

	localIP, ok := firstNonLoopbackIPv4()
	if !ok {
		t.Skip("no non-loopback IPv4 address available on this host")
	}

	const wantEmail = "e2e-peer@example.com"
	const socksPassword = "loopback-only-not-a-real-secret"

	// --- real TCP + UDP echo servers on a real, non-loopback address ---
	// (dialing 127.0.0.1 as a tunnel-internal destination hangs -- gVisor
	// won't route loopback out an arbitrary NIC -- so the client dials
	// localIP instead; it must still be a *real* address since the actual
	// relay leg is a genuine OS-level dial from the xray-core process, not
	// anything inside the tunnel's virtual netstack.)
	tcpEcho, tcpEchoAddr := startTCPEcho(t, localIP)
	defer tcpEcho.Close()
	udpEcho, udpEchoAddr := startUDPEcho(t, localIP)
	defer udpEcho.Close()

	// --- real embedded AmneziaWG server + client, same shape as Phase 1's tests ---
	serverPriv, serverPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate server keypair: %v", err)
	}
	clientPriv, clientPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate client keypair: %v", err)
	}

	const listenPort = 58715
	inst := amneziawg.Instance{
		Id:            4,
		InterfaceName: "awgtest4",
		ListenPort:    listenPort,
		PrivateKey:    serverPriv,
		PublicKey:     serverPub,
		Address:       []string{"10.204.0.1/24"},
		MTU:           1420,
		Obfuscation: amneziawg.Obfuscation20{
			Jc: 4, Jmin: 40, Jmax: 70,
			S1: 20, S2: 30, S3: 20, S4: 20,
		},
		Peers: []amneziawg.Peer{{
			Email:      wantEmail,
			PublicKey:  clientPub,
			AllowedIPs: []string{"10.204.0.2/32"},
		}},
	}
	dev, err := newUnconfiguredDevice(inst, DeviceOptions{})
	if err != nil {
		t.Fatalf("newUnconfiguredDevice: %v", err)
	}
	defer dev.Close()
	idx := NewPeerIndex(inst.Peers)

	// --- real xray-core process with a SOCKS5 inbound built by this package ---
	socksPort := freePort(t)
	settingsJSON, err := SocksInboundSettings([]string{wantEmail}, socksPassword)
	if err != nil {
		t.Fatalf("SocksInboundSettings: %v", err)
	}
	var rawSettings any
	if err := json.Unmarshal(settingsJSON, &rawSettings); err != nil {
		t.Fatalf("unmarshal generated SOCKS5 settings: %v", err)
	}
	xrayCfg := map[string]any{
		"log": map[string]any{"loglevel": "debug"},
		"inbounds": []any{
			map[string]any{
				"listen":   "127.0.0.1",
				"port":     socksPort,
				"protocol": "socks",
				"settings": rawSettings,
				"tag":      "awg-e2e-socks",
			},
		},
		"outbounds": []any{
			map[string]any{"protocol": "freedom", "settings": map[string]any{}, "tag": "direct"},
		},
		"policy": map[string]any{
			"levels": map[string]any{
				"0": map[string]any{"statsUserUplink": true, "statsUserDownlink": true},
			},
		},
		"stats": map[string]any{},
	}
	cfgBytes, err := json.MarshalIndent(xrayCfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal xray config: %v", err)
	}
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(cfgPath, cfgBytes, 0o644); err != nil {
		t.Fatalf("write xray config: %v", err)
	}

	var xrayLog syncBuffer
	cmd := exec.Command(bin, "-c", cfgPath)
	cmd.Stdout = &xrayLog
	cmd.Stderr = &xrayLog
	if err := cmd.Start(); err != nil {
		t.Fatalf("start xray: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()
	waitForPort(t, socksPort)

	socksAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)
	relay := SocksRelay{Addr: socksAddr, Password: socksPassword}
	udpRelay := NewUDPRelay(relay, dev.Stack)
	defer udpRelay.Close()

	AttachTCPForwarder(dev.Stack, func(conn *gonet.TCPConn, dest netip.AddrPort) {
		srcAddrPort, err := netip.ParseAddrPort(conn.RemoteAddr().String())
		if err != nil {
			conn.Close()
			return
		}
		peer, ok := idx.Lookup(srcAddrPort.Addr().Unmap())
		if !ok {
			conn.Close()
			return
		}
		relay.RelayTCP(conn, peer.Email, dest)
	})
	AttachUDPHandler(dev.Stack, func(src, dst netip.AddrPort, payload []byte) {
		peer, ok := idx.Lookup(src.Addr())
		if !ok {
			return
		}
		udpRelay.Handle(src, dst, peer.Email, payload)
	})

	// Configure (IpcSet) must come after both attaches -- see
	// newUnconfiguredDevice's doc comment.
	if err := dev.Configure(inst, DeviceOptions{}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	// --- real client, real handshake, real traffic through the whole chain ---
	clientTun, clientNet, err := netstack.CreateNetTUN(
		[]netip.Addr{netip.MustParseAddr("10.204.0.2")},
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

	// TCP round trip.
	const tcpMsg = "hello over amneziawgnet+socks5+xray"
	dialDeadline := time.Now().Add(10 * time.Second)
	var tcpConn interface {
		Write([]byte) (int, error)
		Read([]byte) (int, error)
		Close() error
	}
	for {
		c, dialErr := clientNet.DialContext(t.Context(), "tcp", tcpEchoAddr.String())
		if dialErr == nil {
			tcpConn = c
			break
		}
		if time.Now().After(dialDeadline) {
			t.Fatalf("client TCP dial via tunnel never succeeded: %v", dialErr)
		}
		time.Sleep(150 * time.Millisecond)
	}
	defer tcpConn.Close()
	if _, err := tcpConn.Write([]byte(tcpMsg)); err != nil {
		t.Fatalf("client TCP write: %v", err)
	}
	tcpBuf := make([]byte, len(tcpMsg))
	if _, err := readFull(tcpConn, tcpBuf, 10*time.Second); err != nil {
		t.Fatalf("client TCP read: %v", err)
	}
	if string(tcpBuf) != tcpMsg {
		t.Errorf("TCP echo = %q, want %q", tcpBuf, tcpMsg)
	}

	// UDP round trip.
	const udpMsg = "hello-udp-over-socks5"
	uconn, err := clientNet.DialUDPAddrPort(netip.AddrPort{}, udpEchoAddr)
	if err != nil {
		t.Fatalf("client DialUDPAddrPort: %v", err)
	}
	defer uconn.Close()
	udpDeadline := time.Now().Add(10 * time.Second)
	var udpBuf [256]byte
	var gotUDP string
	for time.Now().Before(udpDeadline) {
		_ = uconn.SetWriteDeadline(time.Now().Add(300 * time.Millisecond))
		if _, err := uconn.Write([]byte(udpMsg)); err != nil {
			continue
		}
		_ = uconn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		n, err := uconn.Read(udpBuf[:])
		if err == nil {
			gotUDP = string(udpBuf[:n])
			break
		}
	}
	if gotUDP != udpMsg {
		t.Fatalf("UDP echo = %q, want %q (xray log follows)\n%s", gotUDP, udpMsg, xrayLog.String())
	}

	// Real per-peer stats attribution: stop xray so its log is complete, then
	// look for both directions' counters keyed by the peer's real email --
	// the exact proof Finding 3 established manually in Phase 0.
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
	log := xrayLog.String()
	wantUp := fmt.Sprintf("user>>>%s>>>traffic>>>uplink", wantEmail)
	wantDown := fmt.Sprintf("user>>>%s>>>traffic>>>downlink", wantEmail)
	if !strings.Contains(log, wantUp) {
		t.Errorf("xray log missing uplink stats counter %q\nfull log:\n%s", wantUp, log)
	}
	if !strings.Contains(log, wantDown) {
		t.Errorf("xray log missing downlink stats counter %q\nfull log:\n%s", wantDown, log)
	}
}

// TestManagerEnsureAutomaticallyWiresRelay is Phase 3's own real proof: unlike
// TestSocksRelayAgainstRealXray above (which builds a Device and attaches
// RelayTCP/UDPRelay by hand), this drives everything through the public
// Manager.Ensure entry point the real app actually calls -- confirming
// ensureLocked's own forwarder/UDP-handler attachment (added this phase)
// really does relay a fresh Device's traffic into Xray with zero manual
// wiring from the caller. Uses the exact port/password
// (SOCKSPortForInbound/SocksPassword) the Manager computes internally, so
// this only passes if that internal derivation and the externally-visible
// contract genuinely agree.
func TestManagerEnsureAutomaticallyWiresRelay(t *testing.T) {
	bin := os.Getenv("XRAY_E2E_BINARY")
	if bin == "" {
		t.Skip("set XRAY_E2E_BINARY to an xray binary to run this test")
	}
	localIP, ok := firstNonLoopbackIPv4()
	if !ok {
		t.Skip("no non-loopback IPv4 address available on this host")
	}

	const wantEmail = "manager-e2e-peer@example.com"
	const listenPort = 58716
	const inboundID = 5

	tcpEcho, tcpEchoAddr := startTCPEcho(t, localIP)
	defer tcpEcho.Close()

	serverPriv, serverPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate server keypair: %v", err)
	}
	clientPriv, clientPub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		t.Fatalf("generate client keypair: %v", err)
	}

	inst := amneziawg.Instance{
		Id:            inboundID,
		InterfaceName: "awgtest5",
		ListenPort:    listenPort,
		PrivateKey:    serverPriv,
		PublicKey:     serverPub,
		Address:       []string{"10.205.0.1/24"},
		MTU:           1420,
		Obfuscation: amneziawg.Obfuscation20{
			Jc: 4, Jmin: 40, Jmax: 70,
			S1: 20, S2: 30, S3: 20, S4: 20,
		},
		Peers: []amneziawg.Peer{{
			Email:      wantEmail,
			PublicKey:  clientPub,
			AllowedIPs: []string{"10.205.0.2/32"},
		}},
	}

	// A real xray-core process with a SOCKS5 inbound at exactly the port and
	// password ensureLocked will derive on its own for this instance --
	// SocksPassword() is cached (sync.Once), so calling it here first and
	// again inside Manager.Ensure below returns the identical value.
	socksPort := SOCKSPortForInbound(inboundID)
	password := SocksPassword()
	settingsJSON, err := SocksInboundSettings([]string{wantEmail}, password)
	if err != nil {
		t.Fatalf("SocksInboundSettings: %v", err)
	}
	var rawSettings any
	if err := json.Unmarshal(settingsJSON, &rawSettings); err != nil {
		t.Fatalf("unmarshal generated SOCKS5 settings: %v", err)
	}
	xrayCfg := map[string]any{
		"log": map[string]any{"loglevel": "debug"},
		"inbounds": []any{
			map[string]any{
				"listen":   "127.0.0.1",
				"port":     socksPort,
				"protocol": "socks",
				"settings": rawSettings,
				"tag":      "awg-e2e-manager",
			},
		},
		"outbounds": []any{
			map[string]any{"protocol": "freedom", "settings": map[string]any{}, "tag": "direct"},
		},
		"policy": map[string]any{
			"levels": map[string]any{
				"0": map[string]any{"statsUserUplink": true, "statsUserDownlink": true},
			},
		},
		"stats": map[string]any{},
	}
	cfgBytes, err := json.MarshalIndent(xrayCfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal xray config: %v", err)
	}
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(cfgPath, cfgBytes, 0o644); err != nil {
		t.Fatalf("write xray config: %v", err)
	}

	var xrayLog syncBuffer
	cmd := exec.Command(bin, "-c", cfgPath)
	cmd.Stdout = &xrayLog
	cmd.Stderr = &xrayLog
	if err := cmd.Start(); err != nil {
		t.Fatalf("start xray: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()
	waitForPort(t, socksPort)

	// A throwaway Manager, not the process-wide singleton, so this test
	// doesn't interact with any other test's state.
	m := &Manager{ifaces: map[int]*managed{}}
	defer m.StopAll()
	if err := m.Ensure(Desired{Instance: inst}); err != nil {
		t.Fatalf("Manager.Ensure: %v", err)
	}
	dev, _, ok := m.Lookup(inboundID)
	if !ok {
		t.Fatal("Lookup after Ensure: not found")
	}
	defer dev.Close() // StopAll would also do this; explicit for clarity

	clientTun, clientNet, err := netstack.CreateNetTUN(
		[]netip.Addr{netip.MustParseAddr("10.205.0.2")},
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

	const tcpMsg = "hello via Manager.Ensure's automatic relay wiring"
	dialDeadline := time.Now().Add(10 * time.Second)
	var conn net.Conn
	for {
		c, dialErr := clientNet.DialContext(t.Context(), "tcp", tcpEchoAddr.String())
		if dialErr == nil {
			conn = c
			break
		}
		if time.Now().After(dialDeadline) {
			t.Fatalf("client TCP dial via tunnel never succeeded: %v", dialErr)
		}
		time.Sleep(150 * time.Millisecond)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(tcpMsg)); err != nil {
		t.Fatalf("client TCP write: %v", err)
	}
	buf := make([]byte, len(tcpMsg))
	if _, err := readFull(conn, buf, 10*time.Second); err != nil {
		t.Fatalf("client TCP read: %v", err)
	}
	if string(buf) != tcpMsg {
		t.Errorf("TCP echo = %q, want %q", buf, tcpMsg)
	}

	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
	log := xrayLog.String()
	wantUp := fmt.Sprintf("user>>>%s>>>traffic>>>uplink", wantEmail)
	if !strings.Contains(log, wantUp) {
		t.Errorf("xray log missing uplink stats counter %q (Manager.Ensure's automatic relay wiring may not be attributing traffic correctly)\nfull log:\n%s", wantUp, log)
	}
}

// firstNonLoopbackIPv4 finds a real, locally-bound IPv4 address suitable as
// a relay-reachable test destination.
func firstNonLoopbackIPv4() (netip.Addr, bool) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return netip.Addr{}, false
	}
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() {
			continue
		}
		if v4 := ipNet.IP.To4(); v4 != nil {
			addr, ok := netip.AddrFromSlice(v4)
			if ok {
				return addr, true
			}
		}
	}
	return netip.Addr{}, false
}

func startTCPEcho(t *testing.T, addr netip.Addr) (io interface{ Close() error }, ap netip.AddrPort) {
	t.Helper()
	ln, err := net.Listen("tcp", net.JoinHostPort(addr.String(), "0"))
	if err != nil {
		t.Fatalf("start TCP echo listener: %v", err)
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				buf := make([]byte, 4096)
				for {
					n, err := c.Read(buf)
					if n > 0 {
						if _, werr := c.Write(buf[:n]); werr != nil {
							return
						}
					}
					if err != nil {
						return
					}
				}
			}()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port
	return ln, netip.AddrPortFrom(addr, uint16(port))
}

func startUDPEcho(t *testing.T, addr netip.Addr) (io interface{ Close() error }, ap netip.AddrPort) {
	t.Helper()
	pc, err := net.ListenPacket("udp", net.JoinHostPort(addr.String(), "0"))
	if err != nil {
		t.Fatalf("start UDP echo listener: %v", err)
	}
	go func() {
		buf := make([]byte, 4096)
		for {
			n, raddr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			if _, err := pc.WriteTo(buf[:n], raddr); err != nil {
				return
			}
		}
	}()
	port := pc.LocalAddr().(*net.UDPAddr).Port
	return pc, netip.AddrPortFrom(addr, uint16(port))
}

// readFull reads exactly len(buf) bytes or fails after timeout, since
// gonet.TCPConn (and net.Conn generally) may return short reads.
func readFull(r interface{ Read([]byte) (int, error) }, buf []byte, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	total := 0
	for total < len(buf) {
		if time.Now().After(deadline) {
			return total, fmt.Errorf("timed out after reading %d/%d bytes", total, len(buf))
		}
		n, err := r.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// syncBuffer is a concurrency-safe bytes buffer for capturing a subprocess's
// combined stdout/stderr while the test may read it from another goroutine.
type syncBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func waitForPort(t *testing.T, port int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("xray port %d did not open in time", port)
}
