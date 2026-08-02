package amneziawgnet

import (
	"fmt"
	"net/netip"

	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

// UDPHandler is called for every UDP packet a tunnel client sends, with its
// source (the peer's tunnel-internal address) and its real,
// dynamically-arbitrary destination -- recovered the same way the TCP
// forwarder recovers its destination, from the packet's own transport
// endpoint ID, never from a preconfigured table. The handler owns all flow
// tracking and reply delivery (via WriteUDPReply): gVisor has no
// udp.NewForwarder the way it does for TCP, so unlike AttachTCPForwarder
// this can't just hand back a ready net.Conn.
type UDPHandler func(src, dst netip.AddrPort, payload []byte)

// AttachUDPHandler attaches a raw UDP handler to gstack, independently
// enabling the same promiscuous+spoofing mode AttachTCPForwarder needs --
// safe and idempotent to call regardless of whether AttachTCPForwarder was
// attached to the same stack first, or at all. Adapted from xtls/xray-core's
// proxy/wireguard/tun.go UDP path (MIT), which hand-tracks flows for the
// identical reason: gVisor doesn't provide a UDP forwarder.
func AttachUDPHandler(gstack *stack.Stack, handler UDPHandler) {
	enablePromiscuousRouting(gstack)

	gstack.SetTransportProtocolHandler(udp.ProtocolNumber, func(id stack.TransportEndpointID, pkt *stack.PacketBuffer) bool {
		data := pkt.Clone().Data().AsRange().ToSlice()
		src := netip.AddrPortFrom(addrFromTcpip(id.RemoteAddress), id.RemotePort)
		dst := netip.AddrPortFrom(addrFromTcpip(id.LocalAddress), id.LocalPort)
		handler(src, dst, data)
		return true
	})
}

// WriteUDPReply injects a UDP packet into gstack as if it arrived from
// `from` addressed to `to` -- i.e. a reply travelling back into the tunnel
// toward the client -- constructed by hand since gVisor exposes no
// connected-socket-style Write for an address the stack doesn't itself own.
func WriteUDPReply(gstack *stack.Stack, from, to netip.AddrPort, payload []byte) error {
	udpLen := header.UDPMinimumSize + len(payload)
	srcIP := tcpip.AddrFromSlice(from.Addr().AsSlice())
	dstIP := tcpip.AddrFromSlice(to.Addr().AsSlice())

	isIPv4 := from.Addr().Is4()
	ipHdrSize := header.IPv6MinimumSize
	ipProtocol := header.IPv6ProtocolNumber
	if isIPv4 {
		ipHdrSize = header.IPv4MinimumSize
		ipProtocol = header.IPv4ProtocolNumber
	}

	pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
		ReserveHeaderBytes: ipHdrSize + header.UDPMinimumSize,
		Payload:            buffer.MakeWithData(payload),
	})
	defer pkt.DecRef()

	udpHdr := header.UDP(pkt.TransportHeader().Push(header.UDPMinimumSize))
	udpHdr.Encode(&header.UDPFields{
		SrcPort: from.Port(),
		DstPort: to.Port(),
		Length:  uint16(udpLen),
	})
	xsum := header.PseudoHeaderChecksum(header.UDPProtocolNumber, srcIP, dstIP, uint16(udpLen))
	udpHdr.SetChecksum(^udpHdr.CalculateChecksum(checksum.Checksum(payload, xsum)))

	if isIPv4 {
		ipHdr := header.IPv4(pkt.NetworkHeader().Push(header.IPv4MinimumSize))
		ipHdr.Encode(&header.IPv4Fields{
			TotalLength: uint16(header.IPv4MinimumSize + udpLen),
			TTL:         64,
			Protocol:    uint8(header.UDPProtocolNumber),
			SrcAddr:     srcIP,
			DstAddr:     dstIP,
		})
		ipHdr.SetChecksum(^ipHdr.CalculateChecksum())
	} else {
		ipHdr := header.IPv6(pkt.NetworkHeader().Push(header.IPv6MinimumSize))
		ipHdr.Encode(&header.IPv6Fields{
			PayloadLength:     uint16(udpLen),
			TransportProtocol: header.UDPProtocolNumber,
			HopLimit:          64,
			SrcAddr:           srcIP,
			DstAddr:           dstIP,
		})
	}

	if tcpipErr := gstack.WriteRawPacket(1, ipProtocol, buffer.MakeWithView(pkt.ToView())); tcpipErr != nil {
		return fmt.Errorf("amneziawgnet: WriteRawPacket: %s", tcpipErr)
	}
	return nil
}
