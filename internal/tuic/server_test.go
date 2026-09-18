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
		CongestionControl:     "bbr",
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
		CongestionControl:     "bbr",
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
	udpMsg := []byte("dns datagram query")
	var dgramBuf bytes.Buffer
	if err := WritePacket(&dgramBuf, 100, 1, 1, 0, target, udpMsg); err != nil {
		t.Fatalf("WritePacket failed: %v", err)
	}

	// Give a tiny moment for auth to register
	time.Sleep(50 * time.Millisecond)

	if err := conn.SendDatagram(dgramBuf.Bytes()); err != nil {
		t.Fatalf("SendDatagram failed: %v", err)
	}

	// 3. Receive echo reply via datagram
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
	replyPayload := make([]byte, hdr.Size)
	if _, err := io.ReadFull(pktReader, replyPayload); err != nil {
		t.Fatalf("read reply payload failed: %v", err)
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
