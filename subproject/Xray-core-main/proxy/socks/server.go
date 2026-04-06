package socks

import (
	"context"
	goerrors "errors"
	"io"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/log"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	udp_proto "github.com/xtls/xray-core/common/protocol/udp"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/policy"
	"github.com/xtls/xray-core/features/routing"
	"github.com/xtls/xray-core/proxy"
	"github.com/xtls/xray-core/proxy/http"
	"github.com/xtls/xray-core/transport"
	"github.com/xtls/xray-core/transport/internet/stat"
	"github.com/xtls/xray-core/transport/internet/udp"
)

// Server is a SOCKS 5 proxy server
type Server struct {
	config        *ServerConfig
	policyManager policy.Manager
	cone          bool
	udpFilter     *UDPFilter
	httpServer    *http.Server
}

// NewServer creates a new Server object.
func NewServer(ctx context.Context, config *ServerConfig) (*Server, error) {
	v := core.MustFromContext(ctx)
	s := &Server{
		config:        config,
		policyManager: v.GetFeature(policy.ManagerType()).(policy.Manager),
		cone:          ctx.Value("cone").(bool),
	}
	httpConfig := &http.ServerConfig{
		UserLevel: config.UserLevel,
	}
	if config.AuthType == AuthType_PASSWORD {
		httpConfig.Accounts = config.Accounts
		s.udpFilter = new(UDPFilter) // We only use this when auth is enabled
	}
	s.httpServer, _ = http.NewServer(ctx, httpConfig)
	return s, nil
}

func (s *Server) policy() policy.Session {
	config := s.config
	p := s.policyManager.ForLevel(config.UserLevel)
	return p
}

// Network implements proxy.Inbound.
func (s *Server) Network() []net.Network {
	list := []net.Network{net.Network_TCP}
	if s.config.UdpEnabled {
		list = append(list, net.Network_UDP)
	}
	return list
}

// Process implements proxy.Inbound.
func (s *Server) Process(ctx context.Context, network net.Network, conn stat.Connection, dispatcher routing.Dispatcher) error {
	inbound := session.InboundFromContext(ctx)
	inbound.Name = "socks"
	inbound.CanSpliceCopy = 2
	inbound.User = &protocol.MemoryUser{
		Level: s.config.UserLevel,
	}
	if !proxy.IsRAWTransportWithoutSecurity(conn) {
		inbound.CanSpliceCopy = 3
	}

	switch network {
	case net.Network_TCP:
		firstbyte := make([]byte, 1)
		if n, err := conn.Read(firstbyte); n == 0 {
			if goerrors.Is(err, io.EOF) {
				errors.LogInfo(ctx, "Connection closed immediately, likely health check connection")
				return nil
			}
			return errors.New("failed to read from connection").Base(err)
		}
		if firstbyte[0] != 5 && firstbyte[0] != 4 { // Check if it is Socks5/4/4a
			errors.LogDebug(ctx, "Not Socks request, try to parse as HTTP request")
			return s.httpServer.ProcessWithFirstbyte(ctx, network, conn, dispatcher, firstbyte...)
		}
		return s.processTCP(ctx, conn, dispatcher, firstbyte)
	case net.Network_UDP:
		return s.handleUDPPayload(ctx, conn, dispatcher)
	default:
		return errors.New("unknown network: ", network)
	}
}

func (s *Server) processTCP(ctx context.Context, conn stat.Connection, dispatcher routing.Dispatcher, firstbyte []byte) error {
	plcy := s.policy()
	if err := conn.SetReadDeadline(time.Now().Add(plcy.Timeouts.Handshake)); err != nil {
		errors.LogInfoInner(ctx, err, "failed to set deadline")
	}

	inbound := session.InboundFromContext(ctx)
	if inbound == nil || !inbound.Gateway.IsValid() {
		return errors.New("inbound gateway not specified")
	}

	svrSession := &ServerSession{
		config:       s.config,
		address:      inbound.Gateway.Address,
		port:         inbound.Gateway.Port,
		localAddress: net.IPAddress(conn.LocalAddr().(*net.TCPAddr).IP),
	}

	// Firstbyte is for forwarded conn from SOCKS inbound
	// Because it needs first byte to choose protocol
	// We need to add it back
	reader := &buf.BufferedReader{
		Reader: buf.NewReader(conn),
		Buffer: buf.MultiBuffer{buf.FromBytes(firstbyte)},
	}
	request, err := svrSession.Handshake(reader, conn)
	if err != nil {
		if inbound.Source.IsValid() {
			log.Record(&log.AccessMessage{
				From:   inbound.Source,
				To:     "",
				Status: log.AccessRejected,
				Reason: err,
			})
		}
		return errors.New("failed to read request").Base(err)
	}
	if request.User != nil {
		inbound.User.Email = request.User.Email
	}

	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		errors.LogInfoInner(ctx, err, "failed to clear deadline")
	}

	if request.Command == protocol.RequestCommandTCP {
		dest := request.Destination()
		errors.LogInfo(ctx, "TCP Connect request to ", dest)
		if inbound.Source.IsValid() {
			ctx = log.ContextWithAccessMessage(ctx, &log.AccessMessage{
				From:   inbound.Source,
				To:     dest,
				Status: log.AccessAccepted,
				Reason: "",
			})
		}
		if inbound.CanSpliceCopy == 2 {
			inbound.CanSpliceCopy = 1
		}
		if err := dispatcher.DispatchLink(ctx, dest, &transport.Link{
			Reader: reader,
			Writer: buf.NewWriter(conn)},
		); err != nil {
			return errors.New("failed to dispatch request").Base(err)
		}
		return nil
	}

	if request.Command == protocol.RequestCommandUDP {
		if s.udpFilter != nil {
			s.udpFilter.Add(conn.RemoteAddr())
		}
		return s.handleUDP(conn)
	}

	return nil
}

func (*Server) handleUDP(c io.Reader) error {
	// The TCP connection closes after this method returns. We need to wait until
	// the client closes it.
	return common.Error2(io.Copy(buf.DiscardBytes, c))
}

func (s *Server) handleUDPPayload(ctx context.Context, conn stat.Connection, dispatcher routing.Dispatcher) error {
	if s.udpFilter != nil && !s.udpFilter.Check(conn.RemoteAddr()) {
		errors.LogDebug(ctx, "Unauthorized UDP access from ", conn.RemoteAddr().String())
		return nil
	}
	udpServer := udp.NewDispatcher(dispatcher, func(ctx context.Context, packet *udp_proto.Packet) {
		payload := packet.Payload
		errors.LogDebug(ctx, "writing back UDP response with ", payload.Len(), " bytes")

		request := protocol.RequestHeaderFromContext(ctx)
		if request == nil {
			payload.Release()
			return
		}

		if payload.UDP != nil {
			request = &protocol.RequestHeader{
				User:    request.User,
				Address: payload.UDP.Address,
				Port:    payload.UDP.Port,
			}
		}

		udpMessage, err := EncodeUDPPacket(request, payload.Bytes())
		payload.Release()

		if err != nil {
			errors.LogWarningInner(ctx, err, "failed to write UDP response")
			return
		}

		conn.Write(udpMessage.Bytes())
		udpMessage.Release()
	})
	defer udpServer.RemoveRay()

	inbound := session.InboundFromContext(ctx)
	if inbound != nil && inbound.Source.IsValid() {
		errors.LogInfo(ctx, "client UDP connection from ", inbound.Source)
	}

	var dest *net.Destination

	reader := buf.NewPacketReader(conn)
	for {
		mpayload, err := reader.ReadMultiBuffer()
		if err != nil {
			return err
		}

		for _, payload := range mpayload {
			request, err := DecodeUDPPacket(payload)
			if err != nil {
				errors.LogInfoInner(ctx, err, "failed to parse UDP request")
				payload.Release()
				continue
			}

			if payload.IsEmpty() {
				payload.Release()
				continue
			}

			destination := request.Destination()

			currentPacketCtx := ctx
			errors.LogDebug(ctx, "send packet to ", destination, " with ", payload.Len(), " bytes")
			if inbound != nil && inbound.Source.IsValid() {
				currentPacketCtx = log.ContextWithAccessMessage(ctx, &log.AccessMessage{
					From:   inbound.Source,
					To:     destination,
					Status: log.AccessAccepted,
					Reason: "",
				})
			}

			payload.UDP = &destination

			if !s.cone || dest == nil {
				dest = &destination
			}

			currentPacketCtx = protocol.ContextWithRequestHeader(currentPacketCtx, request)
			udpServer.Dispatch(currentPacketCtx, *dest, payload)
		}
	}
}

func init() {
	common.Must(common.RegisterConfig((*ServerConfig)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return NewServer(ctx, config.(*ServerConfig))
	}))
}
