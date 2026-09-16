package tuic

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func TestBuildSocks5ConnectRequest(t *testing.T) {
	// IPv4
	ip4 := net.ParseIP("1.2.3.4")
	req4 := buildSocks5ConnectRequest(&Address{Type: AddrTypeIPv4, IP: ip4, Port: 8080})
	if len(req4) != 10 || req4[0] != 0x05 || req4[1] != 0x01 || req4[3] != 0x01 {
		t.Fatalf("unexpected IPv4 CONNECT request: %x", req4)
	}
	if binary.BigEndian.Uint16(req4[8:10]) != 8080 {
		t.Fatalf("expected port 8080, got %d", binary.BigEndian.Uint16(req4[8:10]))
	}

	// IPv6
	ip6 := net.ParseIP("2001:db8::1")
	req6 := buildSocks5ConnectRequest(&Address{Type: AddrTypeIPv6, IP: ip6, Port: 443})
	if len(req6) != 22 || req6[3] != 0x04 {
		t.Fatalf("unexpected IPv6 CONNECT request: %x", req6)
	}
	if binary.BigEndian.Uint16(req6[20:22]) != 443 {
		t.Fatalf("expected port 443, got %d", binary.BigEndian.Uint16(req6[20:22]))
	}

	// Domain
	reqD := buildSocks5ConnectRequest(&Address{Type: AddrTypeDomain, Host: "example.com", Port: 80})
	if reqD == nil || reqD[3] != 0x03 || reqD[4] != byte(len("example.com")) {
		t.Fatalf("unexpected Domain CONNECT request: %x", reqD)
	}
	if binary.BigEndian.Uint16(reqD[len(reqD)-2:]) != 80 {
		t.Fatalf("expected port 80, got %d", binary.BigEndian.Uint16(reqD[len(reqD)-2:]))
	}

	// Nil target
	if buildSocks5ConnectRequest(nil) != nil {
		t.Fatalf("expected nil for nil target")
	}
}

func TestBuildSocks5UDPHeader(t *testing.T) {
	// IPv4
	ip4 := net.ParseIP("192.168.1.1")
	hdr4 := buildSocks5UDPHeader(&Address{Type: AddrTypeIPv4, IP: ip4, Port: 53})
	if len(hdr4) != 10 || hdr4[3] != 0x01 || binary.BigEndian.Uint16(hdr4[8:10]) != 53 {
		t.Fatalf("unexpected IPv4 UDP header: %x", hdr4)
	}

	// IPv6
	ip6 := net.ParseIP("::1")
	hdr6 := buildSocks5UDPHeader(&Address{Type: AddrTypeIPv6, IP: ip6, Port: 5353})
	if len(hdr6) != 22 || hdr6[3] != 0x04 || binary.BigEndian.Uint16(hdr6[20:22]) != 5353 {
		t.Fatalf("unexpected IPv6 UDP header: %x", hdr6)
	}

	// Domain
	hdrD := buildSocks5UDPHeader(&Address{Type: AddrTypeDomain, Host: "dns.google", Port: 53})
	if hdrD == nil || hdrD[3] != 0x03 || hdrD[4] != byte(len("dns.google")) {
		t.Fatalf("unexpected Domain UDP header: %x", hdrD)
	}

	// Nil target
	if buildSocks5UDPHeader(nil) != nil {
		t.Fatalf("expected nil for nil target")
	}
}

func TestCountingConn(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	var bytesRead atomic.Int64
	var bytesWritten atomic.Int64
	c := &CountingConn{
		Conn:         clientConn,
		bytesRead:    &bytesRead,
		bytesWritten: &bytesWritten,
	}

	go func() {
		buf := make([]byte, 100)
		n, _ := serverConn.Read(buf)
		_, _ = serverConn.Write(buf[:n])
	}()

	msg := []byte("hello counting conn")
	n, err := c.Write(msg)
	if err != nil || n != len(msg) {
		t.Fatalf("write failed: %v", err)
	}
	if bytesWritten.Load() != int64(len(msg)) {
		t.Fatalf("expected %d written, got %d", len(msg), bytesWritten.Load())
	}

	resp := make([]byte, 100)
	rn, err := c.Read(resp)
	if err != nil || rn != len(msg) {
		t.Fatalf("read failed: %v", err)
	}
	if bytesRead.Load() != int64(len(msg)) {
		t.Fatalf("expected %d read, got %d", len(msg), bytesRead.Load())
	}
}

func TestPipeBiDirectional(t *testing.T) {
	a1, a2 := net.Pipe()
	b1, b2 := net.Pipe()

	var up, down atomic.Int64

	done := make(chan struct{})
	go func() {
		PipeBiDirectional(a1, b1, &up, &down)
		close(done)
	}()

	// Send from a2 -> a1 -> b1 -> b2 (upload)
	testDataUp := []byte("upload stream test")
	go func() {
		_, _ = a2.Write(testDataUp)
	}()
	bufUp := make([]byte, len(testDataUp))
	_, err := io.ReadFull(b2, bufUp)
	if err != nil || !bytes.Equal(bufUp, testDataUp) {
		t.Fatalf("upload read failed: %v", err)
	}

	// Send from b2 -> b1 -> a1 -> a2 (download)
	testDataDown := []byte("download stream test")
	go func() {
		_, _ = b2.Write(testDataDown)
	}()
	bufDown := make([]byte, len(testDataDown))
	_, err = io.ReadFull(a2, bufDown)
	if err != nil || !bytes.Equal(bufDown, testDataDown) {
		t.Fatalf("download read failed: %v", err)
	}

	_ = a2.Close()
	_ = b2.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("PipeBiDirectional timed out waiting to finish")
	}

	if up.Load() < int64(len(testDataUp)) {
		t.Fatalf("expected at least %d up, got %d", len(testDataUp), up.Load())
	}
	if down.Load() < int64(len(testDataDown)) {
		t.Fatalf("expected at least %d down, got %d", len(testDataDown), down.Load())
	}
}

func startMockSocks5Server(t *testing.T, expectedUser, expectedPass string) (string, func()) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	stop := make(chan struct{})

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-stop:
					return
				default:
					return
				}
			}
			go handleMockSocksConn(conn, expectedUser, expectedPass)
		}
	}()

	return ln.Addr().String(), func() {
		close(stop)
		_ = ln.Close()
	}
}

func handleMockSocksConn(conn net.Conn, expectedUser, expectedPass string) {
	defer conn.Close()
	// Read greeting
	var greeting [4]byte
	if _, err := io.ReadFull(conn, greeting[:]); err != nil {
		return
	}
	// Select user/password auth (0x02)
	if _, err := conn.Write([]byte{0x05, 0x02}); err != nil {
		return
	}
	// Auth negotiation
	var authVer [2]byte
	if _, err := io.ReadFull(conn, authVer[:]); err != nil {
		return
	}
	uLen := int(authVer[1])
	user := make([]byte, uLen)
	if _, err := io.ReadFull(conn, user); err != nil {
		return
	}
	var pLen [1]byte
	if _, err := io.ReadFull(conn, pLen[:]); err != nil {
		return
	}
	pass := make([]byte, int(pLen[0]))
	if _, err := io.ReadFull(conn, pass); err != nil {
		return
	}

	if string(user) != expectedUser || string(pass) != expectedPass {
		_, _ = conn.Write([]byte{0x01, 0x01}) // auth failure
		return
	}
	_, _ = conn.Write([]byte{0x01, 0x00}) // auth success

	// Read command
	var cmdHdr [4]byte
	if _, err := io.ReadFull(conn, cmdHdr[:]); err != nil {
		return
	}
	cmd := cmdHdr[1]
	atyp := cmdHdr[3]

	// Read dest address
	switch atyp {
	case 0x01:
		var ip [4]byte
		_, _ = io.ReadFull(conn, ip[:])
	case 0x04:
		var ip [16]byte
		_, _ = io.ReadFull(conn, ip[:])
	case 0x03:
		var dLen [1]byte
		_, _ = io.ReadFull(conn, dLen[:])
		domain := make([]byte, dLen[0])
		_, _ = io.ReadFull(conn, domain)
	}
	var port [2]byte
	_, _ = io.ReadFull(conn, port[:])

	if cmd == 0x01 { // CONNECT
		// Send success reply: 0x05 0x00 0x00 0x01 (IPv4 127.0.0.1:0)
		_, _ = conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 127, 0, 0, 1, 0x1f, 0x90})
		// Echo server for testing
		_, _ = io.Copy(conn, conn)
	} else if cmd == 0x03 { // UDP ASSOCIATE
		// Bind a UDP listener for the mock
		u, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
		if err != nil {
			return
		}
		defer u.Close()
		bindAddr := u.LocalAddr().(*net.UDPAddr)
		bindPort := uint16(bindAddr.Port)

		resp := make([]byte, 10)
		resp[0] = 0x05
		resp[1] = 0x00
		resp[2] = 0x00
		resp[3] = 0x01
		copy(resp[4:8], bindAddr.IP.To4())
		binary.BigEndian.PutUint16(resp[8:10], bindPort)
		if _, err := conn.Write(resp); err != nil {
			return
		}

		// Read one UDP packet, echo it back
		go func() {
			buf := make([]byte, 2048)
			n, remoteAddr, err := u.ReadFrom(buf)
			if err != nil {
				return
			}
			_, _ = u.WriteTo(buf[:n], remoteAddr)
		}()

		// Keep conn open until closed
		buf := make([]byte, 1)
		_, _ = conn.Read(buf)
	}
}

func TestSocksRelayDialTCP(t *testing.T) {
	addr, cleanup := startMockSocks5Server(t, "user@test.com", "secretpass")
	defer cleanup()

	relay := &SocksRelay{
		Addr:     addr,
		Password: "secretpass",
	}

	target := &Address{
		Type: AddrTypeIPv4,
		IP:   net.ParseIP("93.184.216.34"),
		Port: 80,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := relay.DialTCP(ctx, "user@test.com", target)
	if err != nil {
		t.Fatalf("DialTCP failed: %v", err)
	}
	defer conn.Close()

	// Send echo payload
	msg := []byte("ping through socks")
	if _, err := conn.Write(msg); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	reply := make([]byte, len(msg))
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !bytes.Equal(reply, msg) {
		t.Fatalf("expected %q, got %q", msg, reply)
	}
}

func TestSocksRelayDialUDP(t *testing.T) {
	addr, cleanup := startMockSocks5Server(t, "user@test.com", "secretpass")
	defer cleanup()

	relay := &SocksRelay{
		Addr:     addr,
		Password: "secretpass",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	session, err := relay.DialUDP(ctx, "user@test.com")
	if err != nil {
		t.Fatalf("DialUDP failed: %v", err)
	}
	defer session.Close()

	target := &Address{
		Type: AddrTypeIPv4,
		IP:   net.ParseIP("8.8.8.8"),
		Port: 53,
	}
	payload := []byte("dns packet payload")

	n, err := session.Send(target, payload)
	if err != nil || n == 0 {
		t.Fatalf("Send failed: %v", err)
	}

	buf := make([]byte, 2048)
	recvAddr, recvPayload, err := session.Receive(buf)
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}
	if !bytes.Equal(recvPayload, payload) {
		t.Fatalf("expected payload %q, got %q", payload, recvPayload)
	}
	if recvAddr.IP.String() != "8.8.8.8" || recvAddr.Port != 53 {
		t.Fatalf("unexpected addr: %v", recvAddr)
	}
}
