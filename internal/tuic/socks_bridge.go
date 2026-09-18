package tuic

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"
)

// SocksRelay describes the loopback SOCKS5 endpoint where decrypted TUIC traffic is forwarded.
type SocksRelay struct {
	Addr     string // e.g. "127.0.0.1:63201"
	Password string // internal shared password for the SOCKS inbound
}

// CountingConn wraps a net.Conn and tracks bytes read and written atomically.
type CountingConn struct {
	net.Conn
	bytesRead    *atomic.Int64
	bytesWritten *atomic.Int64
}

func (c *CountingConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 && c.bytesRead != nil {
		c.bytesRead.Add(int64(n))
	}
	return n, err
}

func (c *CountingConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	if n > 0 && c.bytesWritten != nil {
		c.bytesWritten.Add(int64(n))
	}
	return n, err
}

// DialTCP establishes a SOCKS5 CONNECT tunnel to the target address on behalf of user.
func (r *SocksRelay) DialTCP(ctx context.Context, user string, target *Address) (net.Conn, error) {
	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", r.Addr)
	if err != nil {
		return nil, fmt.Errorf("tuic socks: dial relay %s: %w", r.Addr, err)
	}

	if err := socks5Handshake(conn, user, r.Password); err != nil {
		conn.Close()
		return nil, err
	}

	// Send SOCKS5 CONNECT request
	req := buildSocks5ConnectRequest(target)
	if req == nil {
		conn.Close()
		return nil, ErrInvalidAddr
	}
	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, fmt.Errorf("tuic socks: send CONNECT request: %w", err)
	}

	if _, err := readSocks5Reply(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func buildSocks5ConnectRequest(target *Address) []byte {
	if target == nil {
		return nil
	}
	var req []byte
	switch target.Type {
	case AddrTypeIPv4:
		ip4 := target.IP.To4()
		if len(ip4) != 4 {
			return nil
		}
		req = make([]byte, 4+4+2)
		req[0] = 0x05 // SOCKS5
		req[1] = 0x01 // CONNECT
		req[2] = 0x00 // RSV
		req[3] = 0x01 // ATYP IPv4
		copy(req[4:8], ip4)
		binary.BigEndian.PutUint16(req[8:10], target.Port)

	case AddrTypeIPv6:
		ip16 := target.IP.To16()
		if len(ip16) != 16 {
			return nil
		}
		req = make([]byte, 4+16+2)
		req[0] = 0x05
		req[1] = 0x01
		req[2] = 0x00
		req[3] = 0x04 // ATYP IPv6
		copy(req[4:20], ip16)
		binary.BigEndian.PutUint16(req[20:22], target.Port)

	case AddrTypeDomain:
		dLen := len(target.Host)
		if dLen == 0 || dLen > 255 {
			return nil
		}
		req = make([]byte, 4+1+dLen+2)
		req[0] = 0x05
		req[1] = 0x01
		req[2] = 0x00
		req[3] = 0x03 // ATYP Domain
		req[4] = byte(dLen)
		copy(req[5:5+dLen], []byte(target.Host))
		binary.BigEndian.PutUint16(req[5+dLen:7+dLen], target.Port)

	default:
		return nil
	}
	return req
}

// SocksUDPSession manages a SOCKS5 UDP ASSOCIATE tunnel to Xray.
type SocksUDPSession struct {
	ctrl     net.Conn
	udpConn  *net.UDPConn
	targetEP net.Addr
	user     string
	closed   atomic.Bool
}

// DialUDP establishes a SOCKS5 UDP ASSOCIATE tunnel to Xray.
func (r *SocksRelay) DialUDP(ctx context.Context, user string) (*SocksUDPSession, error) {
	dialer := net.Dialer{Timeout: 10 * time.Second}
	ctrl, err := dialer.DialContext(ctx, "tcp", r.Addr)
	if err != nil {
		return nil, fmt.Errorf("tuic socks: dial UDP control connection: %w", err)
	}

	if err := socks5Handshake(ctrl, user, r.Password); err != nil {
		ctrl.Close()
		return nil, err
	}

	// SOCKS5 UDP ASSOCIATE (0x03), dst 0.0.0.0:0
	if _, err := ctrl.Write([]byte{0x05, 0x03, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		ctrl.Close()
		return nil, fmt.Errorf("tuic socks: send UDP ASSOCIATE request: %w", err)
	}

	bind, err := readSocks5Reply(ctrl)
	if err != nil {
		ctrl.Close()
		return nil, err
	}

	udpConn, err := net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(bind))
	if err != nil {
		ctrl.Close()
		return nil, fmt.Errorf("tuic socks: dial UDP relay endpoint %s: %w", bind, err)
	}

	return &SocksUDPSession{
		ctrl:     ctrl,
		udpConn:  udpConn,
		targetEP: udpConn.RemoteAddr(),
		user:     user,
	}, nil
}

// Send sends a UDP payload to target via the SOCKS5 UDP ASSOCIATE relay.
func (s *SocksUDPSession) Send(target *Address, payload []byte) (int, error) {
	if s.closed.Load() {
		return 0, net.ErrClosed
	}

	hdr := buildSocks5UDPHeader(target)
	if hdr == nil {
		return 0, ErrInvalidAddr
	}

	packet := make([]byte, len(hdr)+len(payload))
	copy(packet, hdr)
	copy(packet[len(hdr):], payload)

	return s.udpConn.Write(packet)
}

// Receive reads a relayed UDP payload and extracts its original source address.
func (s *SocksUDPSession) Receive(buf []byte) (*Address, []byte, error) {
	if s.closed.Load() {
		return nil, nil, net.ErrClosed
	}

	n, err := s.udpConn.Read(buf)
	if err != nil {
		return nil, nil, err
	}
	if n < 4 {
		return nil, nil, fmt.Errorf("tuic socks: UDP packet too short (%d bytes)", n)
	}

	// SOCKS5 UDP header: [RSV(2)][FRAG(1)][ATYP(1)]
	atyp := buf[3]
	var addr *Address
	var offset int

	switch atyp {
	case 0x01: // IPv4
		if n < 10 {
			return nil, nil, fmt.Errorf("tuic socks: truncated IPv4 UDP reply")
		}
		ip := net.IP(buf[4:8])
		port := binary.BigEndian.Uint16(buf[8:10])
		addr = &Address{Type: AddrTypeIPv4, IP: ip, Host: ip.String(), Port: port}
		offset = 10

	case 0x04: // IPv6
		if n < 22 {
			return nil, nil, fmt.Errorf("tuic socks: truncated IPv6 UDP reply")
		}
		ip := net.IP(buf[4:20])
		port := binary.BigEndian.Uint16(buf[20:22])
		addr = &Address{Type: AddrTypeIPv6, IP: ip, Host: ip.String(), Port: port}
		offset = 22

	case 0x03: // Domain
		dLen := int(buf[4])
		if n < 5+dLen+2 {
			return nil, nil, fmt.Errorf("tuic socks: truncated domain UDP reply")
		}
		host := string(buf[5 : 5+dLen])
		port := binary.BigEndian.Uint16(buf[5+dLen : 7+dLen])
		addr = &Address{Type: AddrTypeDomain, Host: host, Port: port}
		offset = 7 + dLen

	default:
		return nil, nil, fmt.Errorf("tuic socks: unsupported reply ATYP 0x%02x", atyp)
	}

	return addr, buf[offset:n], nil
}

// Close closes the SOCKS5 UDP session.
func (s *SocksUDPSession) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	_ = s.udpConn.Close()
	return s.ctrl.Close()
}

func buildSocks5UDPHeader(target *Address) []byte {
	if target == nil {
		return nil
	}
	var hdr []byte
	switch target.Type {
	case AddrTypeIPv4:
		ip4 := target.IP.To4()
		if len(ip4) != 4 {
			return nil
		}
		hdr = make([]byte, 10)
		hdr[0] = 0x00 // RSV
		hdr[1] = 0x00 // RSV
		hdr[2] = 0x00 // FRAG
		hdr[3] = 0x01 // ATYP IPv4
		copy(hdr[4:8], ip4)
		binary.BigEndian.PutUint16(hdr[8:10], target.Port)

	case AddrTypeIPv6:
		ip16 := target.IP.To16()
		if len(ip16) != 16 {
			return nil
		}
		hdr = make([]byte, 22)
		hdr[0] = 0x00
		hdr[1] = 0x00
		hdr[2] = 0x00
		hdr[3] = 0x04 // ATYP IPv6
		copy(hdr[4:20], ip16)
		binary.BigEndian.PutUint16(hdr[20:22], target.Port)

	case AddrTypeDomain:
		dLen := len(target.Host)
		if dLen == 0 || dLen > 255 {
			return nil
		}
		hdr = make([]byte, 4+1+dLen+2)
		hdr[0] = 0x00
		hdr[1] = 0x00
		hdr[2] = 0x00
		hdr[3] = 0x03 // ATYP Domain
		hdr[4] = byte(dLen)
		copy(hdr[5:5+dLen], []byte(target.Host))
		binary.BigEndian.PutUint16(hdr[5+dLen:7+dLen], target.Port)

	default:
		return nil
	}
	return hdr
}

func socks5Handshake(conn net.Conn, user, password string) error {
	if _, err := conn.Write([]byte{0x05, 0x02, 0x00, 0x02}); err != nil {
		return fmt.Errorf("tuic socks: send greeting: %w", err)
	}
	var resp [2]byte
	if _, err := io.ReadFull(conn, resp[:]); err != nil {
		return fmt.Errorf("tuic socks: read greeting reply: %w", err)
	}
	if resp[0] != 0x05 {
		return fmt.Errorf("tuic socks: unexpected SOCKS version %d", resp[0])
	}
	switch resp[1] {
	case 0x00: // no auth
		return nil
	case 0x02: // username/password
		req := make([]byte, 0, 3+len(user)+len(password))
		req = append(req, 0x01, byte(len(user)))
		req = append(req, user...)
		req = append(req, byte(len(password)))
		req = append(req, password...)
		if _, err := conn.Write(req); err != nil {
			return fmt.Errorf("tuic socks: send auth: %w", err)
		}
		var authResp [2]byte
		if _, err := io.ReadFull(conn, authResp[:]); err != nil {
			return fmt.Errorf("tuic socks: read auth reply: %w", err)
		}
		if authResp[1] != 0x00 {
			return fmt.Errorf("tuic socks: auth rejected (code %d)", authResp[1])
		}
		return nil
	default:
		return fmt.Errorf("tuic socks: unsupported auth method %d", resp[1])
	}
}

func readSocks5Reply(r io.Reader) (netip.AddrPort, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return netip.AddrPort{}, fmt.Errorf("tuic socks: read reply header: %w", err)
	}
	if hdr[0] != 0x05 {
		return netip.AddrPort{}, fmt.Errorf("tuic socks: unexpected SOCKS version %d", hdr[0])
	}
	if hdr[1] != 0x00 {
		return netip.AddrPort{}, fmt.Errorf("tuic socks: request rejected (code %d)", hdr[1])
	}
	addr, err := readSocks5Addr(r, hdr[3])
	if err != nil {
		return netip.AddrPort{}, err
	}
	var portBytes [2]byte
	if _, err := io.ReadFull(r, portBytes[:]); err != nil {
		return netip.AddrPort{}, fmt.Errorf("tuic socks: read reply port: %w", err)
	}
	return netip.AddrPortFrom(addr, binary.BigEndian.Uint16(portBytes[:])), nil
}

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
			return netip.Addr{}, fmt.Errorf("tuic socks: resolve domain reply %q: %w", name, err)
		}
		addr, ok := netip.AddrFromSlice(resolved.IP)
		if !ok {
			return netip.Addr{}, fmt.Errorf("tuic socks: unparseable domain reply address")
		}
		return addr, nil
	default:
		return netip.Addr{}, fmt.Errorf("tuic socks: unsupported SOCKS5 address type %d", atyp)
	}
}

// PipeBiDirectional pipes data between two connections and tracks byte counts in each direction.
func PipeBiDirectional(a, b io.ReadWriteCloser, upCounter, downCounter *atomic.Int64) {
	var wg sync.WaitGroup
	wg.Add(2)

	pipe := func(dst io.Writer, src io.Reader, counter *atomic.Int64) {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			n, err := src.Read(buf)
			if n > 0 {
				if counter != nil {
					counter.Add(int64(n))
				}
				if _, werr := dst.Write(buf[:n]); werr != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
		if cw, ok := dst.(interface{ CloseWrite() error }); ok {
			_ = cw.CloseWrite()
		}
	}

	// a -> b (upload: client to upstream)
	go pipe(b, a, upCounter)
	// b -> a (download: upstream to client)
	go pipe(a, b, downCounter)

	wg.Wait()
	_ = a.Close()
	_ = b.Close()
}

// SOCKSBasePort is the first loopback port used for a TUIC inbound's
// internal Xray SOCKS5 relay inbound.
const SOCKSBasePort = 63200

// SOCKSPortForInbound derives one inbound's loopback SOCKS5 relay port from
// its id, bounded within the valid TCP port range (<= 65535).
func SOCKSPortForInbound(inboundID int) int {
	port := SOCKSBasePort + inboundID
	if port > 65535 {
		port = 63200 + (inboundID % 1000)
	}
	return port
}

var (
	socksPasswordOnce sync.Once
	socksPassword     string
)

// SocksPassword returns the process-wide password used to authenticate into
// every TUIC SOCKS5 relay inbound.
func SocksPassword() string {
	socksPasswordOnce.Do(func() {
		var b [24]byte
		if _, err := rand.Read(b[:]); err != nil {
			socksPassword = fmt.Sprintf("tuic-fallback-%x", b)
			return
		}
		socksPassword = base64.RawURLEncoding.EncodeToString(b[:])
	})
	return socksPassword
}

// SocksInboundSettings builds the JSON `settings` block for a stock Xray
// SOCKS5 inbound with one username/password account per email, all sharing
// password. UDP is enabled for UDP ASSOCIATE proxying.
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
