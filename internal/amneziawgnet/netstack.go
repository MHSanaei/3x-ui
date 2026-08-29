// Package amneziawgnet embeds amneziawg-go and gVisor netstack in-process
// as a userspace alternative to kernel wireguard / awg-quick.
package amneziawgnet

import (
	"fmt"
	"net/netip"
	"os"
	"sync"
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

// tunQueueDepth is the outbound queue depth for channel endpoint and handoff.
const tunQueueDepth = 1024

// stackTun implements amneziawg-go tun.Device over a gVisor channel endpoint,
// exposing *stack.Stack for forwarder attachment.
type stackTun struct {
	ep             *channel.Endpoint
	stack          *stack.Stack
	events         chan awgtun.Event
	notifyHandle   *channel.NotificationHandle
	incomingPacket chan *buffer.View
	done           chan struct{}
	closeMu        sync.Mutex
	closed         bool
	mtu            int
}

// createNetTUNWithStack builds a gVisor-backed tun.Device for localAddresses
// and returns underlying *stack.Stack to attach forwarders.
func createNetTUNWithStack(localAddresses []netip.Addr, mtu int) (awgtun.Device, *stack.Stack, error) {
	opts := stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol, ipv6.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol, icmp.NewProtocol6, icmp.NewProtocol4},
		// HandleLocal stays false so non-local destinations reach forwarder.
		HandleLocal: false,
	}
	dev := &stackTun{
		// tunQueueDepth buffers channel.New and incomingPacket for pipelining.
		ep:             channel.New(tunQueueDepth, uint32(mtu), ""),
		stack:          stack.New(opts),
		events:         make(chan awgtun.Event, 10),
		incomingPacket: make(chan *buffer.View, tunQueueDepth),
		done:           make(chan struct{}),
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

// Read drains incomingPacket into buf, supporting batched reads.
func (t *stackTun) Read(buf [][]byte, sizes []int, offset int) (int, error) {
	var view *buffer.View
	select {
	case <-t.done:
		return 0, os.ErrClosed
	case view = <-t.incomingPacket:
	}
	n, err := view.Read(buf[0][offset:])
	if err != nil {
		return 0, err
	}
	sizes[0] = n
	count := 1
	for count < len(buf) {
		select {
		case view = <-t.incomingPacket:
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

// WriteNotify runs on gVisor dispatch while Close tears the endpoint down,
// so it must never block on closeMu across ep.Read or stack teardown.
func (t *stackTun) WriteNotify() {
	t.closeMu.Lock()
	if t.closed {
		t.closeMu.Unlock()
		return
	}
	t.closeMu.Unlock()

	pkt := t.ep.Read()
	if pkt == nil {
		return
	}
	view := pkt.ToView()
	pkt.DecRef()

	// Select against done so racing dispatch abandons packet on close
	// without blocking Close or panicking on closed channel.
	select {
	case t.incomingPacket <- view:
	case <-t.done:
	}
}

func (t *stackTun) Close() error {
	t.closeMu.Lock()
	if t.closed {
		t.closeMu.Unlock()
		return nil
	}
	t.closed = true
	close(t.done)
	t.closeMu.Unlock()

	t.stack.RemoveNIC(1)
	t.stack.Close()
	t.ep.RemoveNotify(t.notifyHandle)
	t.ep.Close()
	if t.events != nil {
		close(t.events)
	}
	return nil
}

// enablePromiscuousRouting configures NIC promiscuous and spoofing modes.
func enablePromiscuousRouting(gstack *stack.Stack) {
	gstack.SetPromiscuousMode(1, true)
	gstack.SetSpoofing(1, true)
}

// addrFromTcpip converts a gVisor tcpip.Address to netip.Addr.
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
