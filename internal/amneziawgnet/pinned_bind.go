package amneziawgnet

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"

	awgconn "github.com/amnezia-vpn/amneziawg-go/v3/conn"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// pinnedBind opens its UDP socket on exactly one host address (#6367).
// Empty/wildcard listen still uses StdNetBind via newListenBind.
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
	pc, err := (&net.ListenConfig{}).ListenPacket(context.Background(), network, net.JoinHostPort(b.addr.String(), strconv.Itoa(int(uport))))
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

// SetMark is a no-op: the panel never configures a WireGuard fwmark here.
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

// isWildcardListen reports empty / dual-stack wildcard listen values.
// Includes ::0 (isAnyListen) and [::] so AmneziaWG keeps dual-stack StdNetBind.
func isWildcardListen(listen string) bool {
	switch strings.TrimSpace(listen) {
	case "", "0.0.0.0", "::", "::0", "[::]", "[::0]":
		return true
	default:
		return false
	}
}

// parseListenAddr returns a concrete host address to pin. ok is false for
// wildcards and for values that are not a bare IP (previously inert for AWG).
func parseListenAddr(listen string) (addr netip.Addr, ok bool) {
	listen = strings.TrimSpace(listen)
	if isWildcardListen(listen) {
		return netip.Addr{}, false
	}
	// Bracketed IPv6 literal e.g. [::1] — strip for ParseAddr.
	if strings.HasPrefix(listen, "[") && strings.HasSuffix(listen, "]") {
		listen = listen[1 : len(listen)-1]
	}
	addr, err := netip.ParseAddr(listen)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

// listenBindable probes whether addr can be used as a UDP local address.
func listenBindable(addr netip.Addr) bool {
	network := "udp4"
	if addr.Is6() {
		network = "udp6"
	}
	pc, err := (&net.ListenConfig{}).ListenPacket(context.Background(), network, net.JoinHostPort(addr.String(), "0"))
	if err != nil {
		return false
	}
	_ = pc.Close()
	return true
}

// newListenBind returns StdNetBind for wildcards / unusable listen values, or
// a pinnedBind for a real local address. Never fails the inbound on bad listen.
func newListenBind(listen string) awgconn.Bind {
	raw := strings.TrimSpace(listen)
	addr, pinned := parseListenAddr(raw)
	if !pinned {
		if raw != "" && !isWildcardListen(raw) {
			logger.Warningf("amneziawgnet: listen %q is not a bindable IP; using dual-stack wildcard", raw)
		}
		return awgconn.NewDefaultBind()
	}
	if !listenBindable(addr) {
		logger.Warningf("amneziawgnet: listen %q is not usable on this host; using dual-stack wildcard", raw)
		return awgconn.NewDefaultBind()
	}
	return newPinnedBind(addr)
}

// normalizedListenFP collapses wildcard spellings so fingerprint rebuilds
// only when the effective Bind actually changes.
func normalizedListenFP(listen string) string {
	if isWildcardListen(listen) {
		return ""
	}
	addr, ok := parseListenAddr(listen)
	if !ok {
		return "" // unusable → same Bind as wildcard fallback
	}
	if !listenBindable(addr) {
		return ""
	}
	return addr.String()
}
