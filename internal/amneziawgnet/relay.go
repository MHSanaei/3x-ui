// Phase 2: relaying a recovered tunnel connection into Xray's own,
// completely stock SOCKS5 inbound -- authenticating as the owning peer's
// email -- is what gives every embedded AmneziaWG connection real, native
// Xray stats/routing/sniffing with no Xray-core fork at all (Finding 3 of
// the migration plan: a stock SOCKS5 inbound sets its per-connection stats
// identity directly from the SOCKS5 auth username).
package amneziawgnet

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"
	"time"

	"golang.org/x/net/proxy"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/stack"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// SocksRelay describes the loopback SOCKS5 inbound decapsulated AmneziaWG
// traffic gets relayed into.
type SocksRelay struct {
	// Addr is the SOCKS5 inbound's own address, e.g. "127.0.0.1:11500".
	Addr string
	// Password is shared across every account. This traffic never leaves
	// loopback, so the password is not a real secrecy boundary -- it only
	// needs to satisfy Xray's SOCKS5 inbound requiring *some* username/
	// password auth before it will accept a connection and use the
	// username as the stats identity. Document this reasoning wherever a
	// caller generates or displays it, so it's never mistaken later for a
	// real credential.
	Password string
}

// SocksInboundSettings builds the JSON `settings` block for a stock Xray
// SOCKS5 inbound with one username/password account per email, all sharing
// password (see SocksRelay's doc comment). udp:true is required: RelayUDP
// depends on the inbound accepting UDP ASSOCIATE, not just CONNECT.
func SocksInboundSettings(emails []string, password string) ([]byte, error) {
	type account struct {
		User string `json:"user"`
		Pass string `json:"pass"`
	}
	settings := struct {
		Auth     string    `json:"auth"`
		UDP      bool      `json:"udp"`
		Accounts []account `json:"accounts"`
	}{Auth: "password", UDP: true}
	for _, email := range emails {
		settings.Accounts = append(settings.Accounts, account{User: email, Pass: password})
	}
	return json.Marshal(settings)
}

// RelayTCP dials r.Addr, authenticates as email, issues a SOCKS5 CONNECT to
// dest, and pipes bytes both ways until either side closes or errors.
// Blocks until the relay ends; meant to be called from (or as) an
// AttachTCPForwarder handler, which already runs each connection on its own
// goroutine.
func (r SocksRelay) RelayTCP(conn *gonet.TCPConn, email string, dest netip.AddrPort) {
	defer conn.Close()

	auth := &proxy.Auth{User: email, Password: r.Password}
	dialer, err := proxy.SOCKS5("tcp", r.Addr, auth, proxy.Direct)
	if err != nil {
		logger.Warningf("amneziawgnet: RelayTCP: build SOCKS5 dialer: %v", err)
		return
	}
	upstream, err := dialer.Dial("tcp", dest.String())
	if err != nil {
		logger.Warningf("amneziawgnet: RelayTCP: SOCKS5 CONNECT to %s as %q: %v", dest, email, err)
		return
	}
	defer upstream.Close()

	done := make(chan struct{}, 2)
	go func() { io.Copy(upstream, conn); done <- struct{}{} }()
	go func() { io.Copy(conn, upstream); done <- struct{}{} }()
	<-done
}

// socks5UDPSession is one established SOCKS5 UDP ASSOCIATE session: udpConn
// is the actual socket packets are sent to (and replies read from); ctrl is
// the TCP control connection that must stay open for the session's
// lifetime -- per RFC 1928, closing it tears the association down.
type socks5UDPSession struct {
	ctrl    net.Conn
	udpConn *net.UDPConn
}

// newSocks5UDPSession performs the SOCKS5 greeting, username/password auth,
// and UDP ASSOCIATE request/reply by hand: golang.org/x/net/proxy's SOCKS5
// client (used by RelayTCP above) only implements CONNECT, and xray-core's
// own proxy/socks/client.go is written against its internal transport
// types, not reusable as a standalone dialer -- so this is a small, direct,
// from-the-RFC implementation rather than an existing library call.
func newSocks5UDPSession(addr, user, password string) (*socks5UDPSession, error) {
	ctrl, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("amneziawgnet: dial SOCKS5 control connection: %w", err)
	}
	if err := socks5Handshake(ctrl, user, password); err != nil {
		ctrl.Close()
		return nil, err
	}

	// UDP ASSOCIATE, dst 0.0.0.0:0 ("I don't know my own source yet, and I
	// don't need to specify one for a loopback relay").
	if _, err := ctrl.Write([]byte{0x05, 0x03, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		ctrl.Close()
		return nil, fmt.Errorf("amneziawgnet: send UDP ASSOCIATE request: %w", err)
	}
	bind, err := readSocks5Reply(ctrl)
	if err != nil {
		ctrl.Close()
		return nil, err
	}

	udpConn, err := net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(bind))
	if err != nil {
		ctrl.Close()
		return nil, fmt.Errorf("amneziawgnet: dial SOCKS5 UDP relay endpoint %s: %w", bind, err)
	}
	return &socks5UDPSession{ctrl: ctrl, udpConn: udpConn}, nil
}

// socks5Handshake performs the version greeting and (if the server
// requires it) username/password auth. Xray's SOCKS5 inbound with
// auth:"password" always requires it; the no-auth branch exists so this
// helper isn't silently wrong against a differently-configured server.
func socks5Handshake(conn net.Conn, user, password string) error {
	if _, err := conn.Write([]byte{0x05, 0x02, 0x00, 0x02}); err != nil {
		return fmt.Errorf("amneziawgnet: send SOCKS5 greeting: %w", err)
	}
	var resp [2]byte
	if _, err := io.ReadFull(conn, resp[:]); err != nil {
		return fmt.Errorf("amneziawgnet: read SOCKS5 greeting reply: %w", err)
	}
	if resp[0] != 0x05 {
		return fmt.Errorf("amneziawgnet: unexpected SOCKS5 version %d", resp[0])
	}
	switch resp[1] {
	case 0x00: // no auth required
		return nil
	case 0x02: // username/password
		req := make([]byte, 0, 3+len(user)+len(password))
		req = append(req, 0x01, byte(len(user)))
		req = append(req, user...)
		req = append(req, byte(len(password)))
		req = append(req, password...)
		if _, err := conn.Write(req); err != nil {
			return fmt.Errorf("amneziawgnet: send SOCKS5 auth: %w", err)
		}
		var authResp [2]byte
		if _, err := io.ReadFull(conn, authResp[:]); err != nil {
			return fmt.Errorf("amneziawgnet: read SOCKS5 auth reply: %w", err)
		}
		if authResp[1] != 0x00 {
			return fmt.Errorf("amneziawgnet: SOCKS5 auth rejected (status %d)", authResp[1])
		}
		return nil
	default:
		return fmt.Errorf("amneziawgnet: SOCKS5 server offered unsupported auth method %d", resp[1])
	}
}

// readSocks5Reply reads a SOCKS5 reply (the common format shared by CONNECT
// and UDP ASSOCIATE replies) and returns its bound address.
func readSocks5Reply(r io.Reader) (netip.AddrPort, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return netip.AddrPort{}, fmt.Errorf("amneziawgnet: read SOCKS5 reply header: %w", err)
	}
	if hdr[0] != 0x05 {
		return netip.AddrPort{}, fmt.Errorf("amneziawgnet: unexpected SOCKS5 reply version %d", hdr[0])
	}
	if hdr[1] != 0x00 {
		return netip.AddrPort{}, fmt.Errorf("amneziawgnet: SOCKS5 request failed (reply code %d)", hdr[1])
	}
	addr, err := readSocks5Addr(r, hdr[3])
	if err != nil {
		return netip.AddrPort{}, err
	}
	var portBytes [2]byte
	if _, err := io.ReadFull(r, portBytes[:]); err != nil {
		return netip.AddrPort{}, fmt.Errorf("amneziawgnet: read SOCKS5 reply port: %w", err)
	}
	return netip.AddrPortFrom(addr, binary.BigEndian.Uint16(portBytes[:])), nil
}

// readSocks5Addr reads the address portion of a SOCKS5 reply for the given
// address type (IPv4, IPv6, or domain -- resolved locally since a loopback
// Xray inbound is not expected to reply with one, but it's cheap to handle
// correctly rather than fail oddly if it ever does).
func readSocks5Addr(r io.Reader, atyp byte) (netip.Addr, error) {
	switch atyp {
	case 0x01:
		var b [4]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return netip.Addr{}, err
		}
		return netip.AddrFrom4(b), nil
	case 0x04:
		var b [16]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return netip.Addr{}, err
		}
		return netip.AddrFrom16(b), nil
	case 0x03:
		var l [1]byte
		if _, err := io.ReadFull(r, l[:]); err != nil {
			return netip.Addr{}, err
		}
		name := make([]byte, l[0])
		if _, err := io.ReadFull(r, name); err != nil {
			return netip.Addr{}, err
		}
		resolved, err := net.ResolveIPAddr("ip", string(name))
		if err != nil {
			return netip.Addr{}, fmt.Errorf("amneziawgnet: resolve SOCKS5 domain reply %q: %w", name, err)
		}
		addr, ok := netip.AddrFromSlice(resolved.IP)
		if !ok {
			return netip.Addr{}, fmt.Errorf("amneziawgnet: unparseable resolved SOCKS5 domain reply address")
		}
		return addr, nil
	default:
		return netip.Addr{}, fmt.Errorf("amneziawgnet: unsupported SOCKS5 address type %d", atyp)
	}
}

// Close ends the UDP ASSOCIATE session: closing ctrl tells the SOCKS5
// server to tear down its relay side too (RFC 1928).
func (s *socks5UDPSession) Close() error {
	s.udpConn.Close()
	return s.ctrl.Close()
}

// sendTo wraps payload in a SOCKS5 UDP request header addressed to dest and
// sends it to the session's relay endpoint.
func (s *socks5UDPSession) sendTo(dest netip.AddrPort, payload []byte) error {
	hdr := make([]byte, 0, 3+1+16+2+len(payload))
	hdr = append(hdr, 0x00, 0x00, 0x00) // RSV RSV FRAG(=0, no fragmentation)
	if dest.Addr().Is4() {
		b := dest.Addr().As4()
		hdr = append(hdr, 0x01)
		hdr = append(hdr, b[:]...)
	} else {
		b := dest.Addr().As16()
		hdr = append(hdr, 0x04)
		hdr = append(hdr, b[:]...)
	}
	var portBytes [2]byte
	binary.BigEndian.PutUint16(portBytes[:], dest.Port())
	hdr = append(hdr, portBytes[:]...)
	hdr = append(hdr, payload...)
	_, err := s.udpConn.Write(hdr)
	return err
}

// receive reads one reply datagram into buf, returning the address the
// SOCKS5 server says it came from and the actual payload (a sub-slice of
// buf -- valid only until the next receive call).
func (s *socks5UDPSession) receive(buf []byte) (netip.AddrPort, []byte, error) {
	n, err := s.udpConn.Read(buf)
	if err != nil {
		return netip.AddrPort{}, nil, err
	}
	data := buf[:n]
	if len(data) < 4 {
		return netip.AddrPort{}, nil, fmt.Errorf("amneziawgnet: short SOCKS5 UDP reply (%d bytes)", n)
	}
	atyp := data[3]
	data = data[4:]
	addr, err := readSocks5Addr(bytesReader{data}, atyp)
	if err != nil {
		return netip.AddrPort{}, nil, err
	}
	switch atyp {
	case 0x01:
		data = data[4:]
	case 0x04:
		data = data[16:]
	}
	if len(data) < 2 {
		return netip.AddrPort{}, nil, fmt.Errorf("amneziawgnet: truncated SOCKS5 UDP reply port")
	}
	port := binary.BigEndian.Uint16(data[:2])
	return netip.AddrPortFrom(addr, port), data[2:], nil
}

// bytesReader is the minimal io.Reader readSocks5Addr needs, over an
// in-memory slice that's already fully available (a received UDP
// datagram) -- avoids pulling in bytes.Reader just for this.
type bytesReader struct{ b []byte }

func (r bytesReader) Read(p []byte) (int, error) {
	n := copy(p, r.b)
	if n < len(p) {
		return n, io.ErrUnexpectedEOF
	}
	return n, nil
}

// UDPRelay tracks one SOCKS5 UDP ASSOCIATE session per source (tunnel-
// internal client) flow, relaying each into r's SOCKS5 inbound and writing
// replies back through gstack -- the UDP counterpart of RelayTCP, meant to
// be driven by an AttachUDPHandler callback (see udp.go).
type UDPRelay struct {
	relay  SocksRelay
	gstack *stack.Stack

	mu       sync.Mutex
	sessions map[string]*socks5UDPSession
}

// NewUDPRelay creates a UDPRelay for one embedded AmneziaWG Device's stack.
func NewUDPRelay(relay SocksRelay, gstack *stack.Stack) *UDPRelay {
	return &UDPRelay{relay: relay, gstack: gstack, sessions: map[string]*socks5UDPSession{}}
}

// Handle relays one packet from src (the peer's tunnel-internal source) to
// dst (its real, recovered destination), opening a fresh SOCKS5 UDP
// ASSOCIATE session for src the first time it's seen (authenticating as
// email, so Xray attributes the whole flow's stats to the right peer) and
// reusing it for subsequent packets from the same src.
func (u *UDPRelay) Handle(src, dst netip.AddrPort, email string, payload []byte) {
	u.mu.Lock()
	sess, ok := u.sessions[src.String()]
	u.mu.Unlock()

	if !ok {
		var err error
		sess, err = newSocks5UDPSession(u.relay.Addr, email, u.relay.Password)
		if err != nil {
			logger.Warningf("amneziawgnet: UDPRelay: SOCKS5 associate for %q: %v", email, err)
			return
		}
		u.mu.Lock()
		u.sessions[src.String()] = sess
		u.mu.Unlock()
		go u.pump(src, sess)
	}
	if err := sess.sendTo(dst, payload); err != nil {
		logger.Warningf("amneziawgnet: UDPRelay: send to %s: %v", dst, err)
	}
}

// pump reads replies from sess and writes them back into the tunnel toward
// src until the session errors out or goes idle for 2 minutes, then tears
// it down -- both the map entry and the underlying SOCKS5 association.
func (u *UDPRelay) pump(src netip.AddrPort, sess *socks5UDPSession) {
	defer func() {
		u.mu.Lock()
		delete(u.sessions, src.String())
		u.mu.Unlock()
		sess.Close()
	}()
	buf := make([]byte, 65536)
	for {
		_ = sess.udpConn.SetReadDeadline(time.Now().Add(2 * time.Minute))
		from, payload, err := sess.receive(buf)
		if err != nil {
			return
		}
		if err := WriteUDPReply(u.gstack, from, src, payload); err != nil {
			logger.Warningf("amneziawgnet: UDPRelay: reply write: %v", err)
		}
	}
}

// Close tears down every open session. Call when the owning Device is
// closed.
func (u *UDPRelay) Close() {
	u.mu.Lock()
	defer u.mu.Unlock()
	for k, s := range u.sessions {
		s.Close()
		delete(u.sessions, k)
	}
}
