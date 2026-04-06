package udphop

import (
	"errors"
	"math/rand"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/xtls/xray-core/common/crypto"
	"github.com/xtls/xray-core/transport/internet/finalmask"
)

const (
	packetQueueSize = 1024
	udpBufferSize   = finalmask.UDPSize

	defaultHopInterval = 30 * time.Second
)

type UdpHopPacketConn struct {
	Addr           net.Addr
	Addrs          []net.Addr
	HopIntervalMin int64
	HopIntervalMax int64
	ListenUDPFunc  ListenUDPFunc

	connMutex   sync.RWMutex
	prevConn    net.PacketConn
	currentConn net.PacketConn
	addrIndex   int

	readBufferSize  int
	writeBufferSize int

	recvQueue chan *udpPacket
	closeChan chan struct{}
	closed    bool

	bufPool sync.Pool
}

type udpPacket struct {
	Buf  []byte
	N    int
	Addr net.Addr
	Err  error
}

type ListenUDPFunc = func(*net.UDPAddr) (net.PacketConn, error)

func NewUDPHopPacketConn(addr *UDPHopAddr, index int, intervalMin int64, intervalMax int64, listenUDPFunc ListenUDPFunc, pktConn net.PacketConn) (net.PacketConn, error) {
	if intervalMin == 0 || intervalMax == 0 {
		intervalMin = int64(defaultHopInterval)
		intervalMax = int64(defaultHopInterval)
	}
	if intervalMin < 5 || intervalMax < 5 {
		return nil, errors.New("hop interval must be at least 5 seconds")
	}
	// if listenUDPFunc == nil {
	// 	listenUDPFunc = func() (net.PacketConn, error) {
	// 		return net.ListenUDP("udp", nil)
	// 	}
	// }
	if listenUDPFunc == nil {
		return nil, errors.New("nil listenUDPFunc")
	}
	addrs, err := addr.addrs()
	if err != nil {
		return nil, err
	}
	// curConn, err := listenUDPFunc()
	// if err != nil {
	// 	return nil, err
	// }
	hConn := &UdpHopPacketConn{
		Addr:           addr,
		Addrs:          addrs,
		HopIntervalMin: intervalMin,
		HopIntervalMax: intervalMax,
		ListenUDPFunc:  listenUDPFunc,
		prevConn:       nil,
		currentConn:    pktConn,
		addrIndex:      index,
		recvQueue:      make(chan *udpPacket, packetQueueSize),
		closeChan:      make(chan struct{}),
		bufPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, udpBufferSize)
			},
		},
	}
	go hConn.recvLoop(pktConn)
	go hConn.hopLoop()
	return hConn, nil
}

func (u *UdpHopPacketConn) recvLoop(conn net.PacketConn) {
	for {
		buf := u.bufPool.Get().([]byte)
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			u.bufPool.Put(buf)
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				// Only pass through timeout errors here, not permanent errors
				// like connection closed. Connection close is normal as we close
				// the old connection to exit this loop every time we hop.
				u.recvQueue <- &udpPacket{nil, 0, nil, netErr}
			}
			return
		}
		select {
		case u.recvQueue <- &udpPacket{buf, n, addr, nil}:
			// Packet successfully queued
		default:
			// Queue is full, drop the packet
			u.bufPool.Put(buf)
		}
	}
}

func (u *UdpHopPacketConn) hopLoop() {
	ticker := time.NewTicker(time.Duration(crypto.RandBetween(u.HopIntervalMin, u.HopIntervalMax)) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			u.hop()
			ticker.Reset(time.Duration(crypto.RandBetween(u.HopIntervalMin, u.HopIntervalMax)) * time.Second)
		case <-u.closeChan:
			return
		}
	}
}

func (u *UdpHopPacketConn) hop() {
	u.connMutex.Lock()
	defer u.connMutex.Unlock()
	if u.closed {
		return
	}
	// Update addrIndex to a new random value
	u.addrIndex = rand.Intn(len(u.Addrs))
	newConn, err := u.ListenUDPFunc(u.Addrs[u.addrIndex].(*net.UDPAddr))
	if err != nil {
		// Could be temporary, just skip this hop
		return
	}
	// We need to keep receiving packets from the previous connection,
	// because otherwise there will be packet loss due to the time gap
	// between we hop to a new port and the server acknowledges this change.
	// So we do the following:
	// Close prevConn,
	// move currentConn to prevConn,
	// set newConn as currentConn,
	// start recvLoop on newConn.
	if u.prevConn != nil {
		_ = u.prevConn.Close() // recvLoop for this conn will exit
	}
	u.prevConn = u.currentConn
	u.currentConn = newConn
	// Set buffer sizes if previously set
	if u.readBufferSize > 0 {
		_ = trySetReadBuffer(u.currentConn, u.readBufferSize)
	}
	if u.writeBufferSize > 0 {
		_ = trySetWriteBuffer(u.currentConn, u.writeBufferSize)
	}
	go u.recvLoop(newConn)
}

func (u *UdpHopPacketConn) ReadFrom(b []byte) (n int, addr net.Addr, err error) {
	for {
		select {
		case p := <-u.recvQueue:
			if p.Err != nil {
				return 0, nil, p.Err
			}
			// Currently we do not check whether the packet is from
			// the server or not due to performance reasons.
			n := copy(b, p.Buf[:p.N])
			u.bufPool.Put(p.Buf)
			return n, u.Addr, nil
		case <-u.closeChan:
			return 0, nil, net.ErrClosed
		}
	}
}

func (u *UdpHopPacketConn) WriteTo(b []byte, addr net.Addr) (n int, err error) {
	u.connMutex.RLock()
	defer u.connMutex.RUnlock()
	if u.closed {
		return 0, net.ErrClosed
	}
	// Skip the check for now, always write to the server,
	// for the same reason as in ReadFrom.
	return u.currentConn.WriteTo(b, u.Addrs[u.addrIndex])
}

func (u *UdpHopPacketConn) Close() error {
	u.connMutex.Lock()
	defer u.connMutex.Unlock()
	if u.closed {
		return nil
	}
	// Close prevConn and currentConn
	// Close closeChan to unblock ReadFrom & hopLoop
	// Set closed flag to true to prevent double close
	if u.prevConn != nil {
		_ = u.prevConn.Close()
	}
	err := u.currentConn.Close()
	close(u.closeChan)
	u.closed = true
	u.Addrs = nil // For GC
	return err
}

func (u *UdpHopPacketConn) LocalAddr() net.Addr {
	u.connMutex.RLock()
	defer u.connMutex.RUnlock()
	return u.currentConn.LocalAddr()
}

func (u *UdpHopPacketConn) SetDeadline(t time.Time) error {
	u.connMutex.RLock()
	defer u.connMutex.RUnlock()
	if u.prevConn != nil {
		_ = u.prevConn.SetDeadline(t)
	}
	return u.currentConn.SetDeadline(t)
}

func (u *UdpHopPacketConn) SetReadDeadline(t time.Time) error {
	u.connMutex.RLock()
	defer u.connMutex.RUnlock()
	if u.prevConn != nil {
		_ = u.prevConn.SetReadDeadline(t)
	}
	return u.currentConn.SetReadDeadline(t)
}

func (u *UdpHopPacketConn) SetWriteDeadline(t time.Time) error {
	u.connMutex.RLock()
	defer u.connMutex.RUnlock()
	if u.prevConn != nil {
		_ = u.prevConn.SetWriteDeadline(t)
	}
	return u.currentConn.SetWriteDeadline(t)
}

// UDP-specific methods below

func (u *UdpHopPacketConn) SetReadBuffer(bytes int) error {
	u.connMutex.Lock()
	defer u.connMutex.Unlock()
	u.readBufferSize = bytes
	if u.prevConn != nil {
		_ = trySetReadBuffer(u.prevConn, bytes)
	}
	return trySetReadBuffer(u.currentConn, bytes)
}

func (u *UdpHopPacketConn) SetWriteBuffer(bytes int) error {
	u.connMutex.Lock()
	defer u.connMutex.Unlock()
	u.writeBufferSize = bytes
	if u.prevConn != nil {
		_ = trySetWriteBuffer(u.prevConn, bytes)
	}
	return trySetWriteBuffer(u.currentConn, bytes)
}

func (u *UdpHopPacketConn) SyscallConn() (syscall.RawConn, error) {
	u.connMutex.RLock()
	defer u.connMutex.RUnlock()
	sc, ok := u.currentConn.(syscall.Conn)
	if !ok {
		return nil, errors.New("not supported")
	}
	return sc.SyscallConn()
}

func trySetReadBuffer(pc net.PacketConn, bytes int) error {
	sc, ok := pc.(interface {
		SetReadBuffer(bytes int) error
	})
	if ok {
		return sc.SetReadBuffer(bytes)
	}
	return nil
}

func trySetWriteBuffer(pc net.PacketConn, bytes int) error {
	sc, ok := pc.(interface {
		SetWriteBuffer(bytes int) error
	})
	if ok {
		return sc.SetWriteBuffer(bytes)
	}
	return nil
}
