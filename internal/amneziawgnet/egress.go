package amneziawgnet

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// EgressBasePort is the fixed loopback port the panel's own SOCKS5 egress
// server listens on. Fixed (not derived) because it appears in every
// amneziawg outbound's generated socks replacement; 64900 sits below the
// inbound relay range (SOCKSBasePort 65100+) and above the ephemeral ports
// most distributions allocate by default.
const EgressBasePort = 64900

// socks5EgressServer is a minimal loopback-only SOCKS5 server: CONNECT for
// TCP, UDP ASSOCIATE for UDP. It is the egress half of the AmneziaWG
// outbound design -- Xray's generated config replaces each "amneziawg"
// outbound with a plain socks outbound pointed here, authenticating with the
// outbound's tag as username, so this server can route the flow into that
// outbound's embedded device's netstack. The password is ignored: only
// loopback can reach the listener, and the username already carries all the
// routing information there is.
type socks5EgressServer struct {
	mu       sync.Mutex
	listener net.Listener
	stacks   map[string]*Device // outbound tag -> its device
	closing  chan struct{}
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
			closing: make(chan struct{}),
		}
	})
	return egressServer
}

// SetStack registers or replaces the device backing an outbound tag.
func (s *socks5EgressServer) SetStack(tag string, dev *Device) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stacks[tag] = dev
}

// DeleteStack drops an outbound tag's registration (outbound removed).
func (s *socks5EgressServer) DeleteStack(tag string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.stacks, tag)
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
	logger.Infof("amneziawgnet: egress socks listening on %s", ln.Addr())
	s.wg.Add(1)
	go s.acceptLoop()
	return nil
}

// Close stops the listener and waits for in-flight handlers.
func (s *socks5EgressServer) Close() {
	s.mu.Lock()
	ln := s.listener
	s.listener = nil
	s.mu.Unlock()
	if ln == nil {
		return
	}
	ln.Close()
	close(s.closing)
	s.wg.Wait()
}

func (s *socks5EgressServer) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.closing:
				return
			default:
			}
			logger.Warningf("amneziawgnet: egress accept: %v", err)
			continue
		}
		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			s.handleConn(c)
		}(conn)
	}
}

// stackFor resolves a tag to its live device at use time, so a reconfigure
// that rebuilds the device takes effect for every new connection without any
// re-registration of the listener itself.
func (s *socks5EgressServer) stackFor(tag string) (*Device, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dev, ok := s.stacks[tag]
	return dev, ok
}

func (s *socks5EgressServer) handleConn(conn net.Conn) {
	defer conn.Close()

	method, err := socks5Greeting(conn)
	if err != nil || method == 0xFF {
		return
	}
	user := ""
	if method == 0x02 {
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
		// Any credentials are accepted: loopback-only listener, and the
		// username is routing information, not a secret.
		if _, err := conn.Write([]byte{0x01, 0x00}); err != nil {
			return
		}
	}

	var req [4]byte
	if _, err := io.ReadFull(conn, req[:]); err != nil {
		return
	}
	dest, err := readSocksRequestAddr(conn, req[3])
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
		s.relayTCP(dev, user, conn, dest)
	case 0x03: // UDP ASSOCIATE
		dev, ok := s.stackFor(user)
		if !ok {
			writeSocksReply(conn, 0x05, netip.AddrPort{})
			return
		}
		s.relayUDP(dev, user, udpControl{conn: conn}, dest)
	default:
		writeSocksReply(conn, 0x07, netip.AddrPort{})
	}
}

// socks5Greeting reads the client greeting, replies with our chosen auth
// method (username/password when offered, else no-auth), returning the
// selected method code. 0xFF means "no acceptable method" was already
// answered with an error by us or the read failed.
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
	chosen := byte(0x00)
	if hasUserPass {
		chosen = 0x02
	}
	if _, err := conn.Write([]byte{0x05, chosen}); err != nil {
		return 0xFF, err
	}
	return chosen, nil
}

// readSocksRequestAddr parses the address portion of a request.
func readSocksRequestAddr(r io.Reader, atyp byte) (netip.AddrPort, error) {
	addr, err := readSocks5Addr(r, atyp)
	if err != nil {
		return netip.AddrPort{}, err
	}
	var portBytes [2]byte
	if _, err := io.ReadFull(r, portBytes[:]); err != nil {
		return netip.AddrPort{}, err
	}
	return netip.AddrPortFrom(addr, binary.BigEndian.Uint16(portBytes[:])), nil
}

// writeSocksReply emits a success/failure reply with an empty v4 bind
// address -- Xray's socks client reads the code byte and its own dialed
// address; it does not need the server to echo one back.
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
	tunnelConn, err := gonet.DialContextTCP(context.Background(), dev.Stack, fa, tunnelNetwork(dest.Addr()))
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

// relayUDP answers the associate request, then serves datagrams from the
// control socket into the tunnel until either side closes -- the mirror image
// of relay.go's inbound UDPRelay.pump.
func (s *socks5EgressServer) relayUDP(dev *Device, tag string, ctl udpControl, _ netip.AddrPort) {
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

	// Reader side: datagrams arriving from Xray carry SOCKS5 UDP headers;
	// strip and forward each payload into the tunnel stack.
	go func() {
		buf := make([]byte, 65536)
		for {
			n, err := udpConn.Read(buf)
			if err != nil {
				return
			}
			data := buf[:n]
			if len(data) < 4 {
				continue
			}
			atyp := data[3]
			hdrLen := 4 + addrLen(atyp) + 2
			if atyp == 0x03 || hdrLen <= 6 || len(data) < hdrLen {
				continue // domain-targeted datagrams never occur from Xray here
			}
			dst, err := parseDatagramHeader(data[:hdrLen])
			if err != nil {
				continue
			}
			src := netip.AddrPortFrom(tunnelLocalAddress(dev, dst.Addr()), 0)
			if werr := WriteUDPReply(dev.Stack, src, dst, data[hdrLen:]); werr != nil {
				logger.Debugf("amneziawgnet: egress %q: inject udp to %s: %v", tag, dst, werr)
			}
		}
	}()

	// Block until the control connection closes, then tear down relaying --
	// RFC 1928 semantics, same contract as relay.go's UDPRelay sessions.
	buf := make([]byte, 512)
	for {
		if _, err := ctl.conn.Read(buf); err != nil {
			return
		}
	}
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

// tunnelLocalAddress picks the stack's own configured address matching dst's
// family, so injected packets have a plausible source. Falls back to an
// unspecified address when the stack has none of that family (the packet is
// then dropped downstream anyway).
func tunnelLocalAddress(dev *Device, dst netip.Addr) netip.Addr {
	for _, a := range dev.LocalAddresses() {
		if a.Is4() == dst.Is4() {
			return a
		}
	}
	return netip.Addr{}
}
