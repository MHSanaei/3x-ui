package amneziawgnet

import (
	"net"
	"net/netip"
	"strconv"
	"testing"
	"time"

	awgconn "github.com/amnezia-vpn/amneziawg-go/v3/conn"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
)

func TestParseListenAddr(t *testing.T) {
	cases := []struct {
		in     string
		pinned bool
		want   string
	}{
		{in: "", pinned: false},
		{in: "  ", pinned: false},
		{in: "0.0.0.0", pinned: false},
		{in: "::", pinned: false},
		{in: "::0", pinned: false},
		{in: "[::]", pinned: false},
		{in: "[::0]", pinned: false},
		{in: "127.0.0.1", pinned: true, want: "127.0.0.1"},
		{in: "::1", pinned: true, want: "::1"},
		{in: "[::1]", pinned: true, want: "::1"},
		{in: "not-an-ip", pinned: false},
		{in: "/var/run/awg.sock", pinned: false},
	}
	for _, tc := range cases {
		addr, ok := parseListenAddr(tc.in)
		if ok != tc.pinned {
			t.Fatalf("parseListenAddr(%q) pinned=%v, want %v", tc.in, ok, tc.pinned)
		}
		if tc.pinned && addr.String() != tc.want {
			t.Fatalf("parseListenAddr(%q) = %s, want %s", tc.in, addr, tc.want)
		}
	}
}

func TestNewListenBindPinsSpecificAddress(t *testing.T) {
	bind := newListenBind("127.0.0.1")
	pb, ok := bind.(*pinnedBind)
	if !ok {
		t.Fatalf("bind type = %T, want *pinnedBind", bind)
	}
	fns, port, err := pb.Open(0)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer pb.Close()
	if len(fns) != 1 {
		t.Fatalf("ReceiveFuncs = %d, want 1", len(fns))
	}
	if port == 0 {
		t.Fatal("expected a concrete ephemeral port")
	}

	laddr := pb.conn.LocalAddr().(*net.UDPAddr)
	got := laddr.AddrPort().Addr().Unmap()
	if got.String() != "127.0.0.1" {
		t.Fatalf("LocalAddr = %v, want 127.0.0.1", got)
	}

	clash := newListenBind("127.0.0.1")
	if _, _, err := clash.Open(port); err == nil {
		clash.Close()
		t.Fatalf("Open(%d) unexpectedly succeeded on an already-bound address", port)
	}
}

func TestNewListenBindWildcardUsesDefault(t *testing.T) {
	for _, listen := range []string{"", "0.0.0.0", "::", "::0", "[::]", "hostname.example", "203.0.113.10", "not-an-ip"} {
		bind := newListenBind(listen)
		if _, ok := bind.(*pinnedBind); ok {
			t.Fatalf("newListenBind(%q) returned pinnedBind, want default StdNetBind", listen)
		}
	}
}

func TestPinnedBindRoundTrip(t *testing.T) {
	server := newListenBind("127.0.0.1")
	recvFns, port, err := server.Open(0)
	if err != nil {
		t.Fatalf("server Open: %v", err)
	}
	defer server.Close()

	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("client listen: %v", err)
	}
	defer client.Close()

	payload := []byte("hello-awg-listen")
	dst := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))
	ap, err := netip.ParseAddrPort(dst)
	if err != nil {
		t.Fatalf("ParseAddrPort: %v", err)
	}
	if _, err := client.WriteToUDPAddrPort(payload, ap); err != nil {
		t.Fatalf("client write: %v", err)
	}

	bufs := [][]byte{make([]byte, 1500)}
	sizes := make([]int, 1)
	eps := make([]awgconn.Endpoint, 1)
	n, err := recvFns[0](bufs, sizes, eps)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if n != 1 || sizes[0] != len(payload) {
		t.Fatalf("receive n=%d size=%d, want 1/%d", n, sizes[0], len(payload))
	}
	if string(bufs[0][:sizes[0]]) != string(payload) {
		t.Fatalf("payload = %q, want %q", bufs[0][:sizes[0]], payload)
	}

	reply := []byte("pong")
	if err := server.Send([][]byte{reply}, eps[0]); err != nil {
		t.Fatalf("Send: %v", err)
	}
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1500)
	rn, _, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("client read: %v", err)
	}
	if string(buf[:rn]) != string(reply) {
		t.Fatalf("reply = %q, want %q", buf[:rn], reply)
	}
}

func TestAddressFingerprintIncludesListen(t *testing.T) {
	base := amneziawg.Instance{
		MTU:         1420,
		Address:     []string{"10.8.1.1/24"},
		Obfuscation: amneziawg.Obfuscation31{},
	}
	a := addressFingerprint(base)
	base.Listen = "127.0.0.1"
	b := addressFingerprint(base)
	if a == b {
		t.Fatalf("listen edit did not change addressFingerprint: %q", a)
	}
	base.Listen = "0.0.0.0"
	if addressFingerprint(base) != a {
		t.Fatal("wildcard spellings must share the empty-listen fingerprint")
	}
	base.Listen = "hostname.example"
	if addressFingerprint(base) != a {
		t.Fatal("unusable listen must fingerprint like wildcard fallback")
	}
}

var _ awgconn.Bind = (*pinnedBind)(nil)
