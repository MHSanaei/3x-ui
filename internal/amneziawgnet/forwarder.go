package amneziawgnet

import (
	"net/netip"

	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/waiter"
)

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
// Adapted from xtls/xray-core's proxy/wireguard/tun.go createForwarder (MIT).
func AttachTCPForwarder(gstack *stack.Stack, handler func(conn *gonet.TCPConn, src, dest netip.AddrPort)) {
	enablePromiscuousRouting(gstack)

	fwd := tcp.NewForwarder(gstack, 0, 65535, func(r *tcp.ForwarderRequest) {
		go func(r *tcp.ForwarderRequest) {
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
