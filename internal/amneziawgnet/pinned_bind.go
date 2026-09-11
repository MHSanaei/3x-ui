package amneziawgnet

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"

	awgconn "github.com/amnezia-vpn/amneziawg-go/v3/conn"
)

// pinnedBind is a Bind that opens its UDP socket on exactly one host
// address. StdNetBind always listens on the wildcard ("0.0.0.0" / "::"),
// which is what made AmneziaWG inbounds with a non-empty listen field
// reply from the host's primary address on multi-IP setups (#6367).
//
// BatchSize is 1 and sticky/GSO offloads are omitted: correctness of the
// source address is the point of this Bind, and the common empty-listen
// path still uses StdNetBind via newListenBind.
type pinnedBind struct {
	mu   sync.Mutex
	addr netip.Addr
	conn *net.UDPConn
}

func newPinnedBind(addr netip.Addr) *pinnedBind {
	return &pinnedBind{addr: addr.Unmap()}
}

func (b *pinnedBind) Open(uport uint16) ([]awgconn.ReceiveFunc, uint16, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.conn != nil {
		return nil, 0, awgconn.ErrBindAlreadyOpen
	}

	network := "udp4"
	if b.addr.Is6() {
		network = "udp6"
	}
	pc, err := net.ListenPacket(network, net.JoinHostPort(b.addr.String(), strconv.Itoa(int(uport))))
	if err != nil {
		return nil, 0, err
	}
	uc, ok := pc.(*net.UDPConn)
	if !ok {
		pc.Close()
		return nil, 0, fmt.Errorf("amneziawgnet: listen %s returned %T, want *net.UDPConn", network, pc)
	}
	laddr, ok := uc.LocalAddr().(*net.UDPAddr)
	if !ok {
		uc.Close()
		return nil, 0, fmt.Errorf("amneziawgnet: unexpected local addr %T", uc.LocalAddr())
	}
	b.conn = uc
	return []awgconn.ReceiveFunc{b.makeReceiveFunc(uc)}, uint16(laddr.Port), nil
}

func (b *pinnedBind) makeReceiveFunc(uc *net.UDPConn) awgconn.ReceiveFunc {
	return func(bufs [][]byte, sizes []int, eps []awgconn.Endpoint) (int, error) {
		n, addr, err := uc.ReadFromUDPAddrPort(bufs[0])
		if err != nil {
			return 0, err
		}
		sizes[0] = n
		eps[0] = &awgconn.StdNetEndpoint{AddrPort: netip.AddrPortFrom(addr.Addr().Unmap(), addr.Port())}
		return 1, nil
	}
}

func (b *pinnedBind) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.conn == nil {
		return nil
	}
	err := b.conn.Close()
	b.conn = nil
	return err
}

// SetMark is a no-op: the panel never configures a WireGuard fwmark for
// AmneziaWG, and pinning the listen address already fixes reply sourcing.
func (b *pinnedBind) SetMark(uint32) error { return nil }

func (b *pinnedBind) Send(bufs [][]byte, ep awgconn.Endpoint) error {
	std, ok := ep.(*awgconn.StdNetEndpoint)
	if !ok {
		return awgconn.ErrWrongEndpointType
	}
	b.mu.Lock()
	uc := b.conn
	b.mu.Unlock()
	if uc == nil {
		return net.ErrClosed
	}
	for _, buf := range bufs {
		if _, err := uc.WriteToUDPAddrPort(buf, std.AddrPort); err != nil {
			return err
		}
	}
	return nil
}

func (b *pinnedBind) ParseEndpoint(s string) (awgconn.Endpoint, error) {
	ap, err := netip.ParseAddrPort(s)
	if err != nil {
		return nil, err
	}
	return &awgconn.StdNetEndpoint{AddrPort: netip.AddrPortFrom(ap.Addr().Unmap(), ap.Port())}, nil
}

func (b *pinnedBind) BatchSize() int { return 1 }

// parseListenAddr interprets an inbound listen string. ok is false when the
// value means "bind all addresses" (empty or a wildcard). An unparseable
// non-wildcard value is an error so a typo fails loudly instead of silently
// falling back to the wildcard that caused #6367.
func parseListenAddr(listen string) (addr netip.Addr, ok bool, err error) {
	listen = strings.TrimSpace(listen)
	if listen == "" || listen == "0.0.0.0" || listen == "::" || listen == "[::]" {
		return netip.Addr{}, false, nil
	}
	addr, err = netip.ParseAddr(listen)
	if err != nil {
		return netip.Addr{}, false, fmt.Errorf("invalid listen address %q: %w", listen, err)
	}
	return addr.Unmap(), true, nil
}

// newListenBind returns StdNetBind for wildcard listen values, or a
// pinnedBind bound to a specific host address.
func newListenBind(listen string) (awgconn.Bind, error) {
	addr, pinned, err := parseListenAddr(listen)
	if err != nil {
		return nil, err
	}
	if !pinned {
		return awgconn.NewDefaultBind(), nil
	}
	return newPinnedBind(addr), nil
}
