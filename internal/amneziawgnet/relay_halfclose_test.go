package amneziawgnet

// Half-close through SocksRelay.RelayTCP needs the raw TCP conn to the bridge,
// not x/net/proxy's *socks.Conn wrapper, which has no CloseWrite.

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/amnezia-vpn/amneziawg-go/v3/tun"
	"github.com/amnezia-vpn/amneziawg-go/v3/tun/netstack"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
)

func hcListen(tb testing.TB) net.Listener {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("listen: %v", err)
	}
	return ln
}

type hcNetstacks struct {
	client *netstack.Net
	ln     *gonet.TCPListener
	addr   netip.AddrPort
	close  func()
}

func hcShuttle(src, dst tun.Device) {
	batch := src.BatchSize()
	bufs := make([][]byte, batch)
	for i := range bufs {
		bufs[i] = make([]byte, 65536)
	}
	sizes := make([]int, batch)
	for {
		n, err := src.Read(bufs, sizes, 0)
		if err != nil {
			return
		}
		out := make([][]byte, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, bufs[i][:sizes[i]])
		}
		if _, err := dst.Write(out, 0); err != nil {
			return
		}
	}
}

// hcNewNetstacks links a client stack to a server stack whose accepted conns are the
// *gonet.TCPConn the relay gets in production.
func hcNewNetstacks(tb testing.TB) *hcNetstacks {
	ca := netip.MustParseAddr("10.77.0.1")
	sa := netip.MustParseAddr("10.77.0.2")
	ctun, cnet, err := netstack.CreateNetTUN([]netip.Addr{ca}, nil, 1420)
	if err != nil {
		tb.Fatalf("client netstack: %v", err)
	}
	stun, snet, err := netstack.CreateNetTUN([]netip.Addr{sa}, nil, 1420)
	if err != nil {
		tb.Fatalf("server netstack: %v", err)
	}
	go hcShuttle(ctun, stun)
	go hcShuttle(stun, ctun)
	addr := netip.AddrPortFrom(sa, 80)
	ln, err := snet.ListenTCPAddrPort(addr)
	if err != nil {
		tb.Fatalf("server listen: %v", err)
	}
	return &hcNetstacks{client: cnet, ln: ln, addr: addr, close: func() {
		_ = ln.Close()
		_ = ctun.Close()
		_ = stun.Close()
	}}
}

func (n *hcNetstacks) pair(tb testing.TB) (client, server *gonet.TCPConn) {
	ch := make(chan net.Conn, 1)
	go func() {
		c, err := n.ln.Accept()
		if err != nil {
			ch <- nil
			return
		}
		ch <- c
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := n.client.DialContextTCPAddrPort(ctx, n.addr)
	if err != nil {
		tb.Fatalf("tunnel dial: %v", err)
	}
	s := <-ch
	if s == nil {
		tb.Fatal("tunnel accept failed")
	}
	return client, s.(*gonet.TCPConn)
}

// hcServeSocks answers one username/password SOCKS5 CONNECT (as Xray's inbound would),
// reads the 4-byte request, writes payload and half-closes.
func hcServeSocks(c net.Conn, payload []byte) error {
	br := bufio.NewReader(c)
	buf := make([]byte, 512)
	read := func(n int) ([]byte, error) { _, err := io.ReadFull(br, buf[:n]); return buf[:n], err }
	hdr, err := read(2)
	if err != nil {
		return err
	}
	if _, err := read(int(hdr[1])); err != nil {
		return err
	}
	if _, err := c.Write([]byte{5, 2}); err != nil {
		return err
	}
	v, err := read(2)
	if err != nil {
		return err
	}
	if _, err := read(int(v[1])); err != nil {
		return err
	}
	pl, err := read(1)
	if err != nil {
		return err
	}
	if _, err := read(int(pl[0])); err != nil {
		return err
	}
	if _, err := c.Write([]byte{1, 0}); err != nil {
		return err
	}
	req, err := read(4)
	if err != nil {
		return err
	}
	alen := map[byte]int{1: 4, 4: 16}[req[3]]
	if req[3] == 3 {
		l, err := read(1)
		if err != nil {
			return err
		}
		alen = int(l[0])
	}
	if _, err := read(alen + 2); err != nil {
		return err
	}
	if _, err := c.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
		return err
	}
	if _, err := read(4); err != nil {
		return err
	}
	if _, err := c.Write(payload); err != nil {
		return err
	}
	if tc, ok := c.(*net.TCPConn); ok {
		return tc.CloseWrite()
	}
	return errors.New("not tcp")
}

// TestHCHalfCloseThroughRelayTCP: a client that sends its request and half-closes
// must still get the upstream's reply through SocksRelay.RelayTCP.
func TestHCHalfCloseThroughRelayTCP(t *testing.T) {
	ns := hcNewNetstacks(t)
	defer ns.close()
	bridge := hcListen(t)
	defer bridge.Close()
	relay := SocksRelay{Addr: bridge.Addr().String(), Password: "x"}
	reply := []byte("reply-after-half-close")
	served := make(chan error, 1)
	go func() {
		c, err := bridge.Accept()
		if err != nil {
			served <- err
			return
		}
		defer c.Close()
		served <- hcServeSocks(c, reply)
	}()
	client, server := ns.pair(t)
	defer client.Close()
	done := make(chan struct{})
	go func() { relay.RelayTCP(server, "bench@example", netip.MustParseAddrPort("192.0.2.1:443")); close(done) }()
	_, _ = client.Write([]byte("ping"))
	_ = client.CloseWrite()
	_ = client.SetReadDeadline(time.Now().Add(5 * time.Second))
	got, err := io.ReadAll(client)
	<-done
	srvErr := <-served
	if string(got) != string(reply) {
		t.Fatalf("client got %q (read err %v, bridge err %v), want %q: the half-close killed the upstream before its reply", got, err, srvErr, reply)
	}
}
