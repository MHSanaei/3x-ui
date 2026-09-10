package amneziawgnet

import (
	"context"
	"crypto/hmac"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// EgressBasePort is the fixed loopback port of the panel's SOCKS5 egress
// server; it appears in every generated amneziawg socks bridge.
const EgressBasePort = 64900

// socks5EgressServer is a minimal loopback SOCKS5 server routing Xray's
// bridged amneziawg outbounds into their embedded devices' netstacks.
type socks5EgressServer struct {
	mu      sync.Mutex
	stacks  map[string]*Device // outbound tag -> its device
	dns     map[string]string  // outbound tag -> its DNS server
	tracked map[net.Conn]struct{}

	// dnsServer resolves domain targets through the outbound netstack.
	dnsServer string

	listener net.Listener  // nil when stopped; acceptLoop takes it as an arg
	closing  chan struct{} // per-listener lifetime signal, rearmed by Listen
	wg       sync.WaitGroup
}

var (
	egressOnce   sync.Once
	egressServer *socks5EgressServer
)

// GetEgressServer returns the process-wide SOCKS5 egress server singleton.
func GetEgressServer() *socks5EgressServer {
	egressOnce.Do(func() {
		egressServer = &socks5EgressServer{
			stacks:  map[string]*Device{},
			dns:     map[string]string{},
			tracked: map[net.Conn]struct{}{},
		}
	})
	return egressServer
}

// currentDNSServer reads dnsServer under lock -- custom per-tag DNS preferred.
func (s *socks5EgressServer) currentDNSServer(tag ...string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(tag) > 0 && tag[0] != "" {
		if custom, ok := s.dns[tag[0]]; ok && custom != "" {
			return custom
		}
	}
	return s.dnsServer
}

// SetDNSServer overrides the domain-target resolver (tests).
func (s *socks5EgressServer) SetDNSServer(addr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dnsServer = addr
}

// SetStack registers or replaces the device backing an outbound tag.
func (s *socks5EgressServer) SetStack(tag string, dev *Device, dnsServer ...string) {
	norm := ""
	if len(dnsServer) > 0 && dnsServer[0] != "" {
		norm = normalizeDNSServer(dnsServer[0])
	}
	s.mu.Lock()
	prevDev := s.stacks[tag]
	prevDNS := s.dns[tag]
	s.stacks[tag] = dev
	if norm != "" {
		s.dns[tag] = norm
	} else {
		delete(s.dns, tag)
	}
	changed := prevDev != dev || prevDNS != norm
	s.mu.Unlock()
	if changed {
		flushTunnelDNSCacheForTag(tag)
	}
}

// DeleteStack drops an outbound tag's registration (outbound removed).
func (s *socks5EgressServer) DeleteStack(tag string) {
	s.mu.Lock()
	delete(s.stacks, tag)
	delete(s.dns, tag)
	s.mu.Unlock()
	flushTunnelDNSCacheForTag(tag)
}

// Listen starts accepting on the loopback listener. Idempotent; a bind
// failure is returned and retried by the caller's reconcile tick.
func (s *socks5EgressServer) Listen() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return nil
	}
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", EgressBasePort))
	if err != nil {
		return fmt.Errorf("amneziawgnet: egress listen: %w", err)
	}
	s.listener = ln
	s.closing = make(chan struct{})
	logger.Infof("amneziawgnet: egress socks listening on %s", ln.Addr())
	s.wg.Add(1)
	go s.acceptLoop(ln, s.closing)
	return nil
}

// Close stops the listener and in-flight handlers; signal first so an accept
// error always observes closing.
func (s *socks5EgressServer) Close() {
	s.mu.Lock()
	ln := s.listener
	s.listener = nil
	if ln == nil {
		s.mu.Unlock()
		return
	}
	close(s.closing)
	tracked := s.tracked
	s.tracked = map[net.Conn]struct{}{}
	s.mu.Unlock()

	ln.Close()
	for conn := range tracked {
		conn.Close()
	}
	s.wg.Wait()
}

func (s *socks5EgressServer) acceptLoop(ln net.Listener, closing chan struct{}) {
	defer s.wg.Done()
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-closing:
				return
			default:
			}
			logger.Warningf("amneziawgnet: egress accept: %v", err)
			continue
		}
		select {
		case <-closing:
			conn.Close()
			return
		default:
		}
		s.mu.Lock()
		if s.listener == nil {
			s.mu.Unlock()
			conn.Close()
			return
		}
		s.tracked[conn] = struct{}{}
		s.wg.Add(1)
		s.mu.Unlock()
		go func(c net.Conn) {
			defer s.wg.Done()
			defer func() {
				s.mu.Lock()
				delete(s.tracked, c)
				s.mu.Unlock()
			}()
			s.handleConn(c)
		}(conn)
	}
}

// stackFor resolves a tag to its live device at use time, so rebuilds take
// effect for new connections without touching the listener.
func (s *socks5EgressServer) stackFor(tag string) (*Device, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dev, ok := s.stacks[tag]
	return dev, ok
}

func (s *socks5EgressServer) handleConn(conn net.Conn) {
	defer conn.Close()

	// Bound the pre-auth handshake so a silent client never pins a handler
	// indefinitely across Close() and wg.Wait().
	_ = conn.SetDeadline(time.Now().Add(portForwardDialTimeout))
	method, err := socks5Greeting(conn)
	if err != nil || method == 0xFF {
		return
	}
	user := ""
	if method == 0x02 {
		// RFC 1929 sub-negotiation: VER(1) | ULEN(1) | UNAME | PLEN(1) |
		// PASSWD -- the leading 0x01 version byte must be consumed first.
		var ver [1]byte
		if _, err := io.ReadFull(conn, ver[:]); err != nil {
			return
		}
		var ulen [1]byte
		if _, err := io.ReadFull(conn, ulen[:]); err != nil {
			return
		}
		uname := make([]byte, ulen[0])
		if _, err := io.ReadFull(conn, uname); err != nil {
			return
		}
		user = string(uname)
		var plen [1]byte
		if _, err := io.ReadFull(conn, plen[:]); err != nil {
			return
		}
		pass := make([]byte, plen[0])
		if _, err := io.ReadFull(conn, pass); err != nil {
			return
		}
		if !hmac.Equal(pass, []byte(SocksPassword())) {
			_, _ = conn.Write([]byte{0x01, 0x01})
			return
		}
		if _, err := conn.Write([]byte{0x01, 0x00}); err != nil {
			return
		}
	}

	var req [4]byte
	if _, err := io.ReadFull(conn, req[:]); err != nil {
		return
	}
	// Handshake complete: clear deadline for the relay phase.
	_ = conn.SetDeadline(time.Time{})
	target, err := readSocksRequestTarget(conn, req[3])
	if err != nil {
		writeSocksReply(conn, 0x01, netip.AddrPort{})
		return
	}

	switch req[1] {
	case 0x01: // CONNECT
		dev, ok := s.stackFor(user)
		if !ok {
			writeSocksReply(conn, 0x05, netip.AddrPort{})
			return
		}
		dest, err := target.resolveTunnelVia(s.currentDNSServer(user), user, dev)
		if err != nil {
			logger.Warningf("amneziawgnet: egress %q: resolve %s: %v", user, target, err)
			writeSocksReply(conn, 0x04, netip.AddrPort{})
			return
		}
		s.relayTCP(dev, user, conn, dest)
	case 0x03: // UDP ASSOCIATE
		dev, ok := s.stackFor(user)
		if !ok {
			writeSocksReply(conn, 0x05, netip.AddrPort{})
			return
		}
		s.relayUDP(dev, user, udpControl{conn: conn}, target)
	default:
		writeSocksReply(conn, 0x07, netip.AddrPort{})
	}
}

// socks5Greeting requires RFC 1929 username/password auth (0x02).
// Returns 0xFF when unauthenticated or unsupported.
func socks5Greeting(conn net.Conn) (byte, error) {
	var hdr [2]byte
	if _, err := io.ReadFull(conn, hdr[:]); err != nil {
		return 0xFF, err
	}
	methods := make([]byte, hdr[1])
	if _, err := io.ReadFull(conn, methods); err != nil {
		return 0xFF, err
	}
	hasUserPass := false
	for _, m := range methods {
		if m == 0x02 {
			hasUserPass = true
			break
		}
	}
	if !hasUserPass {
		_, _ = conn.Write([]byte{0x05, 0xFF})
		return 0xFF, nil
	}
	if _, err := conn.Write([]byte{0x05, 0x02}); err != nil {
		return 0xFF, err
	}
	return 0x02, nil
}

// socksTarget is a parsed SOCKS5 request address: an IP, or the raw hostname
// for ATYP 0x03 (resolved through the outbound's tunnel, never host-side).
type socksTarget struct {
	host string
	ip   netip.Addr
	port uint16
}

func readSocksRequestTarget(r io.Reader, atyp byte) (socksTarget, error) {
	var t socksTarget
	switch atyp {
	case 0x01:
		var b [4]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return t, err
		}
		t.ip = netip.AddrFrom4(b)
	case 0x04:
		var b [16]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return t, err
		}
		t.ip = netip.AddrFrom16(b)
	case 0x03:
		var l [1]byte
		if _, err := io.ReadFull(r, l[:]); err != nil {
			return t, err
		}
		name := make([]byte, l[0])
		if _, err := io.ReadFull(r, name); err != nil {
			return t, err
		}
		t.host = string(name)
	default:
		return t, fmt.Errorf("unsupported SOCKS5 request address type %d", atyp)
	}
	var portBytes [2]byte
	if _, err := io.ReadFull(r, portBytes[:]); err != nil {
		return t, err
	}
	t.port = binary.BigEndian.Uint16(portBytes[:])
	return t, nil
}

func (t socksTarget) String() string {
	if t.ip.IsValid() {
		return netip.AddrPortFrom(t.ip, t.port).String()
	}
	return fmt.Sprintf("%s:%d", t.host, t.port)
}

// Domain targets resolve via the tunnel; reply-side helper must not be used here.
func (t socksTarget) resolveTunnelVia(dnsServer, tag string, dev *Device) (netip.AddrPort, error) {
	if t.ip.IsValid() {
		return netip.AddrPortFrom(t.ip, t.port), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), tunnelResolveTimeout)
	defer cancel()
	addr, err := resolveTunnelVia(ctx, dev, tag, dnsServer, t.host)
	if err != nil {
		return netip.AddrPort{}, err
	}
	return netip.AddrPortFrom(addr, t.port), nil
}

// writeSocksReply emits a reply with an empty v4 bind address; Xray only
// reads the code byte.
func writeSocksReply(w io.Writer, code byte, _ netip.AddrPort) {
	out := []byte{0x05, code, 0x00, 0x01, 0, 0, 0, 0, 0, 0}
	_, _ = w.Write(out)
}

// relayTCP dials dest inside the tagged outbound's netstack and pipes both
// directions until either side closes.
func (s *socks5EgressServer) relayTCP(dev *Device, tag string, upstream net.Conn, dest netip.AddrPort) {
	fa := tcpip.FullAddress{
		NIC:  1,
		Addr: tcpip.AddrFromSlice(dest.Addr().AsSlice()),
		Port: dest.Port(),
	}
	// Bound dial with portForwardDialTimeout so unreachable peers do not
	// pin goroutines and netstack endpoints in s.tracked.
	dctx, dcancel := context.WithTimeout(context.Background(), portForwardDialTimeout)
	defer dcancel()
	tunnelConn, err := gonet.DialContextTCP(dctx, dev.Stack, fa, tunnelNetwork(dest.Addr()))
	if err != nil {
		logger.Warningf("amneziawgnet: egress %q: dial tunnel %s: %v", tag, dest, err)
		writeSocksReply(upstream, 0x01, netip.AddrPort{})
		return
	}
	defer tunnelConn.Close()

	if _, err := upstream.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(tunnelConn, upstream); done <- struct{}{} }()
	go func() { _, _ = io.Copy(upstream, tunnelConn); done <- struct{}{} }()
	<-done
}

// udpControl is the control half of one UDP ASSOCIATE: the TCP connection
// whose lifetime bounds the association (RFC 1928).
type udpControl struct{ conn net.Conn }

// egressUDPSession is one UDP ASSOCIATE flow: host-facing socket plus a
// connected tunnel endpoint whose source port makes replies answerable.
type egressUDPSession struct {
	dst  netip.AddrPort
	conn *gonet.UDPConn
}

// relayUDP answers the associate request and relays datagrams to
// per-destination tunnel endpoints until the control connection closes.
func (s *socks5EgressServer) relayUDP(dev *Device, tag string, ctl udpControl, _ socksTarget) {
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		logger.Warningf("amneziawgnet: egress %q: udp bind: %v", tag, err)
		writeSocksReply(ctl.conn, 0x01, netip.AddrPort{})
		return
	}
	defer udpConn.Close()

	local := udpConn.LocalAddr().(*net.UDPAddr)
	ip4 := local.IP.To4()
	reply := []byte{
		0x05, 0x00, 0x00, 0x01, ip4[0], ip4[1], ip4[2], ip4[3],
		byte(local.Port >> 8), byte(local.Port),
	}
	if _, err := ctl.conn.Write(reply); err != nil {
		return
	}

	sessions := &udpEgressSessions{m: map[netip.AddrPort]*egressUDPSession{}}

	// Reader: strip per-datagram SOCKS5 headers and forward into the tunnel;
	// only the associated client's address is accepted.
	go func() {
		var client netip.AddrPort
		buf := make([]byte, 65536)
		for {
			n, from, err := udpConn.ReadFrom(buf)
			if err != nil {
				return
			}
			if src, ok := udpAddrPort(from); ok {
				if client.IsValid() && src != client {
					continue // RFC 1928: only the associated client may send
				}
				client = src
			}
			data := buf[:n]
			if len(data) < 4 {
				continue
			}
			atyp := data[3]
			var dst netip.AddrPort
			var payloadOff int
			if atyp == 0x03 {
				name, port, hdrLen, perr := parseDatagramDomainHeader(data)
				if perr != nil {
					continue
				}
				// Resolve off reader loop so slow tunnel DNS lookups do not
				// stall other destinations on this association.
				go func(client netip.AddrPort, name string, port uint16, hdrLen int, datagram []byte) {
					dnsSrv := s.currentDNSServer(tag)
					rctx, rcancel := context.WithTimeout(context.Background(), tunnelResolveTimeout)
					daddr, rerr := resolveTunnelVia(rctx, dev, tag, dnsSrv, name)
					rcancel()
					if rerr != nil {
						logger.Warningf("amneziawgnet: egress %q: resolve udp %q (dns=%s): %v", tag, name, dnsSrv, rerr)
						return
					}
					s.deliverUDPDatagram(dev, tag, udpConn, client, sessions, netip.AddrPortFrom(daddr, port), datagram[hdrLen:])
				}(client, name, port, hdrLen, append([]byte(nil), data...))
				continue
			} else {
				hdrLen := 4 + addrLen(atyp) + 2
				if hdrLen <= 6 || len(data) < hdrLen {
					continue
				}
				d, derr := parseDatagramHeader(data[:hdrLen])
				if derr != nil {
					logger.Warningf("amneziawgnet: egress %q: udp header: %v", tag, derr)
					continue
				}
				dst = d
				payloadOff = hdrLen
			}
			s.deliverUDPDatagram(dev, tag, udpConn, client, sessions, dst, data[payloadOff:])
		}
	}()

	// Control-conn close tears down relaying -- relay.go UDPRelay contract.
	buf := make([]byte, 512)
	for {
		if _, err := ctl.conn.Read(buf); err != nil {
			sessions.closeAll()
			return
		}
	}
}

// udpEgressSessions guards the association's session map: the reader
// goroutine inserts while the control-conn teardown iterates.
type udpEgressSessions struct {
	mu sync.Mutex
	m  map[netip.AddrPort]*egressUDPSession
}

func (s *udpEgressSessions) getOrDial(dev *Device, tag string, udpConn *net.UDPConn, client netip.AddrPort, dst netip.AddrPort) *egressUDPSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.m[dst]; ok {
		return sess
	}
	raddr := tcpip.FullAddress{
		NIC:  1,
		Addr: tcpip.AddrFromSlice(dst.Addr().AsSlice()),
		Port: dst.Port(),
	}
	conn, err := gonet.DialUDP(dev.Stack, nil, &raddr, tunnelNetwork(dst.Addr()))
	if err != nil {
		logger.Warningf("amneziawgnet: egress %q: dial udp %s: %v", tag, dst, err)
		return nil
	}
	sess := &egressUDPSession{dst: dst, conn: conn}
	s.m[dst] = sess
	go pumpUDPEgress(udpConn, client, sess, s)
	return sess
}

func (s *udpEgressSessions) closeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sess := range s.m {
		sess.conn.Close()
	}
}

// deliverUDPDatagram forwards one payload to dst through the tunnel endpoint.
// Safe for concurrent use across resolver and direct-path goroutines.
func (s *socks5EgressServer) deliverUDPDatagram(dev *Device, tag string, udpConn *net.UDPConn, client netip.AddrPort, sessions *udpEgressSessions, dst netip.AddrPort, payload []byte) {
	if !client.IsValid() {
		return // nothing to reply to yet
	}
	sess := sessions.getOrDial(dev, tag, udpConn, client, dst)
	if sess == nil {
		return
	}
	if _, werr := sess.conn.Write(payload); werr != nil {
		logger.Warningf("amneziawgnet: egress %q: send udp to %s: %v", tag, dst, werr)
	}
}

// pumpUDPEgress reads replies from one connected tunnel endpoint and writes
// them back to the associated client as SOCKS5 UDP datagrams.
func pumpUDPEgress(udpConn *net.UDPConn, client netip.AddrPort, sess *egressUDPSession, sessions *udpEgressSessions) {
	defer func() {
		sessions.mu.Lock()
		delete(sessions.m, sess.dst)
		sessions.mu.Unlock()
		sess.conn.Close()
	}()
	buf := make([]byte, 65536)
	for {
		// Reap idle egress sessions to avoid holding them indefinitely.
		_ = sess.conn.SetReadDeadline(time.Now().Add(portForwardUDPIdleTimeout))
		n, err := sess.conn.Read(buf)
		if err != nil {
			return
		}
		hdr := make([]byte, 0, 3+1+16+2+n)
		hdr = append(hdr, 0x00, 0x00, 0x00) // RSV RSV FRAG(=0)
		if sess.dst.Addr().Is4() {
			b := sess.dst.Addr().As4()
			hdr = append(hdr, 0x01)
			hdr = append(hdr, b[:]...)
		} else {
			b := sess.dst.Addr().As16()
			hdr = append(hdr, 0x04)
			hdr = append(hdr, b[:]...)
		}
		var portBytes [2]byte
		binary.BigEndian.PutUint16(portBytes[:], sess.dst.Port())
		hdr = append(hdr, portBytes[:]...)
		hdr = append(hdr, buf[:n]...)
		if _, err := udpConn.WriteTo(hdr, net.UDPAddrFromAddrPort(client)); err != nil {
			return
		}
	}
}

// parseDatagramDomainHeader decodes a domain SOCKS5 UDP header (RSV RSV FRAG
// 0x03 LEN NAME PORT) into name, port, and header length.
func parseDatagramDomainHeader(data []byte) (name string, port uint16, hdrLen int, err error) {
	if len(data) < 5 {
		return "", 0, 0, fmt.Errorf("short domain header")
	}
	l := int(data[4])
	hdrLen = 4 + 1 + l + 2
	if l == 0 || len(data) < hdrLen {
		return "", 0, 0, fmt.Errorf("short domain payload")
	}
	name = string(data[5 : 5+l])
	port = binary.BigEndian.Uint16(data[5+l : 7+l])
	return name, port, hdrLen, nil
}

// addrLen returns the wire length of a SOCKS5 address of the given ATYP.
func addrLen(atyp byte) int {
	switch atyp {
	case 0x01:
		return 4
	case 0x04:
		return 16
	default:
		return -1
	}
}

// parseDatagramHeader decodes the destination from the front of a SOCKS5 UDP
// datagram header block (RSV RSV FRAG ATYP ADDR PORT).
func parseDatagramHeader(hdr []byte) (netip.AddrPort, error) {
	if len(hdr) < 4 {
		return netip.AddrPort{}, fmt.Errorf("short header")
	}
	atyp := hdr[3]
	body := hdr[4:]
	switch atyp {
	case 0x01:
		if len(body) < 6 {
			return netip.AddrPort{}, fmt.Errorf("short v4")
		}
		return netip.AddrPortFrom(netip.AddrFrom4([4]byte(body[:4])), binary.BigEndian.Uint16(body[4:6])), nil
	case 0x04:
		if len(body) < 18 {
			return netip.AddrPort{}, fmt.Errorf("short v6")
		}
		return netip.AddrPortFrom(netip.AddrFrom16([16]byte(body[:16])), binary.BigEndian.Uint16(body[16:18])), nil
	default:
		return netip.AddrPort{}, fmt.Errorf("unsupported atyp %d", atyp)
	}
}
