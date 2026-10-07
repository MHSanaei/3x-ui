package tuic

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"
)

func TestAudit3CloseMustInterruptIdleTCPRelay(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
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
	server, conn, _, _ := startLifecycleTestServer(t, listener.Addr().String(), "close-idle@audit3")
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
	go func() { closed <- server.Close() }()

	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatalf("Server.Close waited for idle TCP peer after %s", time.Since(started))
	}
	select {
	case <-peerFIN:
	case <-time.After(time.Second):
		t.Fatal("upstream socket did not close")
	}
	_, _, deltas := server.CollectAllTraffic()
	if len(deltas) != 1 || deltas[0].Up != 1 || deltas[0].Down != 1 {
		t.Fatalf("final traffic: %+v", deltas)
	}
}
