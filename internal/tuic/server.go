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

	"github.com/poise52/quic-go"
	quiccongestion "github.com/poise52/quic-go/congestion"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// Server is an in-process native Go TUIC v5 server terminating QUIC
// and bridging decrypted TCP/UDP into a local SOCKS5 inbound.
type Server struct {
	id                int
	tag               atomic.Pointer[string]
	listenAddr        string
	authTimeout       time.Duration
	congestionControl atomic.Value
	logLevel          atomic.Uint32

	maxUdpRelayPacketSize int

	users *UserRegistry
	relay *SocksRelay

	tlsConfig    *tls.Config
	quicConfig   *quic.Config
	quicListener *quic.Listener
	packetConn   net.PacketConn

	lastOnline  sync.Map // email string -> time.Time
	logThrottle sync.Map // event name -> *atomic.Int64 timestamp

	activeConnsMu sync.Mutex
	activeConns   map[[16]byte]map[*quic.Conn]*User
	connectionsMu sync.Mutex
	connections   map[*quic.Conn]struct{}

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	closed  atomic.Bool
	running atomic.Bool
}

// NewServer creates a new TUIC v5 Server instance.
func NewServer(inst Instance, relay *SocksRelay) (*Server, error) {
	if err := ValidateClients(inst.Clients); err != nil {
		return nil, err
	}
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
	if maxUdpSize > maxSafeUdpRelayPacketSize && maxUdpSize <= maxLegacyUdpRelayPacketSize {
		maxUdpSize = maxSafeUdpRelayPacketSize
	}
	if maxUdpSize > maxLegacyUdpRelayPacketSize {
		return nil, fmt.Errorf("tuic: max UDP relay packet size %d exceeds %d", maxUdpSize, maxSafeUdpRelayPacketSize)
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

	s := &Server{
		id:                    inst.Id,
		listenAddr:            inst.BindTo(),
		authTimeout:           time.Duration(authTimeout) * time.Second,
		maxUdpRelayPacketSize: maxUdpSize,
		users:                 registry,
		activeConns:           make(map[[16]byte]map[*quic.Conn]*User),
		connections:           make(map[*quic.Conn]struct{}),
		relay:                 relay,
		tlsConfig:             tlsConfig,
		quicConfig:            quicConfig,
		ctx:                   ctx,
		cancel:                cancel,
	}
	s.updateRuntimeSettings(inst.Tag, inst.CongestionControl, inst.LogLevel)
	s.quicConfig.GetConfigForClient = func(_ *quic.ClientInfo) (*quic.Config, error) {
		connectionConfig := s.quicConfig.Clone()
		controller, _ := s.congestionControl.Load().(string)
		connectionConfig.ConfigureCongestionControl = func(conn *quic.Conn) {
			s.configureConnectionCongestionControl(conn, controller)
		}
		return connectionConfig, nil
	}
	return s, nil
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
	s.logf(tuicLogInfo, "listener started on %s", s.listenAddr)

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// IsRunning returns whether the server is currently accepting connections.
func (s *Server) IsRunning() bool {
	return s.running.Load() && !s.closed.Load()
}

func (s *Server) updateRuntimeSettings(tag, controller, logLevel string) {
	tagCopy := tag
	s.tag.Store(&tagCopy)
	s.logLevel.Store(parseLogLevel(logLevel))
	normalized, valid := normalizeCongestionControl(controller)
	s.congestionControl.Store(normalized)
	if !valid {
		s.logf(tuicLogWarn, "unsupported congestion controller %q; using %s", controller, normalized)
	}
}

func (s *Server) UpdateRuntimeSettings(tag, controller, logLevel string) {
	s.updateRuntimeSettings(tag, controller, logLevel)
}

func configureCongestionControl(target interface {
	SetCubicCongestionControl(reno bool) bool
	SetCongestionControlFactory(func(quiccongestion.ByteCount) quiccongestion.CongestionControl) bool
}, controller string,
) bool {
	switch controller {
	case "bbr":
		return target.SetCongestionControlFactory(func(size quiccongestion.ByteCount) quiccongestion.CongestionControl {
			return newXrayBBR(size)
		})
	case "cubic":
		return target.SetCubicCongestionControl(false)
	case "new_reno":
		return target.SetCubicCongestionControl(true)
	}
	return false
}

func (s *Server) configureConnectionCongestionControl(conn *quic.Conn, controller string) {
	if !configureCongestionControl(conn, controller) {
		s.logf(tuicLogError, "QUIC implementation does not support the %s controller", controller)
		return
	}
	s.logf(tuicLogDebug, "configured %s congestion controller before QUIC handshake", controller)
}

func (s *Server) registerConn(user *User, conn *quic.Conn) {
	s.activeConnsMu.Lock()
	defer s.activeConnsMu.Unlock()
	if s.activeConns[user.UUID] == nil {
		s.activeConns[user.UUID] = make(map[*quic.Conn]*User)
	}
	s.activeConns[user.UUID][conn] = user
	user.sessions.Add(1)
}

func (s *Server) unregisterConn(user *User, conn *quic.Conn) {
	s.activeConnsMu.Lock()
	if conns := s.activeConns[user.UUID]; conns != nil {
		if registered, ok := conns[conn]; ok {
			delete(conns, conn)
			if registered == user {
				s.users.sessionEnded(user)
			}
		}
		if len(conns) == 0 {
			delete(s.activeConns, user.UUID)
		}
	}
	s.activeConnsMu.Unlock()
}

func (s *Server) closeUserConns(user *User) {
	s.activeConnsMu.Lock()
	conns := s.activeConns[user.UUID]
	var toClose []*quic.Conn
	for conn, registered := range conns {
		if registered == user {
			toClose = append(toClose, conn)
		}
	}
	s.activeConnsMu.Unlock()

	for _, conn := range toClose {
		_ = conn.CloseWithError(0x100, "tuic: user revoked")
	}
}

func (s *Server) closeAllConns() {
	s.connectionsMu.Lock()
	var all []*quic.Conn
	for conn := range s.connections {
		all = append(all, conn)
	}
	s.connectionsMu.Unlock()

	for _, conn := range all {
		_ = conn.CloseWithError(0x00, "tuic: server closed")
	}
}

// UpdateUsers updates the active users dynamically without restarting the listener,
// and terminates active QUIC sessions for any revoked or disabled users.
func (s *Server) UpdateUsers(clients []TuicClientSettings) {
	if err := ValidateClients(clients); err != nil {
		s.logLimited(tuicLogWarn, "users-invalid", 30*time.Second, "User update rejected: %v", err)
		return
	}
	revoked := s.users.SetUsers(clients)
	if len(revoked) > 0 {
		s.logf(tuicLogDebug, "Revoked %d user registrations", len(revoked))
	}
	for _, u := range revoked {
		s.closeUserConns(u)
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
	deltas := s.users.CollectTrafficDeltas()
	for i := range deltas {
		deltas[i].InboundID = s.id
	}
	return deltas
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
	deltas := s.CollectClientTraffic()
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
			s.running.Store(false)
			s.logf(tuicLogError, "QUIC listener stopped accepting connections: %v", err)
			_ = s.quicListener.Close()
			return
		}
		s.connectionsMu.Lock()
		if s.closed.Load() {
			s.connectionsMu.Unlock()
			_ = conn.CloseWithError(0x00, "tuic: server closed")
			return
		}
		s.connections[conn] = struct{}{}
		s.connectionsMu.Unlock()
		s.wg.Add(1)
		go func(c *quic.Conn) {
			defer s.wg.Done()
			s.handleConn(c)
		}(conn)
	}
}

func (s *Server) handleConn(conn *quic.Conn) {
	defer func() {
		s.connectionsMu.Lock()
		delete(s.connections, conn)
		s.connectionsMu.Unlock()
	}()
	sessCtx, sessCancel := context.WithCancel(s.ctx)
	stopConnWatch := context.AfterFunc(conn.Context(), sessCancel)
	defer stopConnWatch()

	var (
		authUser        atomic.Pointer[User]
		authState       atomic.Uint32 // 0 pending, 1 authenticated, 2 timed out
		authSignal      = make(chan struct{})
		authOnce        sync.Once
		udpAssociations = newUdpAssociationRegistry(s.maxUdpRelayPacketSize)
	)

	authTimer := time.AfterFunc(s.authTimeout, func() {
		if authState.CompareAndSwap(0, 2) {
			s.logf(tuicLogWarn, "client authentication timed out")
			sessCancel()
			_ = conn.CloseWithError(0x100, "tuic: authentication timeout")
		}
	})
	defer authTimer.Stop()

	authenticate := func(rawUUID [16]byte, token [32]byte) (*User, error) {
		tlsState := conn.ConnectionState().TLS
		return s.users.AuthenticateAndRegister(&tlsState, rawUUID, token, func(user *User) bool {
			if !authState.CompareAndSwap(0, 1) {
				return authState.Load() == 1 && authUser.Load() == user
			}
			authUser.Store(user)
			s.registerConn(user, conn)
			s.markActive(user.Email)
			authTimer.Stop()
			s.logf(tuicLogInfo, "client authenticated")
			authOnce.Do(func() { close(authSignal) })
			return true
		})
	}

	waitForAuth := func() (*User, error) {
		if authState.Load() == 1 {
			if u := authUser.Load(); u != nil {
				return u, nil
			}
		}
		if authState.Load() == 2 {
			return nil, errors.New("tuic: authentication timeout")
		}
		select {
		case <-authSignal:
			if u := authUser.Load(); u != nil {
				return u, nil
			}
			return nil, errors.New("tuic: authentication unavailable")
		case <-sessCtx.Done():
			return nil, sessCtx.Err()
		}
	}

	var relayWg sync.WaitGroup
	cleanup := func() {
		sessCancel()
		udpAssociations.closeAll()
		relayWg.Wait()
		if u := authUser.Load(); u != nil {
			s.unregisterConn(u, conn)
		}
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
				s.handleUniStream(sessCtx, conn, stream, authenticate, waitForAuth, udpAssociations, &relayWg)
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
				s.handleBiStream(sessCtx, conn, stream, authenticate, waitForAuth)
			}(biStream)
		}
	}()

	// Loop 3: Datagrams
	innerWg.Add(1)
	go func() {
		defer innerWg.Done()
		for {
			dgram, err := conn.ReceiveDatagram(sessCtx)
			if err != nil {
				return
			}
			s.handleDatagram(sessCtx, conn, dgram, waitForAuth, udpAssociations, &relayWg)
		}
	}()

	innerWg.Add(1)
	go func() {
		defer innerWg.Done()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				udpAssociations.reapIdle(time.Now())
			case <-sessCtx.Done():
				return
			}
		}
	}()

	innerWg.Wait()
}

func (s *Server) handleUniStream(
	ctx context.Context,
	conn *quic.Conn,
	stream *quic.ReceiveStream,
	authenticate func([16]byte, [32]byte) (*User, error),
	waitForAuth func() (*User, error),
	udpAssociations *udpAssociationRegistry,
	relayWg *sync.WaitGroup,
) {
	defer stream.CancelRead(0)
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

		_, err := authenticate(rawUUID, token)
		if err != nil {
			s.logLimited(tuicLogWarn, "auth-rejected", 30*time.Second, "client authentication rejected")
			_ = conn.CloseWithError(0x100, "tuic: authentication failed")
			return
		}
	case CmdDissociate:
		if _, err := waitForAuth(); err != nil {
			return
		}
		var assocIDBytes [2]byte
		if _, err := io.ReadFull(stream, assocIDBytes[:]); err != nil {
			return
		}
		assocID := binary.BigEndian.Uint16(assocIDBytes[:])
		if udpAssociations.dissociate(assocID) {
			s.logf(tuicLogInfo, "UDP association %d closed", assocID)
		}

	case CmdPacket:
		user, err := waitForAuth()
		if err != nil {
			return
		}
		hdr, err := ReadPacketHeader(stream)
		if err != nil || int(hdr.Size) > s.maxUdpRelayPacketSize {
			s.logLimited(tuicLogWarn, "udp-malformed", 30*time.Second, "UDP packet rejected: malformed header or size limit")
			return
		}
		payload, err := readPacketPayload(stream, hdr)
		if err != nil {
			return
		}
		s.handlePacket(ctx, conn, user, hdr, payload, packetTransportStream, udpAssociations, relayWg)
	}
}

func (s *Server) handleBiStream(
	ctx context.Context,
	conn *quic.Conn,
	stream *quic.Stream,
	authenticate func([16]byte, [32]byte) (*User, error),
	waitForAuth func() (*User, error),
) {
	defer stream.Close()
	defer stream.CancelRead(0)

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

		_, err := authenticate(rawUUID, token)
		if err != nil {
			s.logLimited(tuicLogWarn, "auth-rejected", 30*time.Second, "client authentication rejected")
			_ = conn.CloseWithError(0x100, "tuic: authentication failed")
			return
		}
	case CmdConnect:
		user, err := waitForAuth()
		if err != nil {
			return
		}
		target, err := ReadAddress(stream)
		if err != nil {
			s.logLimited(tuicLogWarn, "tcp-relay", 30*time.Second, "TCP relay failed: malformed target address")
			return
		}

		s.markActive(user.Email)
		if !isPacketTarget(target) {
			s.logLimited(tuicLogWarn, "tcp-relay", 30*time.Second, "TCP relay failed: invalid target address")
			return
		}
		socksConn, err := s.relay.DialTCP(ctx, user.Email, target)
		if err != nil {
			s.logLimited(tuicLogWarn, "tcp-relay", 30*time.Second, "TCP relay failed: %v", err)
			return
		}
		s.logf(tuicLogInfo, "TCP relay started")

		PipeBiDirectionalContext(ctx, tcpRelayStream{stream}, socksConn, &user.Traffic.BytesUp, &user.Traffic.BytesDown)
		s.logf(tuicLogDebug, "TCP relay closed")
	}
}

type packetFragmentKey struct {
	assocID   uint16
	pktID     uint16
	transport uint8
}

const (
	packetTransportDatagram uint8 = iota
	packetTransportStream
)

type udpRelaySession struct {
	relay             *SocksUDPSession
	responseTransport uint8
}

type packetReassembly struct {
	total     uint8
	received  uint8
	size      int
	frags     [][]byte
	addr      *Address
	updatedAt time.Time
}

type packetReassembler struct {
	mu            sync.Mutex
	maxPacketSize int
	packets       map[packetFragmentKey]*packetReassembly
}

const (
	maxSafeUdpRelayPacketSize   = maxSocksUdpDatagramSize - 262
	maxLegacyUdpRelayPacketSize = maxSocksUdpDatagramSize
	maxUdpRelayPacketSize       = maxSafeUdpRelayPacketSize
	maxPendingPacketAssemblies  = 32
	packetAssemblyTimeout       = 10 * time.Second
)

func newPacketReassembler(maxPacketSize int) *packetReassembler {
	if maxPacketSize <= 0 || maxPacketSize > maxUdpRelayPacketSize {
		maxPacketSize = maxUdpRelayPacketSize
	}
	return &packetReassembler{
		maxPacketSize: maxPacketSize,
		packets:       make(map[packetFragmentKey]*packetReassembly),
	}
}

func (pr *packetReassembler) feed(transport uint8, hdr *PacketHeader, payload []byte) (*Address, []byte, bool) {
	if hdr == nil || hdr.FragTotal == 0 || hdr.FragID >= hdr.FragTotal || int(hdr.Size) != len(payload) || len(payload) > pr.maxPacketSize {
		return nil, nil, false
	}
	pr.mu.Lock()
	defer pr.mu.Unlock()
	now := time.Now()
	pr.expireLocked(now)
	key := packetFragmentKey{assocID: hdr.AssocID, pktID: hdr.PktID, transport: transport}
	if hdr.FragTotal == 1 {
		if hdr.FragID != 0 || !isPacketTarget(hdr.Addr) {
			return nil, nil, false
		}
		delete(pr.packets, key)
		return hdr.Addr, payload, true
	}
	if (hdr.FragID == 0 && !isPacketTarget(hdr.Addr)) || (hdr.FragID != 0 && hdr.Addr != nil && hdr.Addr.Type != AddrTypeNone) {
		return nil, nil, false
	}

	entry, ok := pr.packets[key]
	if !ok {
		if len(pr.packets) >= maxPendingPacketAssemblies {
			return nil, nil, false
		}
		entry = &packetReassembly{
			total:     hdr.FragTotal,
			frags:     make([][]byte, hdr.FragTotal),
			updatedAt: now,
		}
		pr.packets[key] = entry
	} else if entry.total != hdr.FragTotal {
		delete(pr.packets, key)
		return nil, nil, false
	}

	fragment := entry.frags[hdr.FragID]
	if fragment != nil {
		if !bytes.Equal(fragment, payload) {
			delete(pr.packets, key)
		}
		return nil, nil, false
	}
	if entry.size+len(payload) > pr.maxPacketSize {
		delete(pr.packets, key)
		return nil, nil, false
	}
	entry.frags[hdr.FragID] = make([]byte, len(payload))
	copy(entry.frags[hdr.FragID], payload)
	entry.size += len(payload)
	entry.received++
	entry.updatedAt = now
	if hdr.FragID == 0 {
		entry.addr = hdr.Addr
	}

	if entry.received == entry.total {
		delete(pr.packets, key)
		if !isPacketTarget(entry.addr) {
			return nil, nil, false
		}
		assembled := make([]byte, 0, entry.size)
		for _, f := range entry.frags {
			assembled = append(assembled, f...)
		}
		return entry.addr, assembled, true
	}

	return nil, nil, false
}

func (pr *packetReassembler) expireLocked(now time.Time) {
	for key, entry := range pr.packets {
		if now.Sub(entry.updatedAt) > packetAssemblyTimeout {
			delete(pr.packets, key)
		}
	}
}

func (pr *packetReassembler) clearAssociation(assocID uint16) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	for key := range pr.packets {
		if key.assocID == assocID {
			delete(pr.packets, key)
		}
	}
}

func (pr *packetReassembler) clearAll() {
	pr.mu.Lock()
	pr.packets = make(map[packetFragmentKey]*packetReassembly)
	pr.mu.Unlock()
}

func isPacketTarget(addr *Address) bool {
	return addr != nil && addr.Type != AddrTypeNone
}

func readPacketPayload(r io.Reader, hdr *PacketHeader) ([]byte, error) {
	payload := make([]byte, int(hdr.Size))
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (s *Server) handlePacket(
	ctx context.Context,
	conn *quic.Conn,
	user *User,
	hdr *PacketHeader,
	payload []byte,
	transport uint8,
	udpAssociations *udpAssociationRegistry,
	relayWg *sync.WaitGroup,
) {
	association, addr, fullPayload, complete := udpAssociations.feed(transport, hdr, payload)
	if association == nil {
		s.logLimited(tuicLogWarn, "udp-malformed", 30*time.Second, "UDP packet rejected")
	}
	if !complete {
		return
	}
	s.forwardUDPPacket(ctx, conn, user, hdr.AssocID, association, addr, fullPayload, udpAssociations, relayWg)
}

func (s *Server) handleDatagram(
	ctx context.Context,
	conn *quic.Conn,
	dgram []byte,
	waitForAuth func() (*User, error),
	udpAssociations *udpAssociationRegistry,
	relayWg *sync.WaitGroup,
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
		if err != nil || int(hdr.Size) > s.maxUdpRelayPacketSize {
			s.logLimited(tuicLogWarn, "udp-malformed", 30*time.Second, "UDP packet rejected: malformed header or size limit")
			return
		}
		payload, err := readPacketPayload(r, hdr)
		if err != nil || r.Len() != 0 {
			return
		}
		s.handlePacket(ctx, conn, user, hdr, payload, packetTransportDatagram, udpAssociations, relayWg)

	case CmdDissociate:
		if len(dgram) >= 4 {
			assocID := binary.BigEndian.Uint16(dgram[2:4])
			if udpAssociations.dissociate(assocID) {
				s.logf(tuicLogInfo, "UDP association %d closed", assocID)
			}
		}
	}
}

func (s *Server) forwardUDPPacket(
	ctx context.Context,
	conn *quic.Conn,
	user *User,
	assocID uint16,
	association *udpAssociation,
	target *Address,
	payload []byte,
	udpAssociations *udpAssociationRegistry,
	relayWg *sync.WaitGroup,
) {
	if len(payload) > s.maxUdpRelayPacketSize || !isPacketTarget(target) {
		return
	}
	if association == nil || len(payload) > s.maxUdpRelayPacketSize || !isPacketTarget(target) {
		return
	}
	association, created, err := udpAssociations.ensureRelay(ctx, assocID, association, user, s.relay)
	if err != nil {
		s.logLimited(tuicLogWarn, "udp-dial", 30*time.Second, "UDP relay could not be opened: %v", err)
		return
	}
	sess := association.relay
	if created {
		s.logf(tuicLogInfo, "UDP association %d started", assocID)
		relayWg.Add(1)
		go func() {
			defer relayWg.Done()
			s.relayUDPResponses(ctx, conn, user, assocID, association, udpAssociations, sess)
		}()
	}

	if _, err := sess.relay.Send(target, payload); err != nil {
		s.logLimited(tuicLogWarn, "udp-send", 30*time.Second, "UDP relay request failed: %v", err)
		return
	}
	user.Traffic.BytesUp.Add(int64(len(payload)))
	s.markActive(user.Email)
}

const (
	maxDatagramFragmentSize = 850
	maxStreamFragmentSize   = 8 * 1024
)

func (s *Server) relayUDPResponses(
	ctx context.Context,
	conn *quic.Conn,
	user *User,
	assocID uint16,
	association *udpAssociation,
	associations *udpAssociationRegistry,
	sess *udpRelaySession,
) {
	defer associations.release(assocID, association)
	buf := make([]byte, s.maxUdpRelayPacketSize+263)
	var nextPktID uint16
	for {
		srcAddr, respPayload, err := sess.relay.Receive(buf)
		if err != nil {
			if ctx.Err() == nil && !sess.relay.closed.Load() {
				s.logLimited(tuicLogWarn, "udp-receive", 30*time.Second, "UDP relay receive failed: %v", err)
			}
			return
		}
		if len(respPayload) > s.maxUdpRelayPacketSize {
			continue
		}

		user.Traffic.BytesDown.Add(int64(len(respPayload)))
		s.markActive(user.Email)

		nextPktID++
		if err := s.sendUDPPacketFragments(ctx, conn, assocID, nextPktID, srcAddr, respPayload, sess.responseTransport); err != nil {
			if ctx.Err() == nil {
				s.logLimited(tuicLogWarn, "udp-response", 30*time.Second, "UDP relay response failed: %v", err)
			}
			return
		}
		associations.touch(assocID, association, time.Now())
	}
}

func (s *Server) sendUDPPacketFragments(
	ctx context.Context,
	conn *quic.Conn,
	assocID, pktID uint16,
	srcAddr *Address,
	payload []byte,
	transport uint8,
) error {
	if len(payload) > s.maxUdpRelayPacketSize || !isPacketTarget(srcAddr) {
		return fmt.Errorf("tuic: UDP response exceeds configured limit or has invalid source address")
	}
	fragmentSize := maxDatagramFragmentSize
	if transport == packetTransportStream {
		fragmentSize = maxStreamFragmentSize
	}
	fragmentTotal := (len(payload) + fragmentSize - 1) / fragmentSize
	if fragmentTotal == 0 {
		fragmentTotal = 1
	}
	if fragmentTotal > 255 {
		return fmt.Errorf("tuic: UDP response requires too many fragments: %d", fragmentTotal)
	}

	for i := 0; i < fragmentTotal; i++ {
		start := i * fragmentSize
		end := min(start+fragmentSize, len(payload))
		addr := (*Address)(nil)
		if i == 0 {
			addr = srcAddr
		}
		var frame bytes.Buffer
		if err := WritePacket(&frame, assocID, pktID, uint8(fragmentTotal), uint8(i), addr, payload[start:end]); err != nil {
			return err
		}

		if transport == packetTransportStream {
			stream, err := conn.OpenUniStreamSync(ctx)
			if err != nil {
				return err
			}
			if _, err := stream.Write(frame.Bytes()); err != nil {
				stream.CancelWrite(0)
				return err
			}
			if err := stream.Close(); err != nil {
				return err
			}
			continue
		}
		if err := conn.SendDatagram(frame.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

// Close gracefully stops the server and releases all network resources.
func (s *Server) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	s.running.Store(false)
	s.logf(tuicLogInfo, "listener stopped")
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

const (
	tuicLogDebug uint32 = iota
	tuicLogInfo
	tuicLogWarn
	tuicLogError
)

func parseLogLevel(level string) uint32 {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return tuicLogDebug
	case "warn", "warning":
		return tuicLogWarn
	case "error":
		return tuicLogError
	default:
		return tuicLogInfo
	}
}

func normalizeCongestionControl(controller string) (string, bool) {
	normalized, err := NormalizeCongestionControl(controller)
	if err != nil {
		return "new_reno", false
	}
	return normalized, true
}

func (s *Server) logf(level uint32, format string, args ...any) {
	if level < s.logLevel.Load() {
		return
	}
	tag := ""
	if value := s.tag.Load(); value != nil && *value != "" {
		tag = fmt.Sprintf(" (%s)", *value)
	}
	message := fmt.Sprintf("tuic: inbound %d%s: %s", s.id, tag, fmt.Sprintf(format, args...))
	switch level {
	case tuicLogDebug:
		logger.Debugf("%s", message)
	case tuicLogInfo:
		logger.Infof("%s", message)
	case tuicLogWarn:
		logger.Warningf("%s", message)
	case tuicLogError:
		logger.Errorf("%s", message)
	}
}

func (s *Server) logLimited(level uint32, key string, interval time.Duration, format string, args ...any) {
	if level < s.logLevel.Load() {
		return
	}
	value, _ := s.logThrottle.LoadOrStore(key, &atomic.Int64{})
	stamp := value.(*atomic.Int64)
	now := time.Now().UnixNano()
	last := stamp.Load()
	if last != 0 && time.Duration(now-last) < interval {
		return
	}
	if stamp.CompareAndSwap(last, now) {
		s.logf(level, format, args...)
	}
}

func loadCertificate(certInput, keyInput string) (tls.Certificate, error) {
	if strings.Contains(certInput, "-----BEGIN CERTIFICATE-----") {
		return tls.X509KeyPair([]byte(certInput), []byte(keyInput))
	}
	return tls.LoadX509KeyPair(certInput, keyInput)
}

// QUIC Close sends FIN but does not interrupt reads. Relay cancellation must
// cancel reads too, while a normal EOF preserves the peer's half-close.
type tcpRelayStream struct{ *quic.Stream }

func (stream tcpRelayStream) Close() error {
	stream.CancelRead(0)
	return stream.Stream.Close()
}
func (stream tcpRelayStream) CloseWrite() error { return stream.Stream.Close() }
