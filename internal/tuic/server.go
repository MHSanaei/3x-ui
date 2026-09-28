package tuic

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// Server is an in-process native Go TUIC v5 server terminating QUIC
// and bridging decrypted TCP/UDP into a local SOCKS5 inbound.
type Server struct {
	id          int
	tag         string
	listenAddr  string
	authTimeout time.Duration

	maxUdpRelayPacketSize int
	congestionControl     string

	users *UserRegistry
	relay *SocksRelay

	tlsConfig    *tls.Config
	quicConfig   *quic.Config
	quicListener *quic.Listener
	packetConn   net.PacketConn

	lastOnline sync.Map // email string -> time.Time

	activeConnsMu sync.Mutex
	activeConns   map[[16]byte]map[*quic.Conn]struct{}

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	closed  atomic.Bool
	running atomic.Bool
}

// NewServer creates a new TUIC v5 Server instance.
func NewServer(inst Instance, relay *SocksRelay) (*Server, error) {
	if inst.Certificate == "" || inst.PrivateKey == "" {
		return nil, errors.New("tuic: certificate or private key missing")
	}

	tlsCert, err := loadCertificate(inst.Certificate, inst.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("tuic: load tls certificate: %w", err)
	}

	alpn := inst.ALPN
	if len(alpn) == 0 {
		alpn = []string{"h3", "spdy/3.1"}
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   alpn,
	}

	maxIdle := inst.MaxIdleTime
	if maxIdle <= 0 {
		maxIdle = 15
	}
	authTimeout := inst.AuthenticationTimeout
	if authTimeout <= 0 {
		authTimeout = 3
	}
	maxUdpSize := inst.MaxUdpRelayPacketSize
	if maxUdpSize <= 0 {
		maxUdpSize = 1500
	}
	cc := inst.CongestionControl
	if cc == "" {
		cc = "cubic"
	}

	quicConfig := &quic.Config{
		EnableDatagrams: true,
		MaxIdleTimeout:  time.Duration(maxIdle) * time.Second,
		KeepAlivePeriod: time.Duration(maxIdle/2) * time.Second,
		Allow0RTT:       inst.ZeroRTTHandshake,
	}

	registry := NewUserRegistry()
	registry.SetUsers(inst.Clients)

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		id:                    inst.Id,
		tag:                   inst.Tag,
		listenAddr:            inst.BindTo(),
		authTimeout:           time.Duration(authTimeout) * time.Second,
		maxUdpRelayPacketSize: maxUdpSize,
		congestionControl:     cc,
		users:                 registry,
		activeConns:           make(map[[16]byte]map[*quic.Conn]struct{}),
		relay:                 relay,
		tlsConfig:             tlsConfig,
		quicConfig:            quicConfig,
		ctx:                   ctx,
		cancel:                cancel,
	}, nil
}

// Start opens the UDP socket and starts the QUIC listener.
func (s *Server) Start() error {
	var lc net.ListenConfig
	pConn, err := lc.ListenPacket(s.ctx, "udp", s.listenAddr)
	if err != nil {
		return fmt.Errorf("tuic: listen packet on %s: %w", s.listenAddr, err)
	}
	s.packetConn = pConn

	ln, err := quic.Listen(pConn, s.tlsConfig, s.quicConfig)
	if err != nil {
		_ = pConn.Close()
		return fmt.Errorf("tuic: quic listen on %s: %w", s.listenAddr, err)
	}
	s.quicListener = ln
	s.running.Store(true)

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// IsRunning returns whether the server is currently accepting connections.
func (s *Server) IsRunning() bool {
	return s.running.Load() && !s.closed.Load()
}

func (s *Server) registerConn(uuid [16]byte, conn *quic.Conn) {
	s.activeConnsMu.Lock()
	defer s.activeConnsMu.Unlock()
	if s.activeConns[uuid] == nil {
		s.activeConns[uuid] = make(map[*quic.Conn]struct{})
	}
	s.activeConns[uuid][conn] = struct{}{}
}

func (s *Server) unregisterConn(uuid [16]byte, conn *quic.Conn) {
	s.activeConnsMu.Lock()
	defer s.activeConnsMu.Unlock()
	if conns := s.activeConns[uuid]; conns != nil {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(s.activeConns, uuid)
		}
	}
}

func (s *Server) closeUserConns(uuid [16]byte) {
	s.activeConnsMu.Lock()
	conns := s.activeConns[uuid]
	delete(s.activeConns, uuid)
	s.activeConnsMu.Unlock()

	for conn := range conns {
		_ = conn.CloseWithError(0x100, "tuic: user revoked")
	}
}

func (s *Server) closeAllConns() {
	s.activeConnsMu.Lock()
	all := s.activeConns
	s.activeConns = make(map[[16]byte]map[*quic.Conn]struct{})
	s.activeConnsMu.Unlock()

	for _, conns := range all {
		for conn := range conns {
			_ = conn.CloseWithError(0x00, "tuic: server closed")
		}
	}
}

// UpdateUsers updates the active users dynamically without restarting the listener,
// and terminates active QUIC sessions for any revoked or disabled users.
func (s *Server) UpdateUsers(clients []TuicClientSettings) {
	revoked := s.users.SetUsers(clients)
	for _, u := range revoked {
		s.closeUserConns(u.UUID)
	}
}

// GetActiveEmails returns emails that were active within the specified time window.
func (s *Server) GetActiveEmails(window time.Duration) []string {
	now := time.Now()
	var active []string
	s.lastOnline.Range(func(key, value any) bool {
		email := key.(string)
		lastTime := value.(time.Time)
		if now.Sub(lastTime) <= window {
			active = append(active, email)
		}
		return true
	})
	return active
}

// CollectClientTraffic drains and returns traffic deltas for each client.
func (s *Server) CollectClientTraffic() []ClientTrafficDelta {
	return s.users.CollectTrafficDeltas()
}

// CollectTotalTraffic drains client deltas and aggregates total up and down bytes.
func (s *Server) CollectTotalTraffic() (int64, int64) {
	deltas := s.users.CollectTrafficDeltas()
	var totalUp, totalDown int64
	for _, d := range deltas {
		totalUp += d.Up
		totalDown += d.Down
	}
	return totalUp, totalDown
}

// CollectAllTraffic drains client deltas once and returns total up, down and individual client deltas.
func (s *Server) CollectAllTraffic() (int64, int64, []ClientTrafficDelta) {
	deltas := s.users.CollectTrafficDeltas()
	var totalUp, totalDown int64
	for _, d := range deltas {
		totalUp += d.Up
		totalDown += d.Down
	}
	return totalUp, totalDown, deltas
}

func (s *Server) markActive(email string) {
	if email != "" {
		s.lastOnline.Store(email, time.Now())
	}
}

// AddTestTraffic adds byte counts to a client for testing purposes.
func (s *Server) AddTestTraffic(email string, up, down int64) bool {
	s.markActive(email)
	return s.users.AddTestTraffic(email, up, down)
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.quicListener.Accept(s.ctx)
		if err != nil {
			if s.closed.Load() {
				return
			}
			logger.Warningf("tuic: accept quic connection on %s: %v", s.listenAddr, err)
			continue
		}

		s.wg.Add(1)
		go func(c *quic.Conn) {
			defer s.wg.Done()
			s.handleConn(c)
		}(conn)
	}
}

func (s *Server) handleConn(conn *quic.Conn) {
	sessCtx, sessCancel := context.WithCancel(s.ctx)
	defer sessCancel()

	var (
		authUser    atomic.Pointer[User]
		authSignal  = make(chan struct{})
		authOnce    sync.Once
		udpSessions sync.Map // uint16 -> *SocksUDPSession
	)

	markAuth := func(u *User) {
		if u == nil {
			return
		}
		authUser.Store(u)
		s.registerConn(u.UUID, conn)
		s.markActive(u.Email)
		authOnce.Do(func() { close(authSignal) })
	}

	waitForAuth := func() (*User, error) {
		if u := authUser.Load(); u != nil {
			return u, nil
		}
		select {
		case <-authSignal:
			return authUser.Load(), nil
		case <-time.After(s.authTimeout):
			return nil, errors.New("tuic: authentication timeout")
		case <-sessCtx.Done():
			return nil, sessCtx.Err()
		}
	}

	cleanup := func() {
		if u := authUser.Load(); u != nil {
			s.unregisterConn(u.UUID, conn)
		}
		sessCancel()
		udpSessions.Range(func(key, value any) bool {
			sess := value.(*SocksUDPSession)
			_ = sess.Close()
			return true
		})
		_ = conn.CloseWithError(0, "")
	}
	defer cleanup()

	var innerWg sync.WaitGroup

	// Loop 1: Unidirectional streams
	innerWg.Add(1)
	go func() {
		defer innerWg.Done()
		for {
			uniStream, err := conn.AcceptUniStream(sessCtx)
			if err != nil {
				return
			}
			innerWg.Add(1)
			go func(stream *quic.ReceiveStream) {
				defer innerWg.Done()
				s.handleUniStream(sessCtx, conn, stream, markAuth, waitForAuth, &udpSessions)
			}(uniStream)
		}
	}()

	// Loop 2: Bidirectional streams
	innerWg.Add(1)
	go func() {
		defer innerWg.Done()
		for {
			biStream, err := conn.AcceptStream(sessCtx)
			if err != nil {
				return
			}
			innerWg.Add(1)
			go func(stream *quic.Stream) {
				defer innerWg.Done()
				s.handleBiStream(sessCtx, conn, stream, markAuth, waitForAuth)
			}(biStream)
		}
	}()

	reassembler := newPacketReassembler()

	// Loop 3: Datagrams
	innerWg.Add(1)
	go func() {
		defer innerWg.Done()
		for {
			dgram, err := conn.ReceiveDatagram(sessCtx)
			if err != nil {
				return
			}
			s.handleDatagram(sessCtx, conn, dgram, markAuth, waitForAuth, &udpSessions, reassembler)
		}
	}()

	innerWg.Wait()
}

func (s *Server) handleUniStream(
	ctx context.Context,
	conn *quic.Conn,
	stream *quic.ReceiveStream,
	markAuth func(*User),
	waitForAuth func() (*User, error),
	udpSessions *sync.Map,
) {
	_, cmd, err := ReadCommand(stream)
	if err != nil {
		return
	}

	switch cmd {
	case CmdAuthenticate:
		var authData [16 + 32]byte
		if _, err := io.ReadFull(stream, authData[:]); err != nil {
			return
		}
		var rawUUID [16]byte
		var token [32]byte
		copy(rawUUID[:], authData[0:16])
		copy(token[:], authData[16:48])

		tlsState := conn.ConnectionState().TLS
		user, err := s.users.Authenticate(&tlsState, rawUUID, token)
		if err != nil {
			_ = conn.CloseWithError(0x100, "tuic: authentication failed")
			return
		}
		markAuth(user)

	case CmdDissociate:
		if _, err := waitForAuth(); err != nil {
			return
		}
		var assocIDBytes [2]byte
		if _, err := io.ReadFull(stream, assocIDBytes[:]); err != nil {
			return
		}
		assocID := binary.BigEndian.Uint16(assocIDBytes[:])
		if val, ok := udpSessions.LoadAndDelete(assocID); ok {
			_ = val.(*SocksUDPSession).Close()
		}

	case CmdPacket:
		user, err := waitForAuth()
		if err != nil {
			return
		}
		hdr, err := ReadPacketHeader(stream)
		if err != nil {
			return
		}
		payload := make([]byte, hdr.Size)
		if _, err := io.ReadFull(stream, payload); err != nil {
			return
		}
		s.forwardUDPPacket(ctx, conn, user, hdr.AssocID, hdr.Addr, payload, udpSessions)
	}
}

func (s *Server) handleBiStream(
	ctx context.Context,
	conn *quic.Conn,
	stream *quic.Stream,
	markAuth func(*User),
	waitForAuth func() (*User, error),
) {
	defer stream.Close()

	_, cmd, err := ReadCommand(stream)
	if err != nil {
		return
	}

	switch cmd {
	case CmdAuthenticate:
		var authData [16 + 32]byte
		if _, err := io.ReadFull(stream, authData[:]); err != nil {
			return
		}
		var rawUUID [16]byte
		var token [32]byte
		copy(rawUUID[:], authData[0:16])
		copy(token[:], authData[16:48])

		tlsState := conn.ConnectionState().TLS
		user, err := s.users.Authenticate(&tlsState, rawUUID, token)
		if err != nil {
			_ = conn.CloseWithError(0x100, "tuic: authentication failed")
			return
		}
		markAuth(user)

	case CmdConnect:
		user, err := waitForAuth()
		if err != nil {
			return
		}
		target, err := ReadAddress(stream)
		if err != nil {
			return
		}

		s.markActive(user.Email)
		socksConn, err := s.relay.DialTCP(ctx, user.Email, target)
		if err != nil {
			logger.Warningf("tuic: relay DialTCP failed for %s to %s: %v", user.Email, target, err)
			return
		}

		PipeBiDirectional(stream, socksConn, &user.BytesUp, &user.BytesDown)
	}
}

type packetFragmentKey struct {
	assocID uint16
	pktID   uint16
}

type packetReassembly struct {
	total     uint8
	received  uint8
	frags     [][]byte
	addr      *Address
	updatedAt time.Time
}

type packetReassembler struct {
	mu      sync.Mutex
	packets map[packetFragmentKey]*packetReassembly
}

func newPacketReassembler() *packetReassembler {
	return &packetReassembler{
		packets: make(map[packetFragmentKey]*packetReassembly),
	}
}

func (pr *packetReassembler) feed(hdr *PacketHeader, payload []byte) (*Address, []byte) {
	if hdr.FragTotal <= 1 {
		return hdr.Addr, payload
	}
	if hdr.FragID >= hdr.FragTotal {
		return nil, nil
	}

	pr.mu.Lock()
	defer pr.mu.Unlock()

	now := time.Now()
	if len(pr.packets) > 32 {
		for k, v := range pr.packets {
			if now.Sub(v.updatedAt) > 10*time.Second {
				delete(pr.packets, k)
			}
		}
	}

	key := packetFragmentKey{assocID: hdr.AssocID, pktID: hdr.PktID}
	entry, ok := pr.packets[key]
	if !ok {
		entry = &packetReassembly{
			total:     hdr.FragTotal,
			frags:     make([][]byte, hdr.FragTotal),
			addr:      hdr.Addr,
			updatedAt: now,
		}
		pr.packets[key] = entry
	}

	if entry.frags[hdr.FragID] == nil {
		entry.frags[hdr.FragID] = payload
		entry.received++
		entry.updatedAt = now
		if entry.addr == nil && hdr.Addr != nil {
			entry.addr = hdr.Addr
		}
	}

	if entry.received == entry.total {
		delete(pr.packets, key)
		totalLen := 0
		for _, f := range entry.frags {
			totalLen += len(f)
		}
		assembled := make([]byte, 0, totalLen)
		for _, f := range entry.frags {
			assembled = append(assembled, f...)
		}
		return entry.addr, assembled
	}

	return nil, nil
}

func (s *Server) handleDatagram(
	ctx context.Context,
	conn *quic.Conn,
	dgram []byte,
	markAuth func(*User),
	waitForAuth func() (*User, error),
	udpSessions *sync.Map,
	reassembler *packetReassembler,
) {
	if len(dgram) < 2 || dgram[0] != ProtocolVersion {
		return
	}

	cmd := dgram[1]
	switch cmd {
	case CmdHeartbeat:
		if u, _ := waitForAuth(); u != nil {
			s.markActive(u.Email)
		}

	case CmdPacket:
		user, err := waitForAuth()
		if err != nil {
			return
		}
		r := bytes.NewReader(dgram[2:])
		hdr, err := ReadPacketHeader(r)
		if err != nil {
			return
		}
		payload := make([]byte, hdr.Size)
		if _, err := io.ReadFull(r, payload); err != nil {
			return
		}
		addr, fullPayload := reassembler.feed(hdr, payload)
		if fullPayload == nil {
			return
		}
		s.forwardUDPPacket(ctx, conn, user, hdr.AssocID, addr, fullPayload, udpSessions)

	case CmdDissociate:
		if len(dgram) >= 4 {
			assocID := binary.BigEndian.Uint16(dgram[2:4])
			if val, ok := udpSessions.LoadAndDelete(assocID); ok {
				_ = val.(*SocksUDPSession).Close()
			}
		}
	}
}

func (s *Server) forwardUDPPacket(
	ctx context.Context,
	conn *quic.Conn,
	user *User,
	assocID uint16,
	target *Address,
	payload []byte,
	udpSessions *sync.Map,
) {
	var sess *SocksUDPSession
	if val, ok := udpSessions.Load(assocID); ok {
		sess = val.(*SocksUDPSession)
	} else {
		newSess, err := s.relay.DialUDP(ctx, user.Email)
		if err != nil {
			logger.Warningf("tuic: DialUDP relay failed for %s (assoc %d): %v", user.Email, assocID, err)
			return
		}
		actual, loaded := udpSessions.LoadOrStore(assocID, newSess)
		if loaded {
			_ = newSess.Close()
			sess = actual.(*SocksUDPSession)
		} else {
			sess = newSess
			go s.relayUDPResponses(conn, user, assocID, sess)
		}
	}

	if _, err := sess.Send(target, payload); err == nil {
		user.BytesUp.Add(int64(len(payload)))
		s.markActive(user.Email)
	}
}

const maxDatagramFragmentSize = 1150

func (s *Server) relayUDPResponses(
	conn *quic.Conn,
	user *User,
	assocID uint16,
	sess *SocksUDPSession,
) {
	bufSize := s.maxUdpRelayPacketSize
	if bufSize < 1500 {
		bufSize = 1500
	}
	buf := make([]byte, bufSize)
	var nextPktID uint16
	for {
		srcAddr, respPayload, err := sess.Receive(buf)
		if err != nil {
			break
		}

		user.BytesDown.Add(int64(len(respPayload)))
		s.markActive(user.Email)

		nextPktID++
		s.sendUDPPacketFragments(conn, assocID, nextPktID, srcAddr, respPayload)
	}
}

func (s *Server) sendUDPPacketFragments(
	conn *quic.Conn,
	assocID, pktID uint16,
	srcAddr *Address,
	payload []byte,
) {
	if len(payload) <= maxDatagramFragmentSize {
		var out bytes.Buffer
		if err := WritePacket(&out, assocID, pktID, 1, 0, srcAddr, payload); err == nil {
			_ = conn.SendDatagram(out.Bytes())
		}
		return
	}

	numFrags := (len(payload) + maxDatagramFragmentSize - 1) / maxDatagramFragmentSize
	if numFrags > 255 {
		return
	}
	fragTotal := uint8(numFrags)

	for i := 0; i < int(fragTotal); i++ {
		start := i * maxDatagramFragmentSize
		end := start + maxDatagramFragmentSize
		if end > len(payload) {
			end = len(payload)
		}
		chunk := payload[start:end]

		var out bytes.Buffer
		if err := WritePacket(&out, assocID, pktID, fragTotal, uint8(i), srcAddr, chunk); err == nil {
			_ = conn.SendDatagram(out.Bytes())
		}
	}
}

// Close gracefully stops the server and releases all network resources.
func (s *Server) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	s.running.Store(false)
	s.cancel()

	var err error
	if s.quicListener != nil {
		err = s.quicListener.Close()
	}
	if s.packetConn != nil {
		_ = s.packetConn.Close()
	}

	s.closeAllConns()

	s.wg.Wait()
	return err
}

func loadCertificate(certInput, keyInput string) (tls.Certificate, error) {
	if strings.Contains(certInput, "-----BEGIN CERTIFICATE-----") {
		return tls.X509KeyPair([]byte(certInput), []byte(keyInput))
	}
	return tls.LoadX509KeyPair(certInput, keyInput)
}
