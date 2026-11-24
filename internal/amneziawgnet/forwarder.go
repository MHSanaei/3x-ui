package amneziawgnet

import (
	"net/netip"
	"sync"

	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/waiter"
)

// One datagram or bare SYN from a fresh source port buys a flow its sockets, goroutine and
// buffers, so each peer gets a share of an interface instead of the whole of it.
const (
	maxTCPFlowsPerPeer    = 4096
	maxUDPSessionsPerPeer = 1024
)

// peerSlots counts the flows each peer holds; every successful acquire needs one release.
type peerSlots struct {
	mu    sync.Mutex
	limit int
	held  map[string]int
}

func newPeerSlots(limit int) *peerSlots {
	return &peerSlots{limit: limit, held: map[string]int{}}
}

func (s *peerSlots) acquire(peer string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.held[peer] >= s.limit {
		return false
	}
	s.held[peer]++
	return true
}

func (s *peerSlots) release(peer string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.held[peer]--; s.held[peer] <= 0 {
		delete(s.held, peer)
	}
}

// AttachTCPForwarder attaches a TCP forwarder to gstack in promiscuous +
// spoofing mode, so it accepts connections addressed to any destination --
// not just the stack's own configured local address -- and hands the
// handler the accepted connection, the tunnel client's source and its real,
// dynamically-arbitrary destination (recovered from the connection's own
// TransportEndpointID, not from any preconfigured routing table). This is
// the mechanism the whole embedded-AmneziaWG design depends on: what the
// handler does with that destination (dial it directly, relay it into
// Xray's SOCKS5 inbound, ...) is entirely up to the caller.
//
// Handlers must identify the peer by src, never conn.RemoteAddr(): that asks
// the live endpoint and is nil once a RST is processed, possibly before the call.
//
// owner names the peer a tunnel source belongs to; it is asked on the SYN, before anything
// is allocated, and refuses a source nobody owns or one whose owner holds its full share.
//
// Adapted from xtls/xray-core's proxy/wireguard/tun.go createForwarder (MIT).
func AttachTCPForwarder(gstack *stack.Stack, owner func(src netip.Addr) (string, bool), handler func(conn *gonet.TCPConn, src, dest netip.AddrPort)) {
	enablePromiscuousRouting(gstack)

	slots := newPeerSlots(maxTCPFlowsPerPeer)
	fwd := tcp.NewForwarder(gstack, 0, 65535, func(r *tcp.ForwarderRequest) {
		peer, ok := owner(addrFromTcpip(r.ID().RemoteAddress).Unmap())
		if !ok {
			r.Complete(true)
			return
		}
		// CreateEndpoint holds the request until the handshake ends, so a bare SYN costs a slot.
		if !slots.acquire(peer) {
			r.Complete(false)
			return
		}
		go func(r *tcp.ForwarderRequest) {
			defer slots.release(peer)
			var wq waiter.Queue
			id := r.ID()

			ep, err := r.CreateEndpoint(&wq)
			if err != nil {
				r.Complete(true)
				return
			}
			src := netip.AddrPortFrom(addrFromTcpip(id.RemoteAddress), id.RemotePort)
			dest := netip.AddrPortFrom(addrFromTcpip(id.LocalAddress), id.LocalPort)
			handler(gonet.NewTCPConn(&wq, ep), src, dest)
			ep.Close()
			r.Complete(false)
		}(r)
	})
	gstack.SetTransportProtocolHandler(tcp.ProtocolNumber, fwd.HandlePacket)
}
