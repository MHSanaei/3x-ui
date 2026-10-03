package tuic

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"testing"
	"time"

	serverquic "github.com/apernet/quic-go"
	"github.com/google/uuid"
	clientquic "github.com/quic-go/quic-go"
)

func startLifecycleTestServer(t *testing.T, relayAddr, email string) (*Server, *clientquic.Conn, uuid.UUID, string) {
	t.Helper()
	cert, key := generateTestCert(t)
	clientID := uuid.New()
	password := "lifecycle-test-password"
	server, err := NewServer(Instance{
		Id: 99101, Tag: "lifecycle-test", Listen: "127.0.0.1", Port: 0,
		Certificate: string(cert), PrivateKey: string(key), ALPN: []string{"h3"},
		AuthenticationTimeout: 2, MaxIdleTime: 30,
		Clients: []TuicClientSettings{{UUID: clientID.String(), Password: password, Email: email}},
	}, &SocksRelay{Addr: relayAddr, Password: "lifecycle-socks-password"})
	if err != nil {
		t.Fatalf("create TUIC server: %v", err)
	}
	if err := server.Start(); err != nil {
		t.Fatalf("start TUIC server: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := clientquic.DialAddr(ctx, server.packetConn.LocalAddr().String(),
		&tls.Config{InsecureSkipVerify: true, NextProtos: []string{"h3"}},
		&clientquic.Config{EnableDatagrams: true})
	if err != nil {
		t.Fatalf("dial TUIC server: %v", err)
	}
	t.Cleanup(func() { _ = client.CloseWithError(0, "test complete") })

	tlsState := client.ConnectionState().TLS
	token, err := tlsState.ExportKeyingMaterial(string(clientID[:]), []byte(password), 32)
	if err != nil {
		t.Fatalf("derive authentication token: %v", err)
	}
	stream, err := client.OpenUniStreamSync(ctx)
	if err != nil {
		t.Fatalf("open authentication stream: %v", err)
	}
	auth := append([]byte{ProtocolVersion, CmdAuthenticate}, clientID[:]...)
	auth = append(auth, token...)
	if _, err := stream.Write(auth); err != nil {
		t.Fatalf("write authentication: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("close authentication stream: %v", err)
	}
	return server, client, clientID, password
}

func authenticatedServerConnection(t *testing.T, server *Server, clientID uuid.UUID) (*serverquic.Conn, *User) {
	t.Helper()
	id := [16]byte(clientID)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		server.activeConnsMu.Lock()
		for conn, user := range server.activeConns[id] {
			server.activeConnsMu.Unlock()
			return conn, user
		}
		server.activeConnsMu.Unlock()
		time.Sleep(time.Millisecond)
	}
	t.Fatal("server did not register authenticated QUIC connection")
	return nil, nil
}

func TestMalformedBiStreamDelayedFINReleasesReceiveCredit(t *testing.T) {
	_, client, _, _ := startLifecycleTestServer(t, "127.0.0.1:1", "bidi-lifecycle@x")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for i := 0; i < 110; i++ {
		openCtx, openCancel := context.WithTimeout(ctx, 700*time.Millisecond)
		stream, err := client.OpenStreamSync(openCtx)
		openCancel()
		if err != nil {
			t.Fatalf("bidirectional stream %d blocked after unsupported commands: %v", i+1, err)
		}
		if _, err := stream.Write([]byte{ProtocolVersion, 0xff}); err != nil {
			t.Fatalf("write unsupported command %d: %v", i+1, err)
		}
		stream.SetReadDeadline(time.Now().Add(time.Second))
		if _, err := io.Copy(io.Discard, stream); err != nil {
			t.Fatalf("wait for unsupported stream %d to close: %v", i+1, err)
		}
		_ = stream.Close()
	}
}

func TestDownstreamUDPResponseRefreshesAssociationIdleTime(t *testing.T) {
	socksAddr, cleanup := startMockSocks5Server(t, "udp-lifecycle@x", "lifecycle-socks-password")
	defer cleanup()
	server, client, clientID, _ := startLifecycleTestServer(t, socksAddr, "udp-lifecycle@x")
	serverConn, user := authenticatedServerConnection(t, server, clientID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := server.relay.DialUDP(ctx, user.Email)
	if err != nil {
		t.Fatalf("open SOCKS UDP session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	const associationID uint16 = 61244
	oldActive := time.Now().Add(-udpAssociationIdleTimeout - time.Second)
	association := &udpAssociation{
		responseTransport: packetTransportDatagram,
		relay:             &udpRelaySession{relay: session, responseTransport: packetTransportDatagram},
		lastActive:        oldActive,
	}
	registry := newUdpAssociationRegistry(maxUdpRelayPacketSize)
	registry.associations[associationID] = association
	responseDone := make(chan struct{})
	go func() {
		defer close(responseDone)
		server.relayUDPResponses(ctx, serverConn, user, associationID, association, registry, association.relay)
	}()

	target := &Address{Type: AddrTypeIPv4, IP: net.IPv4(8, 8, 8, 8), Port: 53}
	if _, err := session.Send(target, []byte("seed")); err != nil {
		t.Fatalf("send SOCKS seed datagram: %v", err)
	}
	response, err := client.ReceiveDatagram(ctx)
	if err != nil {
		t.Fatalf("receive echoed UDP response: %v", err)
	}
	if len(response) < 2 || response[0] != ProtocolVersion || response[1] != CmdPacket {
		t.Fatalf("unexpected TUIC UDP response: %x", response)
	}
	reader := bytes.NewReader(response[2:])
	header, err := ReadPacketHeader(reader)
	if err != nil {
		t.Fatalf("read response header: %v", err)
	}
	got, err := readPacketPayload(reader, header)
	if err != nil || !bytes.Equal(got, []byte("seed")) {
		t.Fatalf("echo response payload = %q, error=%v", got, err)
	}
	if header.AssocID != associationID {
		t.Fatalf("response association id = %d, want %d", header.AssocID, associationID)
	}

	registry.mu.Lock()
	refreshedAt := association.lastActive
	registry.mu.Unlock()
	if !refreshedAt.After(oldActive) {
		t.Fatal("successful downstream response did not refresh association activity")
	}
	registry.reapIdle(oldActive.Add(udpAssociationIdleTimeout + time.Second))
	registry.mu.Lock()
	remaining := registry.associations[associationID]
	registry.mu.Unlock()
	if remaining != association {
		t.Fatal("association was reaped despite a recently delivered downstream response")
	}
	_ = session.Close()
	select {
	case <-responseDone:
	case <-time.After(time.Second):
		t.Fatal("UDP response relay did not stop after SOCKS session closed")
	}
}

func TestUdpAssociationTouchDoesNotRefreshReusedID(t *testing.T) {
	registry := newUdpAssociationRegistry(maxUdpRelayPacketSize)
	addr := &Address{Type: AddrTypeIPv4, IP: net.IPv4(8, 8, 8, 8), Port: 53}
	header := &PacketHeader{AssocID: 17, PktID: 1, FragTotal: 1, FragID: 0, Size: 1, Addr: addr}
	old, _, _, complete := registry.feed(packetTransportDatagram, header, []byte("x"))
	if !complete {
		t.Fatal("failed to create first association generation")
	}
	oldTime := old.lastActive
	if !registry.dissociate(header.AssocID) {
		t.Fatal("failed to dissociate first association generation")
	}
	newGeneration, _, _, complete := registry.feed(packetTransportDatagram, header, []byte("x"))
	if !complete || newGeneration == old {
		t.Fatal("failed to create replacement association generation")
	}
	if registry.touch(header.AssocID, old, oldTime.Add(time.Hour)) {
		t.Fatal("late response refreshed a replacement association generation")
	}
	if !newGeneration.lastActive.Before(oldTime.Add(time.Hour)) {
		t.Fatal("replacement association timestamp changed after stale touch")
	}
}
