package amneziawgnet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"

	"github.com/amnezia-vpn/amneziawg-go/v3/device"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

func verboseLoggerForTest(prefix string) *device.Logger {
	return device.NewLogger(device.LogLevelVerbose, prefix)
}

const (
	tunnelTestClientAddr  = "10.203.0.2"
	tunnelTestServerAddr  = "10.203.0.1"
	egressTestDialTimeout = 5 * time.Second
)

// pairedTunnel wires an outbound client device to an embedded server device
// over host UDP; the server stack hosts the far-end services under test.
type pairedTunnel struct {
	client   *Device
	server   *Device
	serverIP netip.Addr
}

func newPairedTunnelForTest(t *testing.T) *pairedTunnel {
	t.Helper()
	slog := verboseLoggerForTest("(tsrv) ")
	serverPriv, serverPub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatal(err)
	}
	clientPriv, clientPub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		t.Fatal(err)
	}
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listenPort := pc.LocalAddr().(*net.UDPAddr).Port
	pc.Close()

	obf := amneziawg.Obfuscation31{Jc: 4, Jmin: 40, Jmax: 70, S1: 20, S2: 30, S3: 20, S4: 20}
	serverInst := amneziawg.Instance{
		Id:            1,
		InterfaceName: "awg-dnstest",
		ListenPort:    listenPort,
		PrivateKey:    serverPriv,
		PublicKey:     serverPub,
		Address:       []string{tunnelTestServerAddr + "/24"},
		MTU:           1420,
		Obfuscation:   obf,
		Peers: []amneziawg.Peer{{
			PublicKey:  clientPub,
			AllowedIPs: []string{tunnelTestClientAddr + "/32"},
		}},
	}
	server, err := newUnconfiguredDevice(serverInst, DeviceOptions{Logger: slog})
	if err != nil {
		t.Fatalf("server device: %v", err)
	}
	t.Cleanup(server.Close)

	// Server Up before client exists: the first handshake fires at
	// ConfigureClient; a missed initiation costs a 5s REKEY_TIMEOUT.
	if err := server.Configure(serverInst, DeviceOptions{Logger: slog}); err != nil {
		t.Fatalf("server Configure: %v", err)
	}

	clientInst := amneziawg.OutboundInstance{
		Tag:         "awg-dom-test",
		Address:     []string{tunnelTestClientAddr + "/32"},
		MTU:         1420,
		PrivateKey:  clientPriv,
		Obfuscation: obf,
		Peers: []amneziawg.OutboundPeer{{
			PublicKey:  serverPub,
			Endpoint:   net.JoinHostPort("127.0.0.1", strconv.Itoa(listenPort)),
			AllowedIPs: []string{"0.0.0.0/0", "::/0"},
			KeepAlive:  1,
		}},
	}
	clog := verboseLoggerForTest("(tcli) ")
	client, err := newUnconfiguredClientDevice(clientInst, DeviceOptions{Logger: clog})
	if err != nil {
		t.Fatalf("client device: %v", err)
	}
	if err := client.ConfigureClient(clientInst, DeviceOptions{Logger: clog}); err != nil {
		client.Close()
		t.Fatalf("ConfigureClient: %v", err)
	}
	t.Cleanup(client.Close)

	return &pairedTunnel{
		client:   client,
		server:   server,
		serverIP: netip.MustParseAddr(tunnelTestServerAddr),
	}
}

func registerEgressDeviceForTest(t *testing.T, dev *Device) {
	t.Helper()
	srv := GetEgressServer()
	srv.SetStack("awg-dom-test", dev)
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.DeleteStack("awg-dom-test") })
}

// startTunnelDNS answers A queries from INSIDE the server's netstack;
// reaching it proves DNS rode the tunnel, not the host resolver.
func (p *pairedTunnel) startDNS(t *testing.T, answer netip.Addr) chan string {
	t.Helper()
	ln, err := gonet.DialUDP(p.server.Stack, &tcpip.FullAddress{NIC: 1, Port: 53}, nil, ipv4.ProtocolNumber)
	if err != nil {
		t.Fatalf("bind fake dns in server stack: %v", err)
	}
	got := make(chan string, 8)
	go func() {
		defer ln.Close()
		buf := make([]byte, 512)
		for {
			n, from, rerr := ln.ReadFrom(buf)
			if rerr != nil {
				return
			}
			q := buf[:n]
			if name := dnsQuestionName(q); name != "" {
				select {
				case got <- name:
				default:
				}
			}
			if resp := buildARecordReply(q, answer); resp != nil {
				if _, werr := ln.WriteTo(resp, from); werr != nil {
					return
				}
			}
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return got
}

func (p *pairedTunnel) overrideDNS(t *testing.T, answer netip.Addr) chan string {
	t.Helper()
	srv := GetEgressServer()
	prev := srv.currentDNSServer()
	srv.SetDNSServer(net.JoinHostPort(p.serverIP.String(), "53"))
	t.Cleanup(func() { srv.SetDNSServer(prev) })
	resetTunnelDNSCacheForTest()
	return p.startDNS(t, answer)
}

func resetTunnelDNSCacheForTest() {
	tunnelDNSCache.mu.Lock()
	tunnelDNSCache.m = map[string]tunnelDNSCacheEntry{}
	tunnelDNSCache.mu.Unlock()
}

func dnsQuestionName(q []byte) string {
	if len(q) < 12 {
		return ""
	}
	i := 12
	var parts []byte
	for i < len(q) {
		l := int(q[i])
		i++
		if l == 0 {
			break
		}
		if i+l > len(q) || l > 63 {
			return ""
		}
		parts = append(parts, q[i:i+l]...)
		parts = append(parts, '.')
		i += l
	}
	for len(parts) > 0 && parts[len(parts)-1] == '.' {
		parts = parts[:len(parts)-1]
	}
	return string(parts)
}

func buildARecordReply(q []byte, answer netip.Addr) []byte {
	if len(q) < 17 {
		return nil
	}
	out := make([]byte, 0, len(q)+16)
	header := make([]byte, 12)
	copy(header[0:2], q[0:2])
	header[2] = 0x81 // QR=1 RD=1
	header[3] = 0x80 // RA=1 RCODE=0
	binary.BigEndian.PutUint16(header[4:], 1)
	binary.BigEndian.PutUint16(header[6:], 1)
	out = append(out, header...)
	end := len(q)
	for end >= 5 && q[end-4] == 0 && q[end-3] == 0 && q[end-2] == 0 && q[end-1] == 0 {
		end -= 4
	}
	out = append(out, q[12:end]...)
	if answer.Is4() {
		a := answer.As4()
		rr := make([]byte, 16)
		rr[0], rr[1] = 0xc0, 0x0c
		binary.BigEndian.PutUint16(rr[2:], 1)  // Type A
		binary.BigEndian.PutUint16(rr[4:], 1)  // IN
		binary.BigEndian.PutUint32(rr[6:], 30) // TTL
		binary.BigEndian.PutUint16(rr[10:], 4)
		copy(rr[12:], a[:])
		out = append(out, rr...)
	} else if answer.Is6() {
		a16 := answer.As16()
		rr := make([]byte, 28)
		rr[0], rr[1] = 0xc0, 0x0c
		binary.BigEndian.PutUint16(rr[2:], 28) // Type AAAA
		binary.BigEndian.PutUint16(rr[4:], 1)  // IN
		binary.BigEndian.PutUint32(rr[6:], 30) // TTL
		binary.BigEndian.PutUint16(rr[10:], 16)
		copy(rr[12:], a16[:])
		out = append(out, rr...)
	}
	return out
}

func socksAuth(t *testing.T, ctl net.Conn) {
	t.Helper()
	ctl.SetDeadline(time.Now().Add(egressTestDialTimeout))
	if _, err := ctl.Write([]byte{0x05, 0x02, 0x00, 0x02}); err != nil {
		t.Fatal(err)
	}
	r := make([]byte, 2)
	if _, err := io.ReadFull(ctl, r); err != nil {
		t.Fatalf("greeting read: %v", err)
	}
	user, pass := "awg-dom-test", SocksPassword()
	req := make([]byte, 0, 3+len(user)+len(pass))
	req = append(req, 0x01, byte(len(user)))
	req = append(req, user...)
	req = append(req, byte(len(pass)))
	req = append(req, pass...)
	if _, err := ctl.Write(req); err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 2)
	if _, err := io.ReadFull(ctl, auth); err != nil || auth[1] != 0x00 {
		t.Fatalf("auth rejected: %v %v", err, auth)
	}
}

func TestEgressGreetingRejectsNoAuthClient(t *testing.T) {
	tun := newPairedTunnelForTest(t)
	registerEgressDeviceForTest(t, tun.client)

	ctl, err := (&net.Dialer{Timeout: egressTestDialTimeout}).Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(EgressBasePort)))
	if err != nil {
		t.Fatal(err)
	}
	defer ctl.Close()
	ctl.SetDeadline(time.Now().Add(egressTestDialTimeout))
	// Client offers only NO-AUTH; server must answer 0xFF.
	if _, err := ctl.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		t.Fatal(err)
	}
	r := make([]byte, 2)
	if _, err := io.ReadFull(ctl, r); err != nil {
		t.Fatalf("greeting read: %v", err)
	}
	if r[0] != 0x05 || r[1] != 0xFF {
		t.Fatalf("greeting reply = %v, want 05 FF (auth required)", r)
	}
}

func TestEgressConnectDomainResolvesThroughTunnel(t *testing.T) {
	tun := newPairedTunnelForTest(t)
	registerEgressDeviceForTest(t, tun.client)
	// Resolving to the server's own tunnel address makes the follow-up dial
	// fail fast (nothing listens on :80), while proving resolution happened.
	gotQuery := tun.overrideDNS(t, tun.serverIP)

	ctl, err := (&net.Dialer{Timeout: egressTestDialTimeout}).Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(EgressBasePort)))
	if err != nil {
		t.Fatal(err)
	}
	defer ctl.Close()

	socksAuth(t, ctl)
	name := "example.internal"
	req := make([]byte, 0, 7+len(name))
	req = append(req, 0x05, 0x01, 0x00, 0x03, byte(len(name)))
	req = append(req, name...)
	req = append(req, 0x00, 0x50)
	if _, err := ctl.Write(req); err != nil {
		t.Fatal(err)
	}

	select {
	case queried := <-gotQuery:
		if len(queried) < len(name) || queried[:len(name)] != name {
			t.Fatalf("resolver queried %q, want prefix %q -- DNS did not ride the tunnel", queried, name)
		}
	case <-time.After(egressTestDialTimeout):
		t.Fatal("no DNS query reached the in-tunnel resolver")
	}

	reply := make([]byte, 10)
	ctl.SetDeadline(time.Now().Add(egressTestDialTimeout))
	if _, err := io.ReadFull(ctl, reply); err != nil {
		t.Fatalf("read reply: %v", err)
	}
	if reply[1] == 0x00 {
		t.Fatal("unexpected success: nothing should be listening on the resolved address")
	}
}

func TestEgressUDPDatagramDomainForwardedIntoTunnel(t *testing.T) {
	tun := newPairedTunnelForTest(t)
	registerEgressDeviceForTest(t, tun.client)
	gotQuery := tun.overrideDNS(t, tun.serverIP)

	in, err := gonet.DialUDP(tun.server.Stack, &tcpip.FullAddress{NIC: 1, Port: 9999}, nil, ipv4.ProtocolNumber)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	ctl, err := (&net.Dialer{Timeout: egressTestDialTimeout}).Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(EgressBasePort)))
	if err != nil {
		t.Fatal(err)
	}
	defer ctl.Close()

	socksAuth(t, ctl)
	if _, err := ctl.Write([]byte{0x05, 0x03, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 10)
	if _, err := io.ReadFull(ctl, reply); err != nil || reply[1] != 0x00 {
		t.Fatalf("associate failed: %v %v", err, reply)
	}
	bindPort := binary.BigEndian.Uint16(reply[8:10])

	udp, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: int(bindPort)})
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	udp.SetDeadline(time.Now().Add(egressTestDialTimeout))

	// Plain-IP control datagram isolates domain parsing from transport.
	// Retried to avoid warmup race on slow -race runners.
	ctrl := []byte{0x00, 0x00, 0x00, 0x01, 10, 203, 0, 1, 0x27, 0x0f, 'c', 't', 'r', 'l'}
	rcv := make([]byte, 64)
	var nr int
	var rerr error
	for attempt := 0; attempt < 3; attempt++ {
		if _, err := udp.Write(ctrl); err != nil {
			t.Fatal(err)
		}
		in.SetReadDeadline(time.Now().Add(3 * time.Second))
		nr, _, rerr = in.ReadFrom(rcv)
		if rerr == nil {
			break
		}
	}
	if rerr != nil {
		t.Fatalf("CONTROL datagram never reached the tunnel target: %v", rerr)
	}
	if string(rcv[:nr]) != "ctrl" {
		t.Fatalf("control payload = %q", rcv[:nr])
	}

	name := "quic.internal"
	dgram := make([]byte, 0, 5+len(name)+2+4)
	dgram = append(dgram, 0x00, 0x00, 0x00, 0x03, byte(len(name)))
	dgram = append(dgram, name...)
	dgram = append(dgram, 0x27, 0x0f)
	dgram = append(dgram, 'p', 'i', 'n', 'g')

	var queried string
	for attempt := 0; attempt < 3 && queried == ""; attempt++ {
		if _, err := udp.Write(dgram); err != nil {
			t.Fatal(err)
		}
		select {
		case q := <-gotQuery:
			queried = q
		case <-time.After(1500 * time.Millisecond):
		}
	}
	if len(queried) < len(name) || queried[:len(name)] != name {
		t.Fatalf("resolver queried %q, want prefix %q -- DNS did not ride the tunnel", queried, name)
	}

	in.SetReadDeadline(time.Now().Add(3 * time.Second))
	nr, _, rerr = in.ReadFrom(rcv)
	if rerr != nil {
		t.Fatalf("domain datagram never reached the tunnel target: %v", rerr)
	}
	if nr < 4 || string(rcv[:4]) != "ping" {
		t.Fatalf("payload = %q (n=%d)", rcv[:nr], nr)
	}
}

// TestEgressUDPDatagramDomainInterleavedClients pins the round-6 resolver
// race fix: two clients interleave domain datagrams, so each resolver
// goroutine must observe the client address handed to it at spawn
// (pass-by-value) rather than a shared variable rewritten by the reader
// loop while a resolve is parked.
func TestEgressUDPDatagramDomainInterleavedClients(t *testing.T) {
	tun := newPairedTunnelForTest(t)
	registerEgressDeviceForTest(t, tun.client)
	gotQuery := tun.overrideDNS(t, tun.serverIP)

	dialUDP := func() *net.UDPConn {
		ctl, err := (&net.Dialer{Timeout: egressTestDialTimeout}).Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(EgressBasePort)))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { ctl.Close() })
		socksAuth(t, ctl)
		if _, err := ctl.Write([]byte{0x05, 0x03, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
			t.Fatal(err)
		}
		reply := make([]byte, 10)
		if _, err := io.ReadFull(ctl, reply); err != nil || reply[1] != 0x00 {
			t.Fatalf("associate failed: %v %v", err, reply)
		}
		bindPort := binary.BigEndian.Uint16(reply[8:10])
		udp, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: int(bindPort)})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { udp.Close() })
		udp.SetDeadline(time.Now().Add(egressTestDialTimeout))
		return udp
	}

	name := func(i int) string { return fmt.Sprintf("interleaved-%d.internal", i) }
	dgram := func(i int, payload string) []byte {
		n := name(i)
		d := make([]byte, 0, 5+len(n)+2+len(payload))
		d = append(d, 0x00, 0x00, 0x00, 0x03, byte(len(n)))
		d = append(d, n...)
		d = append(d, 0x27, 0x0f)
		return append(d, payload...)
	}

	a, b := dialUDP(), dialUDP()
	// Interleave: each client sends twice, forcing reader-loop writes to
	// `client` (pre-fix) to overlap with parked resolver goroutines. Names
	// are unique per datagram so the tunnel DNS cache cannot dedupe them.
	seq := 0
	for _, c := range []*net.UDPConn{a, b, a, b} {
		if _, err := c.Write(dgram(seq, "x")); err != nil {
			t.Fatal(err)
		}
		seq++
		time.Sleep(50 * time.Millisecond)
	}
	for i := 0; i < 4; i++ {
		select {
		case q := <-gotQuery:
			if !strings.HasPrefix(q, "interleaved-") {
				t.Fatalf("resolver queried %q, want an interleaved-* name", q)
			}
		case <-time.After(4 * time.Second):
			t.Fatalf("only %d/4 tunnel DNS queries observed", i)
		}
	}
}

func TestDefaultDNSFor(t *testing.T) {
	v4 := netip.MustParseAddr("10.8.0.2")
	v6 := netip.MustParseAddr("2001:db8::2")

	if got := defaultDNSFor([]netip.Addr{v4}); got != DefaultTunnelDNSServer {
		t.Errorf("defaultDNSFor(v4) = %q, want %q", got, DefaultTunnelDNSServer)
	}
	if got := defaultDNSFor([]netip.Addr{v4, v6}); got != DefaultTunnelDNSServer {
		t.Errorf("defaultDNSFor(dual) = %q, want %q", got, DefaultTunnelDNSServer)
	}
	if got := defaultDNSFor([]netip.Addr{v6}); got != DefaultTunnelDNSServerV6 {
		t.Errorf("defaultDNSFor(v6-only) = %q, want %q", got, DefaultTunnelDNSServerV6)
	}
	if got := defaultDNSFor(nil); got != DefaultTunnelDNSServer {
		t.Errorf("defaultDNSFor(nil) = %q, want %q", got, DefaultTunnelDNSServer)
	}
}

func TestParseDatagramDomainHeader(t *testing.T) {
	hdr := []byte{0, 0, 0, 0x03, 4, 'a', 'b', '.', 'd', 0x00, 0x35, 'x'}
	name, port, hdrLen, err := parseDatagramDomainHeader(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if name != "ab.d" || port != 53 || hdrLen != 11 {
		t.Fatalf("name=%q port=%d hdrLen=%d", name, port, hdrLen)
	}
	truncated := []byte{0, 0, 0, 0x03, 200, 'a'}
	if _, _, _, err := parseDatagramDomainHeader(truncated); err == nil {
		t.Fatal("truncated domain accepted")
	}
	empty := []byte{0, 0, 0, 0x03, 0, 0x00, 0x35}
	if _, _, _, err := parseDatagramDomainHeader(empty); err == nil {
		t.Fatal("empty domain accepted")
	}
}

func TestReadSocksRequestTargetKeepsHostnameUnresolved(t *testing.T) {
	payload := append([]byte{byte(len("invalid."))}, []byte("invalid.")...)
	payload = append(payload, 0x01, 0xbb)
	tr, err := readSocksRequestTarget(bytes.NewReader(payload), 0x03)
	if err != nil {
		t.Fatalf("domain request rejected: %v", err)
	}
	if tr.host != "invalid." || tr.port != 443 || tr.ip.IsValid() {
		t.Fatalf("target = %+v", tr)
	}
}

func TestTunnelDNSCache_ScopedPerTagAndServer(t *testing.T) {
	resetTunnelDNSCacheForTest()
	tagA, tagB := "out-a", "out-b"
	dns1, dns2 := "1.1.1.1:53", "8.8.8.8:53"
	host := "example.com"

	addrA := netip.MustParseAddr("10.0.0.1")
	addrB := netip.MustParseAddr("10.0.0.2")

	keyA := dnsCacheKey(tagA, dns1, host)
	keyB := dnsCacheKey(tagB, dns1, host)
	keyA2 := dnsCacheKey(tagA, dns2, host)

	tunnelDNSCache.mu.Lock()
	tunnelDNSCache.m[keyA] = tunnelDNSCacheEntry{addr: addrA, exp: time.Now().Add(time.Hour)}
	tunnelDNSCache.m[keyB] = tunnelDNSCacheEntry{addr: addrB, exp: time.Now().Add(time.Hour)}
	tunnelDNSCache.mu.Unlock()

	tunnelDNSCache.mu.Lock()
	eA, okA := tunnelDNSCache.m[keyA]
	eB, okB := tunnelDNSCache.m[keyB]
	_, okA2 := tunnelDNSCache.m[keyA2]
	tunnelDNSCache.mu.Unlock()

	if !okA || eA.addr != addrA {
		t.Fatalf("tagA cache entry mismatch: %v, %v", okA, eA)
	}
	if !okB || eB.addr != addrB {
		t.Fatalf("tagB cache entry mismatch: %v, %v", okB, eB)
	}
	if okA2 {
		t.Fatal("key with different DNS server should not match")
	}

	flushTunnelDNSCacheForTag(tagA)
	tunnelDNSCache.mu.Lock()
	_, okAAfter := tunnelDNSCache.m[keyA]
	_, okBAfter := tunnelDNSCache.m[keyB]
	tunnelDNSCache.mu.Unlock()

	if okAAfter {
		t.Fatal("tagA entry should be flushed")
	}
	if !okBAfter {
		t.Fatal("tagB entry should survive flush of tagA")
	}
}
