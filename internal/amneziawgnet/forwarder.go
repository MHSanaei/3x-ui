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
// handler both the accepted connection and the tunnel client's real,
// dynamically-arbitrary destination (recovered from the connection's own
// TransportEndpointID, not from any preconfigured routing table). This is
// the mechanism the whole embedded-AmneziaWG design depends on: what the
// handler does with that destination (dial it directly, relay it into
// Xray's SOCKS5 inbound, ...) is entirely up to the caller.
//
// Adapted from xtls/xray-core's proxy/wireguard/tun.go createForwarder (MIT).
func AttachTCPForwarder(gstack *stack.Stack, handler func(conn *gonet.TCPConn, dest netip.AddrPort)) {
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
			// Nagle holds small writes until an ACK frees them; the real
			// client's RTT (not this loopback stack's) is what it waits on.
			ep.SocketOptions().SetDelayOption(false)
			dest := netip.AddrPortFrom(addrFromTcpip(id.LocalAddress), id.LocalPort)
			handler(gonet.NewTCPConn(&wq, ep), dest)
			ep.Close()
			r.Complete(false)
		}(r)
	})
	gstack.SetTransportProtocolHandler(tcp.ProtocolNumber, fwd.HandlePacket)
}
