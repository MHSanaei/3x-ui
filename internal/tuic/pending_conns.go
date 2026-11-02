package tuic

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sync"
)

// A connection is pending from its first Initial packet until it authenticates or closes;
// each one holds handshake state and goroutines, so an unauthenticated flood is bounded here.
const (
	// pendingRetryThreshold is where new connections must first answer a stateless Retry,
	// so a sender that never reads replies, or forges its source, creates no state.
	pendingRetryThreshold = 64
	maxPendingConns       = 1024
	// maxPendingPerSource counts verified sources only (an IPv4 address or IPv6 /64).
	maxPendingPerSource = 32
	// maxPreAuthStreams bounds the stream handlers one connection may start per stream
	// kind before it authenticates.
	maxPreAuthStreams = 16
)

var errTooManyPendingConns = errors.New("tuic: too many connections awaiting authentication")

type pendingConns struct {
	mu       sync.Mutex
	total    int
	bySource map[netip.Prefix]int
}

type pendingSlotKey struct{}

// pendingSlot is one connection's place in the count, released exactly once.
type pendingSlot struct {
	once    sync.Once
	release func()
}

func (p *pendingSlot) done() {
	if p != nil {
		p.once.Do(p.release)
	}
}

func (p *pendingConns) needsRetry(net.Addr) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.total >= pendingRetryThreshold
}

// admit takes a slot for a new connection, or refuses it at the ceilings; the slot frees
// when ctx (the connection's own) ends or the connection authenticates.
func (p *pendingConns) admit(ctx context.Context, info connInfo) (context.Context, error) {
	source, counted := pendingSource(info)
	p.mu.Lock()
	if p.total >= maxPendingConns || (counted && p.bySource[source] >= maxPendingPerSource) {
		p.mu.Unlock()
		return ctx, errTooManyPendingConns
	}
	p.total++
	if counted {
		if p.bySource == nil {
			p.bySource = make(map[netip.Prefix]int)
		}
		p.bySource[source]++
	}
	p.mu.Unlock()
	slot := &pendingSlot{release: func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.total--
		if counted {
			if p.bySource[source]--; p.bySource[source] <= 0 {
				delete(p.bySource, source)
			}
		}
	}}
	context.AfterFunc(ctx, slot.done)
	return context.WithValue(ctx, pendingSlotKey{}, slot), nil
}

// connInfo is what admit needs of quic.ClientInfo.
type connInfo struct {
	remote          net.Addr
	addressVerified bool
}

func pendingSource(info connInfo) (netip.Prefix, bool) {
	udp, ok := info.remote.(*net.UDPAddr)
	if !ok || !info.addressVerified {
		return netip.Prefix{}, false
	}
	addr, ok := netip.AddrFromSlice(udp.IP)
	if !ok {
		return netip.Prefix{}, false
	}
	addr = addr.Unmap()
	bits := 32
	if addr.Is6() {
		bits = 64
	}
	prefix, err := addr.Prefix(bits)
	return prefix, err == nil
}

func releasePendingSlot(ctx context.Context) {
	if slot, ok := ctx.Value(pendingSlotKey{}).(*pendingSlot); ok {
		slot.done()
	}
}
