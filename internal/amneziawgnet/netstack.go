// Package amneziawgnet embeds amneziawg-go (a userspace AmneziaWG
// implementation, https://github.com/amnezia-vpn/amneziawg-go) directly in
// the panel process, as an alternative to internal/amneziawg's
// kernel-module (DKMS) + awg-quick approach. A gVisor userspace network
// stack (gvisor.dev/gvisor/pkg/tcpip -- already an indirect dependency via
// xray-core's own proxy/wireguard support) terminates each tunnel, and a
// forwarder recovers each connection's real, dynamically-arbitrary
// destination for the caller to relay onward (see Phase 2 of the migration
// plan: a loopback SOCKS5 dial into Xray, giving native stats/routing/
// sniffing for free).
package amneziawgnet

import (
	"fmt"
	"net/netip"
	"os"
	"syscall"

	awgtun "github.com/amnezia-vpn/amneziawg-go/v3/tun"

	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

// tunQueueDepth is the outbound packet queue depth for both the gVisor channel endpoint and the handoff channel to amneziawg-go's TUN reader (see the stackTun literal in createNetTUNWithStack).
// 1024 was too small for several connections' simultaneous TCP slow-start growth; channel.Endpoint drops silently (not blocks) when full, which reads as network loss.
const tunQueueDepth = 8192

// stackTun implements amneziawg-go's tun.Device directly against a gVisor
// channel endpoint, the same approach amneziawg-go's own tun/netstack
// package and xray-core's proxy/wireguard/netstack.go both take. Neither of
// those exposes the raw *stack.Stack a forwarder needs (amneziawg-go's Net
// type keeps it unexported), so this is a local, from-source reimplementation
// rather than a wrapper -- adapted from amneziawg-go v3.0.3's
// tun/netstack/tun.go (MIT licensed), trimmed to the constructor this
// package needs.
type stackTun struct {
	ep             *channel.Endpoint
	stack          *stack.Stack
	events         chan awgtun.Event
	notifyHandle   *channel.NotificationHandle
	incomingPacket chan *buffer.View
	mtu            int
}

// createNetTUNWithStack builds a gVisor-backed tun.Device for the given
// local addresses (interface address(es), one per family) and returns the
// underlying *stack.Stack alongside it so a caller can attach a forwarder
// (see forwarder.go / udp.go).
func createNetTUNWithStack(localAddresses []netip.Addr, mtu int) (awgtun.Device, *stack.Stack, error) {
	opts := stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol, ipv6.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol, icmp.NewProtocol6, icmp.NewProtocol4},
		// HandleLocal must stay false: promiscuous+spoofing mode (see
		// forwarder.go) is what lets a destination other than the stack's
		// own configured address reach the forwarder at all.
		HandleLocal: false,
	}
	dev := &stackTun{
		// tunQueueDepth matches channel.New's own outbound queue depth
		// below. WriteNotify (called synchronously from whatever gVisor
		// goroutine is sending TCP data for the download/server->client
		// direction) pushes into incomingPacket; RoutineReadFromTUN (a
		// single amneziawg-go goroutine that encrypts and sends each
		// packet over UDP) is the only reader. With no buffer, every
		// outbound packet forced a full synchronous handoff between the
		// two -- gVisor's sender blocked until the encrypt loop was ready
		// for the next one, one packet at a time, no pipelining. The
		// upload/client->server direction has no equivalent stall:
		// Write->InjectInbound->DeliverNetworkPacket hands off into
		// gVisor's own ~1MB per-connection TCP receive buffer and returns
		// immediately. Buffering this channel gives the download
		// direction the same slack the upload direction already had.
		ep:             channel.New(tunQueueDepth, uint32(mtu), ""),
		stack:          stack.New(opts),
		events:         make(chan awgtun.Event, 10),
		incomingPacket: make(chan *buffer.View, tunQueueDepth),
		mtu:            mtu,
	}
	sackEnabledOpt := tcpip.TCPSACKEnabled(true)
	if err := dev.stack.SetTransportProtocolOption(tcp.ProtocolNumber, &sackEnabledOpt); err != nil {
		return nil, nil, fmt.Errorf("amneziawgnet: enable TCP SACK: %s", err)
	}
	dev.notifyHandle = dev.ep.AddNotify(dev)
	if err := dev.stack.CreateNIC(1, dev.ep); err != nil {
		return nil, nil, fmt.Errorf("amneziawgnet: CreateNIC: %s", err)
	}

	var hasV4, hasV6 bool
	for _, ip := range localAddresses {
		var protoNumber tcpip.NetworkProtocolNumber
		switch {
		case ip.Is4():
			protoNumber = ipv4.ProtocolNumber
			hasV4 = true
		case ip.Is6():
			protoNumber = ipv6.ProtocolNumber
			hasV6 = true
		default:
			continue
		}
		protoAddr := tcpip.ProtocolAddress{
			Protocol:          protoNumber,
			AddressWithPrefix: tcpip.AddrFromSlice(ip.AsSlice()).WithPrefix(),
		}
		if err := dev.stack.AddProtocolAddress(1, protoAddr, stack.AddressProperties{}); err != nil {
			return nil, nil, fmt.Errorf("amneziawgnet: AddProtocolAddress(%v): %s", ip, err)
		}
	}
	if hasV4 {
		dev.stack.AddRoute(tcpip.Route{Destination: header.IPv4EmptySubnet, NIC: 1})
	}
	if hasV6 {
		dev.stack.AddRoute(tcpip.Route{Destination: header.IPv6EmptySubnet, NIC: 1})
	}
	dev.events <- awgtun.EventUp
	return dev, dev.stack, nil
}

func (t *stackTun) Name() (string, error)       { return "amneziawgnet", nil }
func (t *stackTun) File() *os.File              { return nil }
func (t *stackTun) Events() <-chan awgtun.Event { return t.events }
func (t *stackTun) MTU() (int, error)           { return t.mtu, nil }
func (t *stackTun) BatchSize() int              { return 1 }

// Read blocks for the first packet, then opportunistically drains any more
// that are already buffered (non-blocking), up to len(buf). amneziawg-go's
// caller (RoutineReadFromTUN) sizes buf/sizes to device.BatchSize(), which
// is the UDP bind's own batch size (128 on Linux, see conn.IdealBatchSize)
// since that's larger than BatchSize()'s 1 below -- so real buffer capacity
// for a batch is already there. Without this drain loop, Read always
// returned exactly one packet no matter how many buf could hold, so every
// downstream step (peer lookup, per-peer staging, and ultimately the UDP
// bind's own genuinely batched Send/sendmmsg) processed the download
// direction one packet at a time while the upload direction's equivalent
// (bind.Receive/recvmmsg -> decrypt -> stackTun.Write, which already loops
// over its whole buf) processed up to 128 per cycle. That asymmetry is
// real, not gVisor/amneziawg-go's -- both the receive and send paths on the
// UDP bind support batching identically, only this Read implementation
// didn't use it.
func (t *stackTun) Read(buf [][]byte, sizes []int, offset int) (int, error) {
	view, ok := <-t.incomingPacket
	if !ok {
		return 0, os.ErrClosed
	}
	n, err := view.Read(buf[0][offset:])
	if err != nil {
		return 0, err
	}
	sizes[0] = n
	count := 1
	for count < len(buf) {
		select {
		case view, ok := <-t.incomingPacket:
			if !ok {
				return count, nil
			}
			n, err := view.Read(buf[count][offset:])
			if err != nil {
				return count, nil
			}
			sizes[count] = n
			count++
		default:
			return count, nil
		}
	}
	return count, nil
}

func (t *stackTun) Write(buf [][]byte, offset int) (int, error) {
	for _, b := range buf {
		packet := b[offset:]
		if len(packet) == 0 {
			continue
		}
		pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{Payload: buffer.MakeWithData(packet)})
		switch packet[0] >> 4 {
		case 4:
			t.ep.InjectInbound(header.IPv4ProtocolNumber, pkb)
		case 6:
			t.ep.InjectInbound(header.IPv6ProtocolNumber, pkb)
		default:
			return 0, syscall.EAFNOSUPPORT
		}
	}
	return len(buf), nil
}

func (t *stackTun) WriteNotify() {
	pkt := t.ep.Read()
	if pkt == nil {
		return
	}
	view := pkt.ToView()
	pkt.DecRef()
	t.incomingPacket <- view
}

func (t *stackTun) Close() error {
	t.stack.RemoveNIC(1)
	t.stack.Close()
	t.ep.RemoveNotify(t.notifyHandle)
	t.ep.Close()
	if t.events != nil {
		close(t.events)
	}
	if t.incomingPacket != nil {
		close(t.incomingPacket)
	}
	return nil
}

// enablePromiscuousRouting puts the NIC into promiscuous + spoofing mode,
// the precondition both AttachTCPForwarder and AttachUDPHandler need to see
// packets addressed to a destination other than the stack's own configured
// local address. Safe to call from both (and more than once): gVisor's
// SetPromiscuousMode/SetSpoofing just set a bool on the NIC, not something
// that accumulates or needs undoing between calls.
func enablePromiscuousRouting(gstack *stack.Stack) {
	gstack.SetPromiscuousMode(1, true)
	gstack.SetSpoofing(1, true)
}

// addrFromTcpip converts a gVisor tcpip.Address (4 or 16 raw bytes) to the
// stdlib netip.Addr type the rest of this package and its callers use.
func addrFromTcpip(a tcpip.Address) netip.Addr {
	if a.Len() == 4 {
		var b [4]byte
		copy(b[:], a.AsSlice())
		return netip.AddrFrom4(b)
	}
	var b [16]byte
	copy(b[:], a.AsSlice())
	return netip.AddrFrom16(b)
}
