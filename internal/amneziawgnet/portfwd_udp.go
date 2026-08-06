package amneziawgnet

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/stack"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// portForwardUDPIdleTimeout matches UDPRelay.pump's own idle window
// (relay.go) -- both are "how long to keep a per-flow session alive with no
// traffic before tearing it down," so there's no reason for the two
// directions to disagree.
const portForwardUDPIdleTimeout = 2 * time.Minute

// udpForwardSession is one established flow from a single external source
// address into the tunnel toward a peer -- conn is a connected gonet UDP
// endpoint (DialUDP with a non-nil raddr), so plain Read/Write, not
// ReadFrom/WriteTo, address it correctly.
type udpForwardSession struct {
	conn *gonet.UDPConn
}

// udpForwardListener is one open host-facing UDP socket for a single
// portForwardKey, demultiplexing by external source address -- the mirror
// image of AttachUDPHandler/UDPRelay, which demultiplex by tunnel-internal
// source for the opposite direction. net.ListenPacket has no accept/session
// model of its own, so this package tracks sessions itself here, the same
// way UDPRelay already does in relay.go.
type udpForwardListener struct {
	pc net.PacketConn

	mu       sync.Mutex
	sessions map[netip.AddrPort]*udpForwardSession
}

// listenPortForwardUDP opens a host-facing UDP socket on key.port and
// starts demultiplexing datagrams into per-source-address tunnel sessions
// toward target(key.email). Bind-failure contract matches
// listenPortForwardTCP exactly: log, return nil, Reconcile retries later.
func listenPortForwardUDP(gstack *stack.Stack, inboundID int, key portForwardKey, target portForwardTargetFunc) *udpForwardListener {
	pc, err := (&net.ListenConfig{}).ListenPacket(context.Background(), "udp", fmt.Sprintf(":%d", key.port))
	if err != nil {
		logger.Warningf("amneziawgnet: port-forward: inbound %d peer %q: listen udp :%d: %v", inboundID, key.email, key.port, err)
		return nil
	}
	l := &udpForwardListener{pc: pc, sessions: map[netip.AddrPort]*udpForwardSession{}}
	logger.Infof("amneziawgnet: port-forward: inbound %d peer %q: listening udp :%d", inboundID, key.email, key.port)
	go l.readLoop(gstack, inboundID, key, target)
	return l
}

func (l *udpForwardListener) readLoop(gstack *stack.Stack, inboundID int, key portForwardKey, target portForwardTargetFunc) {
	buf := make([]byte, 65536)
	for {
		n, from, err := l.pc.ReadFrom(buf)
		if err != nil {
			return // closed
		}
		src, ok := udpAddrPort(from)
		if !ok {
			continue
		}

		l.mu.Lock()
		sess, exists := l.sessions[src]
		l.mu.Unlock()

		if !exists {
			addr, ok := target(key.email)
			if !ok {
				continue
			}
			raddr := tunnelFullAddress(addr, key.port)
			conn, err := gonet.DialUDP(gstack, nil, &raddr, tunnelNetwork(addr))
			if err != nil {
				logger.Warningf("amneziawgnet: port-forward: inbound %d peer %q: dial tunnel %s:%d: %v", inboundID, key.email, addr, key.port, err)
				continue
			}
			sess = &udpForwardSession{conn: conn}
			l.mu.Lock()
			l.sessions[src] = sess
			l.mu.Unlock()
			go l.pump(src, sess)
		}
		// buf is reused by the next ReadFrom the instant this loop continues,
		// so the session's own goroutine can't be handed a slice into it --
		// Write copies synchronously here, on this goroutine, before that
		// can happen, so no copy of the payload is needed.
		if _, err := sess.conn.Write(buf[:n]); err != nil {
			logger.Warningf("amneziawgnet: port-forward: inbound %d peer %q: write tunnel: %v", inboundID, key.email, err)
		}
	}
}

// pump reads replies from sess and writes them back to the external source
// src until the session errors out or goes idle, mirroring UDPRelay.pump's
// exact structure (relay.go) for the opposite direction.
func (l *udpForwardListener) pump(src netip.AddrPort, sess *udpForwardSession) {
	defer func() {
		l.mu.Lock()
		delete(l.sessions, src)
		l.mu.Unlock()
		sess.conn.Close()
	}()
	buf := make([]byte, 65536)
	for {
		_ = sess.conn.SetReadDeadline(time.Now().Add(portForwardUDPIdleTimeout))
		n, err := sess.conn.Read(buf)
		if err != nil {
			return
		}
		if _, err := l.pc.WriteTo(buf[:n], net.UDPAddrFromAddrPort(src)); err != nil {
			return
		}
	}
}

// Close tears down every open session and the underlying socket.
func (l *udpForwardListener) Close() {
	l.mu.Lock()
	sessions := l.sessions
	l.sessions = map[netip.AddrPort]*udpForwardSession{}
	l.mu.Unlock()
	for _, sess := range sessions {
		sess.conn.Close()
	}
	l.pc.Close()
}

// udpAddrPort extracts a netip.AddrPort from a net.Addr returned by
// net.ListenPacket's ReadFrom -- always a *net.UDPAddr in practice for a
// "udp" network listener, but handled defensively rather than assumed.
func udpAddrPort(addr net.Addr) (netip.AddrPort, bool) {
	udpAddr, ok := addr.(*net.UDPAddr)
	if !ok {
		return netip.AddrPort{}, false
	}
	return udpAddr.AddrPort(), true
}
