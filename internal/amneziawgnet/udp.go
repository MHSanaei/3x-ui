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
		// ToSlice already returns an owned copy, so cloning pkt here would only
		// strand a pooled packet buffer and its chunks on every datagram.
		data := pkt.Data().AsRange().ToSlice()
		src := netip.AddrPortFrom(addrFromTcpip(id.RemoteAddress), id.RemotePort)
		dst := netip.AddrPortFrom(addrFromTcpip(id.LocalAddress), id.LocalPort)
		handler(src, dst, data)
		return true
	})
}

// WriteUDPReply sends replies through gVisor's network route,
// which applies the NIC MTU and fragments locally generated IP datagrams.
func WriteUDPReply(gstack *stack.Stack, from, to netip.AddrPort, payload []byte) error {
	if !from.Addr().IsValid() || !to.Addr().IsValid() || from.Addr().Zone() != "" || to.Addr().Zone() != "" || from.Addr().Is4() != to.Addr().Is4() {
		return fmt.Errorf("amneziawgnet: invalid UDP reply addresses: %s -> %s", from, to)
	}
	udpLen := header.UDPMinimumSize + len(payload)
	if udpLen > 65535 {
		return fmt.Errorf("amneziawgnet: UDP reply too large: %d bytes", udpLen)
	}
	srcIP := tcpip.AddrFromSlice(from.Addr().AsSlice())
	dstIP := tcpip.AddrFromSlice(to.Addr().AsSlice())

	ipHdrSize := header.IPv6MinimumSize
	ipProtocol := header.IPv6ProtocolNumber
	if from.Addr().Is4() {
		ipHdrSize = header.IPv4MinimumSize
		ipProtocol = header.IPv4ProtocolNumber
		if ipHdrSize+udpLen > 65535 {
			return fmt.Errorf("amneziawgnet: IPv4 reply too large: %d bytes", ipHdrSize+udpLen)
		}
	}
	route, tcpipErr := gstack.FindRoute(1, srcIP, dstIP, ipProtocol, false)
	if tcpipErr != nil {
		return fmt.Errorf("amneziawgnet: FindRoute: %s", tcpipErr)
	}
	defer route.Release()

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
	udpChecksum := ^udpHdr.CalculateChecksum(checksum.Checksum(payload, xsum))
	if udpChecksum == 0 {
		udpChecksum = 0xffff
	}
	udpHdr.SetChecksum(udpChecksum)

	if tcpipErr := route.WritePacket(stack.NetworkHeaderParams{
		Protocol: header.UDPProtocolNumber,
		TTL:      64,
	}, pkt); tcpipErr != nil {
		return fmt.Errorf("amneziawgnet: WritePacket: %s", tcpipErr)
	}
	return nil
}
