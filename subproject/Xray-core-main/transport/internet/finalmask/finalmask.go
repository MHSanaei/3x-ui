package finalmask

import (
	"context"
	"net"
	"sync"

	"github.com/xtls/xray-core/common/errors"
)

type Udpmask interface {
	UDP()

	WrapPacketConnClient(raw net.PacketConn, level int, levelCount int) (net.PacketConn, error)
	WrapPacketConnServer(raw net.PacketConn, level int, levelCount int) (net.PacketConn, error)
}

type UdpmaskManager struct {
	udpmasks []Udpmask
}

func NewUdpmaskManager(udpmasks []Udpmask) *UdpmaskManager {
	return &UdpmaskManager{
		udpmasks: udpmasks,
	}
}

func (m *UdpmaskManager) WrapPacketConnClient(raw net.PacketConn) (net.PacketConn, error) {
	var sizes []int
	var conns []net.PacketConn
	for i, mask := range m.udpmasks {
		if _, ok := mask.(headerConn); ok {
			conn, err := mask.WrapPacketConnClient(nil, i, len(m.udpmasks)-1)
			if err != nil {
				return nil, err
			}
			sizes = append(sizes, conn.(headerSize).Size())
			conns = append(conns, conn)
		} else {
			if len(conns) > 0 {
				raw = &headerManagerConn{sizes: sizes, conns: conns, PacketConn: raw}
				sizes = nil
				conns = nil
			}
			var err error
			raw, err = mask.WrapPacketConnClient(raw, i, len(m.udpmasks)-1)
			if err != nil {
				return nil, err
			}
		}
	}

	if len(conns) > 0 {
		raw = &headerManagerConn{sizes: sizes, conns: conns, PacketConn: raw}
		sizes = nil
		conns = nil
	}
	return raw, nil
}

func (m *UdpmaskManager) WrapPacketConnServer(raw net.PacketConn) (net.PacketConn, error) {
	var sizes []int
	var conns []net.PacketConn
	for i, mask := range m.udpmasks {
		if _, ok := mask.(headerConn); ok {
			conn, err := mask.WrapPacketConnServer(nil, i, len(m.udpmasks)-1)
			if err != nil {
				return nil, err
			}
			sizes = append(sizes, conn.(headerSize).Size())
			conns = append(conns, conn)
		} else {
			if len(conns) > 0 {
				raw = &headerManagerConn{sizes: sizes, conns: conns, PacketConn: raw}
				sizes = nil
				conns = nil
			}
			var err error
			raw, err = mask.WrapPacketConnServer(raw, i, len(m.udpmasks)-1)
			if err != nil {
				return nil, err
			}
		}
	}

	if len(conns) > 0 {
		raw = &headerManagerConn{sizes: sizes, conns: conns, PacketConn: raw}
		sizes = nil
		conns = nil
	}
	return raw, nil
}

const (
	UDPSize = 4096
)

type headerConn interface {
	HeaderConn()
}

type headerSize interface {
	Size() int
}

type headerManagerConn struct {
	sizes []int
	conns []net.PacketConn
	net.PacketConn
	m        sync.Mutex
	writeBuf [UDPSize]byte
}

func (c *headerManagerConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	buf := p
	if len(buf) < UDPSize {
		buf = make([]byte, UDPSize)
	}

	n, addr, err = c.PacketConn.ReadFrom(buf)
	if n == 0 || err != nil {
		return 0, addr, err
	}
	newBuf := buf[:n]

	sum := 0
	for _, size := range c.sizes {
		sum += size
	}

	if n < sum {
		errors.LogDebug(context.Background(), addr, " mask read err short length")
		return 0, addr, nil
	}

	for i := range c.conns {
		n, _, err = c.conns[i].ReadFrom(newBuf)
		if n == 0 || err != nil {
			errors.LogDebug(context.Background(), addr, " mask read err ", err)
			return 0, addr, nil
		}
		newBuf = newBuf[c.sizes[i] : n+c.sizes[i]]
	}

	if len(p) < n {
		errors.LogDebug(context.Background(), addr, " mask read err short buffer")
		return 0, addr, nil
	}

	copy(p, buf[sum:sum+n])

	return n, addr, nil
}

func (c *headerManagerConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	c.m.Lock()
	defer c.m.Unlock()

	sum := 0
	for _, size := range c.sizes {
		sum += size
	}

	if sum+len(p) > UDPSize {
		errors.LogDebug(context.Background(), addr, " mask write err short write")
		return 0, nil
	}

	n = copy(c.writeBuf[sum:], p)

	for i := len(c.conns) - 1; i >= 0; i-- {
		n, err = c.conns[i].WriteTo(c.writeBuf[sum-c.sizes[i]:n+sum], nil)
		if n == 0 || err != nil {
			errors.LogDebug(context.Background(), addr, " mask write err ", err)
			return 0, nil
		}
		sum -= c.sizes[i]
	}

	n, err = c.PacketConn.WriteTo(c.writeBuf[:n], addr)
	if n == 0 || err != nil {
		return n, err
	}

	return len(p), nil
}

type Tcpmask interface {
	TCP()

	WrapConnClient(net.Conn) (net.Conn, error)
	WrapConnServer(net.Conn) (net.Conn, error)
}

type TcpmaskManager struct {
	tcpmasks []Tcpmask
}

func NewTcpmaskManager(tcpmasks []Tcpmask) *TcpmaskManager {
	return &TcpmaskManager{
		tcpmasks: tcpmasks,
	}
}

func (m *TcpmaskManager) WrapConnClient(raw net.Conn) (net.Conn, error) {
	var err error
	for _, mask := range m.tcpmasks {
		raw, err = mask.WrapConnClient(raw)
		if err != nil {
			return nil, err
		}
	}
	return raw, nil
}

func (m *TcpmaskManager) WrapConnServer(raw net.Conn) (net.Conn, error) {
	var err error
	for _, mask := range m.tcpmasks {
		raw, err = mask.WrapConnServer(raw)
		if err != nil {
			return nil, err
		}
	}
	return raw, nil
}

func (m *TcpmaskManager) WrapListener(l net.Listener) (net.Listener, error) {
	return NewTcpListener(m, l)
}

type tcpListener struct {
	m *TcpmaskManager
	net.Listener
}

func NewTcpListener(m *TcpmaskManager, l net.Listener) (net.Listener, error) {
	return &tcpListener{
		m:        m,
		Listener: l,
	}, nil
}

func (l *tcpListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return conn, err
	}

	newConn, err := l.m.WrapConnServer(conn)
	if err != nil {
		errors.LogDebugInner(context.Background(), err, "mask err")
		// conn.Close()
		return conn, nil
	}

	return newConn, nil
}

type TcpMaskConn interface {
	TcpMaskConn()
	RawConn() net.Conn
	Splice() bool
}

func UnwrapTcpMask(conn net.Conn) net.Conn {
	for {
		if v, ok := conn.(TcpMaskConn); ok {
			if !v.Splice() {
				return conn
			}
			conn = v.RawConn()
		} else {
			return conn
		}
	}
}
