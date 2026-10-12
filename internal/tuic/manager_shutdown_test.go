package tuic

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	clientquic "github.com/quic-go/quic-go"
)

func TestAudit3ManagerStopMustInterruptIdleTCPRelay(t *testing.T) {
	var listener net.Listener
	var err error
	var inboundID int
	for p := 64051; p < 64080; p++ {
		listener, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			inboundID = 500000 + (p - 64000)
			break
		}
	}
	if listener == nil {
		t.Fatal(err)
	}
	defer listener.Close()
	release := make(chan struct{})
	defer close(release)
	peerFIN := make(chan struct{})
	ready := make(chan struct{})
	go func() {
		c, err := listener.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		var greeting [4]byte
		if _, err := io.ReadFull(c, greeting[:]); err != nil {
			return
		}
		c.Write([]byte{5, 0})
		var req [10]byte
		if _, err := io.ReadFull(c, req[:]); err != nil {
			return
		}
		c.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0})
		var payload [1]byte
		if _, err := io.ReadFull(c, payload[:]); err != nil {
			return
		}
		c.Write(payload[:])
		close(ready)
		io.Copy(io.Discard, c)
		close(peerFIN)
		<-release
	}()
	cert, key := generateTestCert(t)
	clientID := uuid.New()
	password := "audit3-password"
	m := &Manager{servers: make(map[int]*managed), lastStartErr: make(map[int]string), pendingTraffic: make(map[string]ClientTrafficDelta)}
	if err := m.Ensure(Instance{Id: inboundID, Tag: "audit3-manager-shutdown", Listen: "127.0.0.1", Port: 0, Certificate: string(cert), PrivateKey: string(key), ALPN: []string{"h3"}, AuthenticationTimeout: 2, MaxIdleTime: 30, Clients: []TuicClientSettings{{UUID: clientID.String(), Password: password, Email: "close-idle@audit3"}}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.StopAll)
	server := m.servers[inboundID].server
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dialCancel()
	conn, err := clientquic.DialAddr(dialCtx, server.packetConn.LocalAddr().String(), &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"h3"}}, &clientquic.Config{EnableDatagrams: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.CloseWithError(0, "") })
	tlsState := conn.ConnectionState().TLS
	token, err := tlsState.ExportKeyingMaterial(string(clientID[:]), []byte(password), 32)
	if err != nil {
		t.Fatal(err)
	}
	auth, err := conn.OpenUniStreamSync(dialCtx)
	if err != nil {
		t.Fatal(err)
	}
	payload := append([]byte{ProtocolVersion, CmdAuthenticate}, clientID[:]...)
	payload = append(payload, token...)
	if _, err := auth.Write(payload); err != nil {
		t.Fatal(err)
	}
	auth.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var cmd bytes.Buffer
	cmd.Write([]byte{ProtocolVersion, CmdConnect})
	WriteAddress(&cmd, &Address{Type: AddrTypeIPv4, IP: net.ParseIP("1.1.1.1"), Port: 443})
	cmd.WriteByte('x')
	if _, err := stream.Write(cmd.Bytes()); err != nil {
		t.Fatal(err)
	}
	var echo [1]byte
	if _, err := io.ReadFull(stream, echo[:]); err != nil {
		t.Fatal(err)
	}
	<-ready
	closed := make(chan error, 1)
	started := time.Now()
	go func() { m.StopAll(); closed <- nil }()

	queried := make(chan bool, 1)
	go func() { queried <- m.HasRunning() }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatalf("StopAll blocked on idle TCP peer after %s", time.Since(started))
	}
	select {
	case <-queried:
	case <-time.After(time.Second):
		t.Fatal("manager query blocked after StopAll")
	}
	select {
	case <-peerFIN:
	case <-time.After(time.Second):
		t.Fatal("upstream connection remained open")
	}
	_, deltas := m.CollectAllTraffic()
	if len(deltas) != 1 || deltas[0].Up != 1 || deltas[0].Down != 1 {
		t.Fatalf("final counters: %+v", deltas)
	}
	_, again := m.CollectAllTraffic()
	if len(again) != 0 {
		t.Fatalf("repeated final counters: %+v", again)
	}
}
