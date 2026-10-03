package tuic

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quic-go/quic-go"
)

func generateTestCert(t *testing.T) (certPEM, keyPEM []byte) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test TUIC Server"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	return certPEM, keyPEM
}

func TestServerTCPConnectE2E(t *testing.T) {
	for _, controller := range []string{"bbr", "cubic", "new_reno"} {
		t.Run(controller, func(t *testing.T) {
			testServerTCPConnectE2E(t, controller)
		})
	}
}

func testServerTCPConnectE2E(t *testing.T, controller string) {
	certPEM, keyPEM := generateTestCert(t)

	// Start mock SOCKS5 server on loopback
	socksAddr, socksCleanup := startMockSocks5Server(t, "alice@example.com", "mock-socks-pass")
	defer socksCleanup()

	testUUID := uuid.New()
	testPassword := "secret-client-password"

	inst := Instance{
		Id:                    1,
		Tag:                   "tuic-test",
		Listen:                "127.0.0.1",
		Port:                  0,
		Certificate:           string(certPEM),
		PrivateKey:            string(keyPEM),
		CongestionControl:     controller,
		ALPN:                  []string{"h3"},
		MaxIdleTime:           5,
		AuthenticationTimeout: 2,
		Clients: []TuicClientSettings{
			{
				UUID:     testUUID.String(),
				Password: testPassword,
				Email:    "alice@example.com",
			},
		},
	}

	relay := &SocksRelay{
		Addr:     socksAddr,
		Password: "mock-socks-pass",
	}

	server, err := NewServer(inst, relay)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Server.Start failed: %v", err)
	}
	defer server.Close()

	serverAddr := server.packetConn.LocalAddr().String()

	// Connect client to TUIC server via QUIC
	clientTLS := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"h3"},
	}
	quicConfig := &quic.Config{
		EnableDatagrams: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := quic.DialAddr(ctx, serverAddr, clientTLS, quicConfig)
	if err != nil {
		t.Fatalf("quic.DialAddr failed: %v", err)
	}
	defer conn.CloseWithError(0, "")

	// 1. Authenticate client on a uni stream
	tlsState := conn.ConnectionState().TLS
	token, err := tlsState.ExportKeyingMaterial(string(testUUID[:]), []byte(testPassword), 32)
	if err != nil {
		t.Fatalf("ExportKeyingMaterial failed: %v", err)
	}

	uniStream, err := conn.OpenUniStreamSync(ctx)
	if err != nil {
		t.Fatalf("OpenUniStreamSync failed: %v", err)
	}
	// Send: [VER (0x05)][0x00][UUID (16)][TOKEN (32)]
	authPayload := make([]byte, 2+16+32)
	authPayload[0] = ProtocolVersion
	authPayload[1] = CmdAuthenticate
	copy(authPayload[2:18], testUUID[:])
	copy(authPayload[18:50], token)

	if _, err := uniStream.Write(authPayload); err != nil {
		t.Fatalf("write auth payload failed: %v", err)
	}
	_ = uniStream.Close()

	// 2. Open bidirectional stream for TCP Connect
	biStream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		t.Fatalf("OpenStreamSync failed: %v", err)
	}
	defer biStream.Close()

	// Send: [VER (0x05)][0x01][ADDR]
	target := &Address{
		Type: AddrTypeIPv4,
		IP:   net.ParseIP("1.1.1.1"),
		Port: 80,
	}
	var connectBuf bytes.Buffer
	connectBuf.WriteByte(ProtocolVersion)
	connectBuf.WriteByte(CmdConnect)
	if err := WriteAddress(&connectBuf, target); err != nil {
		t.Fatalf("WriteAddress failed: %v", err)
	}
	if _, err := biStream.Write(connectBuf.Bytes()); err != nil {
		t.Fatalf("write connect cmd failed: %v", err)
	}

	// 3. Send test data and read echo response back through SOCKS5 bridge
	testMsg := []byte("ping pong over native go tuic!")
	if _, err := biStream.Write(testMsg); err != nil {
		t.Fatalf("write test message failed: %v", err)
	}

	recvBuf := make([]byte, len(testMsg))
	if _, err := io.ReadFull(biStream, recvBuf); err != nil {
		t.Fatalf("read echo failed: %v", err)
	}

	if !bytes.Equal(recvBuf, testMsg) {
		t.Fatalf("expected %q, got %q", testMsg, recvBuf)
	}

	// 4. Verify traffic was recorded for alice@example.com
	activeEmails := server.GetActiveEmails(10 * time.Second)
	if len(activeEmails) == 0 || activeEmails[0] != "alice@example.com" {
		t.Fatalf("expected active email alice@example.com, got %v", activeEmails)
	}

	deltas := server.CollectClientTraffic()
	if len(deltas) == 0 {
		t.Fatalf("expected traffic deltas, got none")
	}
	if deltas[0].Email != "alice@example.com" || deltas[0].Up < int64(len(testMsg)) || deltas[0].Down < int64(len(testMsg)) {
		t.Fatalf("unexpected traffic deltas: %+v", deltas[0])
	}
}

func TestServerUDPDatagramE2E(t *testing.T) {
	for _, controller := range []string{"bbr", "cubic", "new_reno"} {
		t.Run(controller, func(t *testing.T) {
			testServerUDPDatagramE2E(t, controller)
		})
	}
}

func testServerUDPDatagramE2E(t *testing.T, controller string) {
	certPEM, keyPEM := generateTestCert(t)

	socksAddr, socksCleanup := startMockSocks5Server(t, "bob@example.com", "mock-socks-pass")
	defer socksCleanup()

	testUUID := uuid.New()
	testPassword := "secret-bob-password"

	inst := Instance{
		Id:                    2,
		Tag:                   "tuic-udp-test",
		Listen:                "127.0.0.1",
		Port:                  0,
		Certificate:           string(certPEM),
		PrivateKey:            string(keyPEM),
		CongestionControl:     controller,
		ALPN:                  []string{"h3"},
		MaxIdleTime:           5,
		AuthenticationTimeout: 2,
		Clients: []TuicClientSettings{
			{
				UUID:     testUUID.String(),
				Password: testPassword,
				Email:    "bob@example.com",
			},
		},
	}

	relay := &SocksRelay{
		Addr:     socksAddr,
		Password: "mock-socks-pass",
	}

	server, err := NewServer(inst, relay)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Server.Start failed: %v", err)
	}
	defer server.Close()

	serverAddr := server.packetConn.LocalAddr().String()

	clientTLS := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"h3"},
	}
	quicConfig := &quic.Config{
		EnableDatagrams: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := quic.DialAddr(ctx, serverAddr, clientTLS, quicConfig)
	if err != nil {
		t.Fatalf("quic.DialAddr failed: %v", err)
	}
	defer conn.CloseWithError(0, "")

	// 1. Authenticate via uni stream
	tlsState := conn.ConnectionState().TLS
	token, err := tlsState.ExportKeyingMaterial(string(testUUID[:]), []byte(testPassword), 32)
	if err != nil {
		t.Fatalf("ExportKeyingMaterial failed: %v", err)
	}

	uniStream, err := conn.OpenUniStreamSync(ctx)
	if err != nil {
		t.Fatalf("OpenUniStreamSync failed: %v", err)
	}
	authPayload := make([]byte, 2+16+32)
	authPayload[0] = ProtocolVersion
	authPayload[1] = CmdAuthenticate
	copy(authPayload[2:18], testUUID[:])
	copy(authPayload[18:50], token)
	if _, err := uniStream.Write(authPayload); err != nil {
		t.Fatalf("write auth payload failed: %v", err)
	}
	_ = uniStream.Close()

	// 2. Send UDP datagram
	target := &Address{
		Type: AddrTypeIPv4,
		IP:   net.ParseIP("8.8.8.8"),
		Port: 53,
	}
	udpMsg := bytes.Repeat([]byte("d"), 1300)

	// Give a tiny moment for auth to register
	time.Sleep(50 * time.Millisecond)

	fragmentTotal := (len(udpMsg) + maxDatagramFragmentSize - 1) / maxDatagramFragmentSize
	for i := 0; i < fragmentTotal; i++ {
		start := i * maxDatagramFragmentSize
		end := min(start+maxDatagramFragmentSize, len(udpMsg))
		addr := (*Address)(nil)
		if i == 0 {
			addr = target
		}
		var frame bytes.Buffer
		if err := WritePacket(&frame, 100, 1, uint8(fragmentTotal), uint8(i), addr, udpMsg[start:end]); err != nil {
			t.Fatalf("WritePacket failed: %v", err)
		}
		if err := conn.SendDatagram(frame.Bytes()); err != nil {
			t.Fatalf("SendDatagram failed: %v", err)
		}
	}

	// 3. Receive and reassemble the echo reply via datagrams.
	replyReassembler := newPacketReassembler(1500)
	var replyPayload []byte
	for replyPayload == nil {
		recvDgram, err := conn.ReceiveDatagram(ctx)
		if err != nil {
			t.Fatalf("ReceiveDatagram failed: %v", err)
		}
		if len(recvDgram) < 2 || recvDgram[0] != ProtocolVersion || recvDgram[1] != CmdPacket {
			t.Fatalf("unexpected datagram reply: %x", recvDgram)
		}
		pktReader := bytes.NewReader(recvDgram[2:])
		hdr, err := ReadPacketHeader(pktReader)
		if err != nil {
			t.Fatalf("ReadPacketHeader failed: %v", err)
		}
		fragment, err := readPacketPayload(pktReader, hdr)
		if err != nil || pktReader.Len() != 0 {
			t.Fatalf("read reply payload failed: %v", err)
		}
		_, assembled, complete := replyReassembler.feed(packetTransportDatagram, hdr, fragment)
		if complete {
			replyPayload = assembled
		}
	}

	if !bytes.Equal(replyPayload, udpMsg) {
		t.Fatalf("expected %q, got %q", udpMsg, replyPayload)
	}

	// 4. Verify traffic
	deltas := server.CollectClientTraffic()
	if len(deltas) == 0 {
		t.Fatalf("expected traffic deltas, got none")
	}
	if deltas[0].Email != "bob@example.com" || deltas[0].Up < int64(len(udpMsg)) || deltas[0].Down < int64(len(udpMsg)) {
		t.Fatalf("unexpected traffic deltas: %+v", deltas[0])
	}
}

func TestServerUDPStreamE2E(t *testing.T) {
	for _, controller := range []string{"bbr", "cubic", "new_reno"} {
		t.Run(controller, func(t *testing.T) {
			testServerUDPStreamE2E(t, controller)
		})
	}
}

func testServerUDPStreamE2E(t *testing.T, controller string) {
	certPEM, keyPEM := generateTestCert(t)
	socksAddr, socksCleanup := startMockSocks5Server(t, "stream@example.com", "mock-socks-pass")
	defer socksCleanup()

	testUUID := uuid.New()
	testPassword := "secret-stream-password"
	server, err := NewServer(Instance{
		Id:                    3,
		Tag:                   "tuic-udp-stream-test",
		Listen:                "127.0.0.1",
		Certificate:           string(certPEM),
		PrivateKey:            string(keyPEM),
		ALPN:                  []string{"h3"},
		MaxIdleTime:           5,
		AuthenticationTimeout: 2,
		MaxUdpRelayPacketSize: maxUdpRelayPacketSize,
		CongestionControl:     controller,
		Clients: []TuicClientSettings{{
			UUID:     testUUID.String(),
			Password: testPassword,
			Email:    "stream@example.com",
		}},
	}, &SocksRelay{Addr: socksAddr, Password: "mock-socks-pass"})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	if err := server.Start(); err != nil {
		t.Fatalf("Server.Start failed: %v", err)
	}
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := quic.DialAddr(ctx, server.packetConn.LocalAddr().String(), &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"h3"},
	}, &quic.Config{EnableDatagrams: true})
	if err != nil {
		t.Fatalf("quic.DialAddr failed: %v", err)
	}
	defer conn.CloseWithError(0, "")

	tlsState := conn.ConnectionState().TLS
	token, err := tlsState.ExportKeyingMaterial(string(testUUID[:]), []byte(testPassword), 32)
	if err != nil {
		t.Fatalf("ExportKeyingMaterial failed: %v", err)
	}
	authStream, err := conn.OpenUniStreamSync(ctx)
	if err != nil {
		t.Fatalf("OpenUniStreamSync for authentication failed: %v", err)
	}
	authPayload := make([]byte, 2+16+32)
	authPayload[0] = ProtocolVersion
	authPayload[1] = CmdAuthenticate
	copy(authPayload[2:18], testUUID[:])
	copy(authPayload[18:], token)
	if _, err := authStream.Write(authPayload); err != nil {
		t.Fatalf("write authentication payload failed: %v", err)
	}
	if err := authStream.Close(); err != nil {
		t.Fatalf("close authentication stream failed: %v", err)
	}

	target := &Address{Type: AddrTypeIPv4, IP: net.ParseIP("8.8.8.8"), Port: 53}
	udpMsg := bytes.Repeat([]byte("s"), 8500)
	fragmentTotal := (len(udpMsg) + maxStreamFragmentSize - 1) / maxStreamFragmentSize
	for i := 0; i < fragmentTotal; i++ {
		start := i * maxStreamFragmentSize
		end := min(start+maxStreamFragmentSize, len(udpMsg))
		addr := (*Address)(nil)
		if i == 0 {
			addr = target
		}
		var frame bytes.Buffer
		if err := WritePacket(&frame, 300, 1, uint8(fragmentTotal), uint8(i), addr, udpMsg[start:end]); err != nil {
			t.Fatalf("WritePacket failed: %v", err)
		}
		packetStream, err := conn.OpenUniStreamSync(ctx)
		if err != nil {
			t.Fatalf("OpenUniStreamSync for packet failed: %v", err)
		}
		if _, err := packetStream.Write(frame.Bytes()); err != nil {
			t.Fatalf("write packet frame failed: %v", err)
		}
		if err := packetStream.Close(); err != nil {
			t.Fatalf("close packet stream failed: %v", err)
		}
	}

	replyReassembler := newPacketReassembler(maxUdpRelayPacketSize)
	var reply []byte
	for reply == nil {
		responseStream, err := conn.AcceptUniStream(ctx)
		if err != nil {
			t.Fatalf("AcceptUniStream for response failed: %v", err)
		}
		_, command, err := ReadCommand(responseStream)
		if err != nil {
			t.Fatalf("read response command: %v", err)
		}
		if command != CmdPacket {
			t.Fatalf("response command = %d, want %d", command, CmdPacket)
		}
		hdr, err := ReadPacketHeader(responseStream)
		if err != nil {
			t.Fatalf("ReadPacketHeader failed: %v", err)
		}
		fragment, err := readPacketPayload(responseStream, hdr)
		if err != nil {
			t.Fatalf("read response payload failed: %v", err)
		}
		_, assembled, complete := replyReassembler.feed(packetTransportStream, hdr, fragment)
		if complete {
			reply = assembled
		}
	}
	if !bytes.Equal(reply, udpMsg) {
		t.Fatalf("stream response size = %d, want %d", len(reply), len(udpMsg))
	}
}

func TestNewServerRejectsOversizedMaxUdpRelayPacketSize(t *testing.T) {
	certPEM, keyPEM := generateTestCert(t)
	_, err := NewServer(Instance{
		Listen:                "127.0.0.1",
		Port:                  8443,
		Certificate:           string(certPEM),
		PrivateKey:            string(keyPEM),
		MaxUdpRelayPacketSize: maxLegacyUdpRelayPacketSize + 1,
	}, &SocksRelay{Addr: "127.0.0.1:1"})
	if err == nil {
		t.Fatal("expected oversized max UDP relay packet size to be rejected")
	}
}

func TestNewServerClampsLegacyUdpPayloadLimit(t *testing.T) {
	certPEM, keyPEM := generateTestCert(t)
	server, err := NewServer(Instance{
		Listen:                "127.0.0.1",
		Port:                  0,
		Certificate:           string(certPEM),
		PrivateKey:            string(keyPEM),
		MaxUdpRelayPacketSize: maxLegacyUdpRelayPacketSize,
	}, &SocksRelay{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if server.maxUdpRelayPacketSize != maxSafeUdpRelayPacketSize {
		t.Fatalf("legacy UDP limit = %d, want clamped limit %d", server.maxUdpRelayPacketSize, maxSafeUdpRelayPacketSize)
	}
}

func TestPacketReassemblerInvalidatesAssemblyWhenFragmentTotalChanges(t *testing.T) {
	reassembler := newPacketReassembler(64)
	first := &PacketHeader{AssocID: 7, PktID: 9, FragTotal: 2, FragID: 0, Addr: &Address{Type: AddrTypeIPv4, IP: net.ParseIP("127.0.0.1"), Port: 53}, Size: 1}
	if _, _, complete := reassembler.feed(packetTransportDatagram, first, []byte("A")); complete {
		t.Fatal("first fragment unexpectedly completed")
	}
	single := &PacketHeader{AssocID: 7, PktID: 9, FragTotal: 1, FragID: 0, Addr: first.Addr, Size: 1}
	if _, got, complete := reassembler.feed(packetTransportDatagram, single, []byte("Z")); !complete || string(got) != "Z" {
		t.Fatalf("single packet = %q, complete=%v; want Z", got, complete)
	}
	last := &PacketHeader{AssocID: 7, PktID: 9, FragTotal: 2, FragID: 1, Size: 1}
	if _, _, complete := reassembler.feed(packetTransportDatagram, last, []byte("B")); complete {
		t.Fatal("stale first fragment was combined with a later packet")
	}
}

func TestUdpAssociationPinsFirstPacketModeAndDissociateClearsFragments(t *testing.T) {
	registry := newUdpAssociationRegistry(64)
	addr := &Address{Type: AddrTypeIPv4, IP: net.ParseIP("127.0.0.1"), Port: 53}
	first := &PacketHeader{AssocID: 3, PktID: 1, FragTotal: 2, FragID: 0, Addr: addr, Size: 1}
	association, _, _, complete := registry.feed(packetTransportDatagram, first, []byte("A"))
	if association == nil || complete {
		t.Fatal("expected first native fragment to establish an incomplete association")
	}
	singleStream := &PacketHeader{AssocID: 3, PktID: 2, FragTotal: 1, FragID: 0, Addr: addr, Size: 1}
	association, _, _, complete = registry.feed(packetTransportStream, singleStream, []byte("S"))
	if association.responseTransport != packetTransportDatagram || !complete {
		t.Fatalf("mixed-mode packet changed response mode: association=%+v complete=%v", association, complete)
	}

	if !registry.dissociate(3) {
		t.Fatal("expected dissociate to remove association")
	}
	late := &PacketHeader{AssocID: 3, PktID: 1, FragTotal: 2, FragID: 1, Size: 1}
	_, _, _, complete = registry.feed(packetTransportDatagram, late, []byte("B"))
	if complete {
		t.Fatal("late fragment completed an assembly from before dissociate")
	}
}

func TestPacketReassembler(t *testing.T) {
	pr := newPacketReassembler(1500)
	targetAddr := &Address{Type: AddrTypeIPv4, IP: net.ParseIP("1.1.1.1"), Port: 53}

	// 1. Unfragmented packet
	hdrSingle := &PacketHeader{
		AssocID:   1,
		PktID:     1,
		FragTotal: 1,
		FragID:    0,
		Size:      uint16(len("hello single")),
		Addr:      targetAddr,
	}
	addr, payload, complete := pr.feed(packetTransportDatagram, hdrSingle, []byte("hello single"))
	if addr == nil || !complete || string(payload) != "hello single" {
		t.Fatalf("unexpected single packet result: %v, %s", addr, payload)
	}

	// 2. In-order fragments (3 parts)
	hdr0 := &PacketHeader{AssocID: 2, PktID: 10, FragTotal: 3, FragID: 0, Size: 6, Addr: targetAddr}
	hdr1 := &PacketHeader{AssocID: 2, PktID: 10, FragTotal: 3, FragID: 1, Size: 6, Addr: &Address{Type: AddrTypeNone}}
	hdr2 := &PacketHeader{AssocID: 2, PktID: 10, FragTotal: 3, FragID: 2, Size: 5, Addr: &Address{Type: AddrTypeNone}}

	_, p0, complete := pr.feed(packetTransportDatagram, hdr0, []byte("part0-"))
	if p0 != nil || complete {
		t.Fatalf("expected nil before all fragments arrive, got %s", p0)
	}
	_, p1, complete := pr.feed(packetTransportDatagram, hdr1, []byte("part1-"))
	if p1 != nil || complete {
		t.Fatalf("expected nil before all fragments arrive, got %s", p1)
	}
	a2, p2, complete := pr.feed(packetTransportDatagram, hdr2, []byte("part2"))
	if a2 == nil || !complete || string(p2) != "part0-part1-part2" {
		t.Fatalf("expected reassembled payload 'part0-part1-part2', got %v, %s", a2, p2)
	}

	// 3. Out-of-order fragments (parts 1, 2, 0)
	hdrOO0 := &PacketHeader{AssocID: 3, PktID: 20, FragTotal: 3, FragID: 0, Size: 6, Addr: targetAddr}
	hdrOO1 := &PacketHeader{AssocID: 3, PktID: 20, FragTotal: 3, FragID: 1, Size: 7, Addr: &Address{Type: AddrTypeNone}}
	hdrOO2 := &PacketHeader{AssocID: 3, PktID: 20, FragTotal: 3, FragID: 2, Size: 3, Addr: &Address{Type: AddrTypeNone}}

	if _, p, done := pr.feed(packetTransportDatagram, hdrOO1, []byte("MIDDLE-")); p != nil || done {
		t.Fatalf("expected nil, got %s", p)
	}
	if _, p, done := pr.feed(packetTransportDatagram, hdrOO2, []byte("END")); p != nil || done {
		t.Fatalf("expected nil, got %s", p)
	}
	aOO, pOO, done := pr.feed(packetTransportDatagram, hdrOO0, []byte("START-"))
	if aOO == nil || !done || string(pOO) != "START-MIDDLE-END" {
		t.Fatalf("expected 'START-MIDDLE-END', got %s", pOO)
	}

	// 4. Invalid FragID >= FragTotal
	hdrInv := &PacketHeader{AssocID: 4, PktID: 30, FragTotal: 2, FragID: 2, Size: 7, Addr: targetAddr}
	if _, p, done := pr.feed(packetTransportDatagram, hdrInv, []byte("invalid")); p != nil || done {
		t.Fatalf("expected nil for invalid FragID, got %s", p)
	}

	// A changed fragment total for an in-flight packet must discard the packet safely.
	hdrMixed0 := &PacketHeader{AssocID: 5, PktID: 40, FragTotal: 2, FragID: 0, Size: 1, Addr: targetAddr}
	hdrMixed3 := &PacketHeader{AssocID: 5, PktID: 40, FragTotal: 4, FragID: 3, Size: 1, Addr: &Address{Type: AddrTypeNone}}
	if _, _, done := pr.feed(packetTransportDatagram, hdrMixed0, []byte("a")); done {
		t.Fatal("expected first mixed-total fragment to remain incomplete")
	}
	if _, _, done := pr.feed(packetTransportDatagram, hdrMixed3, []byte("b")); done {
		t.Fatal("expected inconsistent fragment total to be discarded")
	}
	if _, ok := pr.packets[packetFragmentKey{assocID: 5, pktID: 40, transport: packetTransportDatagram}]; ok {
		t.Fatal("inconsistent packet assembly was not discarded")
	}

	// Fragments from different transports cannot be combined into one packet.
	streamFirst := &PacketHeader{AssocID: 6, PktID: 50, FragTotal: 2, FragID: 0, Size: 1, Addr: targetAddr}
	datagramLast := &PacketHeader{AssocID: 6, PktID: 50, FragTotal: 2, FragID: 1, Size: 1, Addr: &Address{Type: AddrTypeNone}}
	if _, _, done := pr.feed(packetTransportStream, streamFirst, []byte("a")); done {
		t.Fatal("expected first stream fragment to remain incomplete")
	}
	if _, _, done := pr.feed(packetTransportDatagram, datagramLast, []byte("b")); done {
		t.Fatal("fragments from different transports must not combine")
	}
	if _, _, done := pr.feed(packetTransportStream, datagramLast, []byte("b")); !done {
		t.Fatal("expected stream fragments to reassemble")
	}

	// The configured size limit caps both complete packets and reassembly state.
	limited := newPacketReassembler(3)
	tooLarge0 := &PacketHeader{AssocID: 7, PktID: 60, FragTotal: 2, FragID: 0, Size: 2, Addr: targetAddr}
	tooLarge1 := &PacketHeader{AssocID: 7, PktID: 60, FragTotal: 2, FragID: 1, Size: 2, Addr: &Address{Type: AddrTypeNone}}
	if _, _, done := limited.feed(packetTransportDatagram, tooLarge0, []byte("ab")); done {
		t.Fatal("expected first oversized packet fragment to remain incomplete")
	}
	if _, _, done := limited.feed(packetTransportDatagram, tooLarge1, []byte("cd")); done {
		t.Fatal("oversized reassembled packet must be rejected")
	}
	if len(limited.packets) != 0 {
		t.Fatal("oversized reassembly state was not discarded")
	}

	bounded := newPacketReassembler(1500)
	for i := 0; i < maxPendingPacketAssemblies; i++ {
		hdr := &PacketHeader{
			AssocID:   8,
			PktID:     uint16(i),
			FragTotal: 2,
			FragID:    0,
			Size:      1,
			Addr:      targetAddr,
		}
		if _, _, done := bounded.feed(packetTransportDatagram, hdr, []byte("a")); done {
			t.Fatal("expected pending fragment to remain incomplete")
		}
	}
	if len(bounded.packets) != maxPendingPacketAssemblies {
		t.Fatalf("pending assembly count = %d, want %d", len(bounded.packets), maxPendingPacketAssemblies)
	}
	extra := &PacketHeader{AssocID: 8, PktID: 100, FragTotal: 2, FragID: 0, Size: 1, Addr: targetAddr}
	if _, _, done := bounded.feed(packetTransportDatagram, extra, []byte("a")); done {
		t.Fatal("expected new assembly to be rejected when the pending limit is reached")
	}
	if len(bounded.packets) != maxPendingPacketAssemblies {
		t.Fatalf("pending assembly count after overflow = %d, want %d", len(bounded.packets), maxPendingPacketAssemblies)
	}

	for _, packet := range bounded.packets {
		packet.updatedAt = time.Now().Add(-packetAssemblyTimeout - time.Second)
	}
	if _, _, done := bounded.feed(packetTransportDatagram, extra, []byte("a")); done {
		t.Fatal("expected new fragment to remain incomplete after stale entries are evicted")
	}
	if len(bounded.packets) != 1 {
		t.Fatalf("pending assembly count after stale cleanup = %d, want 1", len(bounded.packets))
	}
}

func TestAuthenticationTimeoutClosesUnauthenticatedConnections(t *testing.T) {
	for _, partial := range []bool{false, true} {
		name := "no-authenticate"
		if partial {
			name = "partial-authenticate"
		}
		t.Run(name, func(t *testing.T) {
			certPEM, keyPEM := generateTestCert(t)
			server, err := NewServer(Instance{
				Id:                    99010,
				Tag:                   "auth-timeout-test",
				Listen:                "127.0.0.1",
				Port:                  0,
				Certificate:           string(certPEM),
				PrivateKey:            string(keyPEM),
				ALPN:                  []string{"h3"},
				MaxIdleTime:           5,
				AuthenticationTimeout: 1,
			}, &SocksRelay{})
			if err != nil {
				t.Fatalf("NewServer: %v", err)
			}
			if err := server.Start(); err != nil {
				t.Fatalf("Server.Start: %v", err)
			}
			t.Cleanup(func() { _ = server.Close() })

			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			conn, err := quic.DialAddr(ctx, server.packetConn.LocalAddr().String(), &tls.Config{
				InsecureSkipVerify: true,
				NextProtos:         []string{"h3"},
			}, &quic.Config{EnableDatagrams: true, KeepAlivePeriod: time.Second})
			if err != nil {
				t.Fatalf("quic.DialAddr: %v", err)
			}
			defer conn.CloseWithError(0, "")

			if partial {
				stream, err := conn.OpenUniStreamSync(ctx)
				if err != nil {
					t.Fatalf("OpenUniStreamSync: %v", err)
				}
				if _, err := stream.Write([]byte{ProtocolVersion, CmdAuthenticate, 1}); err != nil {
					t.Fatalf("write partial Authenticate: %v", err)
				}
			}

			select {
			case <-conn.Context().Done():
			case <-ctx.Done():
				t.Fatalf("server left unauthenticated QUIC connection open: %v", ctx.Err())
			}
		})
	}
}
