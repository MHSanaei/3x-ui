package wireguard

import (
	"context"
	"fmt"
	"io"
	"net/netip"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/log"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/proxy/wireguard/gvisortun"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

type tunCreator func(localAddresses []netip.Addr, mtu int, handler promiscuousModeHandler) (Tunnel, error)

type promiscuousModeHandler func(dest net.Destination, conn net.Conn)

type Tunnel interface {
	BuildDevice(ipc string, bind conn.Bind) error
	DialContextTCPAddrPort(ctx context.Context, addr netip.AddrPort) (net.Conn, error)
	DialUDPAddrPort(laddr, raddr netip.AddrPort) (net.Conn, error)
	Close() error
}

type tunnel struct {
	tun    tun.Device
	device *device.Device
	rw     sync.Mutex
}

func (t *tunnel) BuildDevice(ipc string, bind conn.Bind) (err error) {
	t.rw.Lock()
	defer t.rw.Unlock()

	if t.device != nil {
		return errors.New("device is already initialized")
	}

	logger := &device.Logger{
		Verbosef: func(format string, args ...any) {
			log.Record(&log.GeneralMessage{
				Severity: log.Severity_Debug,
				Content:  fmt.Sprintf(format, args...),
			})
		},
		Errorf: func(format string, args ...any) {
			log.Record(&log.GeneralMessage{
				Severity: log.Severity_Error,
				Content:  fmt.Sprintf(format, args...),
			})
		},
	}

	t.device = device.NewDevice(t.tun, bind, logger)
	if err = t.device.IpcSet(ipc); err != nil {
		return err
	}
	if err = t.device.Up(); err != nil {
		return err
	}
	return nil
}

func (t *tunnel) Close() (err error) {
	t.rw.Lock()
	defer t.rw.Unlock()

	if t.device == nil {
		return nil
	}

	t.device.Close()
	t.device = nil
	err = t.tun.Close()
	t.tun = nil
	return nil
}

func CalculateInterfaceName(name string) (tunName string) {
	if runtime.GOOS == "darwin" {
		tunName = "utun"
	} else if name != "" {
		tunName = name
	} else {
		tunName = "tun"
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return
	}
	var tunIndex int
	for _, netInterface := range interfaces {
		if strings.HasPrefix(netInterface.Name, tunName) {
			index, parseErr := strconv.ParseInt(netInterface.Name[len(tunName):], 10, 16)
			if parseErr == nil {
				tunIndex = int(index) + 1
			}
		}
	}
	tunName = fmt.Sprintf("%s%d", tunName, tunIndex)
	return
}

var _ Tunnel = (*gvisorNet)(nil)

type gvisorNet struct {
	tunnel
	net *gvisortun.Net
}

func (g *gvisorNet) Close() error {
	return g.tunnel.Close()
}

func (g *gvisorNet) DialContextTCPAddrPort(ctx context.Context, addr netip.AddrPort) (
	net.Conn, error,
) {
	return g.net.DialContextTCPAddrPort(ctx, addr)
}

func (g *gvisorNet) DialUDPAddrPort(laddr, raddr netip.AddrPort) (net.Conn, error) {
	return g.net.DialUDPAddrPort(laddr, raddr)
}

func createGVisorTun(localAddresses []netip.Addr, mtu int, handler promiscuousModeHandler) (Tunnel, error) {
	out := &gvisorNet{}
	tun, n, gstack, err := gvisortun.CreateNetTUN(localAddresses, mtu, handler != nil)
	if err != nil {
		return nil, err
	}

	if handler != nil {
		// handler is only used for promiscuous mode
		// capture all packets and send to handler

		tcpForwarder := tcp.NewForwarder(gstack, 0, 65535, func(r *tcp.ForwarderRequest) {
			go func(r *tcp.ForwarderRequest) {
				var wq waiter.Queue
				var id = r.ID()

				ep, err := r.CreateEndpoint(&wq)
				if err != nil {
					errors.LogError(context.Background(), err.String())
					r.Complete(true)
					return
				}

				options := ep.SocketOptions()
				options.SetKeepAlive(false)
				options.SetReuseAddress(true)
				options.SetReusePort(true)

				handler(net.TCPDestination(net.IPAddress(id.LocalAddress.AsSlice()), net.Port(id.LocalPort)), gonet.NewTCPConn(&wq, ep))

				ep.Close()
				r.Complete(false)
			}(r)
		})
		gstack.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpForwarder.HandlePacket)

		manager := &udpManager{
			stack:   gstack,
			handler: handler,
			m:       make(map[string]*udpConn),
		}

		gstack.SetTransportProtocolHandler(udp.ProtocolNumber, func(id stack.TransportEndpointID, pkt *stack.PacketBuffer) bool {
			data := pkt.Clone().Data().AsRange().ToSlice()
			// if len(data) == 0 {
			// 	return false
			// }
			srcIP := net.IPAddress(id.RemoteAddress.AsSlice())
			dstIP := net.IPAddress(id.LocalAddress.AsSlice())
			if srcIP == nil || dstIP == nil {
				errors.LogDebug(context.Background(), "drop udp with size ", len(data), " > invalid ip address ", id.RemoteAddress.AsSlice(), " ", id.LocalAddress.AsSlice())
				return true
			}
			src := net.UDPDestination(srcIP, net.Port(id.RemotePort))
			dst := net.UDPDestination(dstIP, net.Port(id.LocalPort))
			manager.feed(src, dst, data)
			return true
		})
	}

	out.tun, out.net = tun, n
	return out, nil
}

type udpManager struct {
	stack   *stack.Stack
	handler func(dest net.Destination, conn net.Conn)
	m       map[string]*udpConn
	mutex   sync.RWMutex
}

func (m *udpManager) feed(src net.Destination, dst net.Destination, data []byte) {
	m.mutex.RLock()
	uc, ok := m.m[src.NetAddr()]
	if ok {
		select {
		case uc.queue <- &packet{
			p:    data,
			dest: &dst,
		}:
		default:
			errors.LogDebug(context.Background(), "drop udp with size ", len(data), " to ", dst.NetAddr(), " original ", uc.dst.NetAddr(), " > queue full")
		}
		m.mutex.RUnlock()
		return
	}
	m.mutex.RUnlock()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	uc, ok = m.m[src.NetAddr()]
	if !ok {
		uc = &udpConn{
			queue: make(chan *packet, 1024),
			src:   src,
			dst:   dst,
		}
		uc.writeFunc = m.writeRawUDPPacket
		uc.closeFunc = func() {
			m.mutex.Lock()
			m.close(uc)
			m.mutex.Unlock()
		}
		m.m[src.NetAddr()] = uc
		go m.handler(dst, uc)
	}

	select {
	case uc.queue <- &packet{
		p:    data,
		dest: &dst,
	}:
	default:
		errors.LogDebug(context.Background(), "drop udp with size ", len(data), " to ", dst.NetAddr(), " original ", uc.dst.NetAddr(), " > queue full")
	}
}

func (m *udpManager) close(uc *udpConn) {
	if !uc.closed {
		uc.closed = true
		close(uc.queue)
		delete(m.m, uc.src.NetAddr())
	}
}

func (m *udpManager) writeRawUDPPacket(payload []byte, src net.Destination, dst net.Destination) error {
	udpLen := header.UDPMinimumSize + len(payload)
	srcIP := tcpip.AddrFromSlice(src.Address.IP())
	dstIP := tcpip.AddrFromSlice(dst.Address.IP())

	// build packet with appropriate IP header size
	isIPv4 := dst.Address.Family().IsIPv4()
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

	// Build UDP header
	udpHdr := header.UDP(pkt.TransportHeader().Push(header.UDPMinimumSize))
	udpHdr.Encode(&header.UDPFields{
		SrcPort: uint16(src.Port),
		DstPort: uint16(dst.Port),
		Length:  uint16(udpLen),
	})

	// Calculate and set UDP checksum
	xsum := header.PseudoHeaderChecksum(header.UDPProtocolNumber, srcIP, dstIP, uint16(udpLen))
	udpHdr.SetChecksum(^udpHdr.CalculateChecksum(checksum.Checksum(payload, xsum)))

	// Build IP header
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

	// dispatch the packet
	err := m.stack.WriteRawPacket(1, ipProtocol, buffer.MakeWithView(pkt.ToView()))
	if err != nil {
		return errors.New("failed to write raw udp packet back to stack err ", err)
	}

	return nil
}

type packet struct {
	p    []byte
	dest *net.Destination
}

type udpConn struct {
	queue     chan *packet
	src       net.Destination
	dst       net.Destination
	writeFunc func(payload []byte, src net.Destination, dst net.Destination) error
	closeFunc func()
	closed    bool
}

func (c *udpConn) ReadMultiBuffer() (buf.MultiBuffer, error) {
	for {
		q, ok := <-c.queue
		if !ok {
			return nil, io.EOF
		}

		b := buf.New()

		_, err := b.Write(q.p)
		if err != nil {
			errors.LogDebugInner(context.Background(), err, "drop udp with size ", len(q.p), " to ", q.dest.NetAddr(), " original ", c.dst.NetAddr())
			b.Release()
			continue
		}

		b.UDP = q.dest

		return buf.MultiBuffer{b}, nil
	}
}

func (c *udpConn) Read(p []byte) (int, error) {
	q, ok := <-c.queue
	if !ok {
		return 0, io.EOF
	}
	n := copy(p, q.p)
	if n != len(q.p) {
		return 0, io.ErrShortBuffer
	}
	return n, nil
}

func (c *udpConn) WriteMultiBuffer(mb buf.MultiBuffer) error {
	for i, b := range mb {
		dst := c.dst
		if b.UDP != nil {
			dst = *b.UDP
		}
		err := c.writeFunc(b.Bytes(), dst, c.src)
		if err != nil {
			buf.ReleaseMulti(mb[i:])
			return err
		}
		b.Release()
	}
	return nil
}

func (c *udpConn) Write(p []byte) (int, error) {
	err := c.writeFunc(p, c.dst, c.src)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *udpConn) Close() error {
	c.closeFunc()
	return nil
}

func (c *udpConn) LocalAddr() net.Addr {
	return c.dst.RawNetAddr()
}

func (c *udpConn) RemoteAddr() net.Addr {
	return c.src.RawNetAddr()
}

func (c *udpConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *udpConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *udpConn) SetWriteDeadline(t time.Time) error {
	return nil
}
