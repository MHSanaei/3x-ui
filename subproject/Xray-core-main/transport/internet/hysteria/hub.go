package hysteria

import (
	"context"
	gotls "crypto/tls"
	"encoding/binary"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apernet/quic-go"
	"github.com/apernet/quic-go/http3"
	"github.com/apernet/quic-go/quicvarint"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/proxy/hysteria/account"
	hyCtx "github.com/xtls/xray-core/proxy/hysteria/ctx"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/hysteria/congestion"
	"github.com/xtls/xray-core/transport/internet/hysteria/congestion/bbr"
	"github.com/xtls/xray-core/transport/internet/tls"
)

type udpSessionManagerServer struct {
	conn           *quic.Conn
	m              map[uint32]*InterUdpConn
	addConn        internet.ConnHandler
	stopCh         chan struct{}
	udpIdleTimeout time.Duration
	mutex          sync.RWMutex

	user *protocol.MemoryUser
}

func (m *udpSessionManagerServer) close(udpConn *InterUdpConn) {
	if !udpConn.closed {
		udpConn.closed = true
		close(udpConn.ch)
		delete(m.m, udpConn.id)
	}
}

func (m *udpSessionManagerServer) clean() {
	ticker := time.NewTicker(idleCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.mutex.RLock()
			now := time.Now()
			timeoutConn := make([]*InterUdpConn, 0, len(m.m))
			for _, udpConn := range m.m {
				if now.Sub(udpConn.GetLast()) > m.udpIdleTimeout {
					timeoutConn = append(timeoutConn, udpConn)
				}
			}
			m.mutex.RUnlock()

			for _, udpConn := range timeoutConn {
				m.mutex.Lock()
				m.close(udpConn)
				m.mutex.Unlock()
			}
		case <-m.stopCh:
			return
		}
	}
}

func (m *udpSessionManagerServer) run() {
	for {
		d, err := m.conn.ReceiveDatagram(context.Background())
		if err != nil {
			break
		}

		if len(d) < 4 {
			continue
		}
		id := binary.BigEndian.Uint32(d[:4])

		m.feed(id, d)
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	close(m.stopCh)

	for _, udpConn := range m.m {
		m.close(udpConn)
	}
}

func (m *udpSessionManagerServer) feed(id uint32, d []byte) {
	m.mutex.RLock()
	udpConn, ok := m.m[id]
	if ok {
		select {
		case udpConn.ch <- d:
		default:
		}
		m.mutex.RUnlock()
		return
	}
	m.mutex.RUnlock()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	udpConn, ok = m.m[id]
	if !ok {
		udpConn = &InterUdpConn{
			conn:   m.conn,
			local:  m.conn.LocalAddr(),
			remote: m.conn.RemoteAddr(),

			id:   id,
			ch:   make(chan []byte, udpMessageChanSize),
			last: time.Now(),

			user: m.user,
		}
		udpConn.closeFunc = func() {
			m.mutex.Lock()
			m.close(udpConn)
			m.mutex.Unlock()
		}
		m.m[id] = udpConn
		m.addConn(udpConn)
	}

	select {
	case udpConn.ch <- d:
	default:
	}
}

type httpHandler struct {
	ctx     context.Context
	conn    *quic.Conn
	addConn internet.ConnHandler

	config      *Config
	quicParams  *internet.QuicParams
	validator   *account.Validator
	masqHandler http.Handler

	auth  bool
	mutex sync.Mutex
	user  *protocol.MemoryUser
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.Host == URLHost && r.URL.Path == URLPath {
		h.mutex.Lock()
		defer h.mutex.Unlock()

		if h.auth {
			w.Header().Set(ResponseHeaderUDPEnabled, strconv.FormatBool(hyCtx.RequireDatagramFromContext(h.ctx)))
			w.Header().Set(CommonHeaderCCRX, strconv.FormatUint(h.quicParams.BrutalDown, 10))
			w.Header().Set(CommonHeaderPadding, authResponsePadding.String())
			w.WriteHeader(StatusAuthOK)
			return
		}

		auth := r.Header.Get(RequestHeaderAuth)
		clientDown, _ := strconv.ParseUint(r.Header.Get(CommonHeaderCCRX), 10, 64)

		var user *protocol.MemoryUser
		var ok bool
		if h.validator != nil {
			user = h.validator.Get(auth)
		} else if auth == h.config.Auth {
			ok = true
		}

		if user != nil || ok {
			h.auth = true
			h.user = user

			switch h.quicParams.Congestion {
			case "reno":
				errors.LogDebug(context.Background(), h.conn.RemoteAddr(), " ", "congestion reno")
			case "bbr":
				errors.LogDebug(context.Background(), h.conn.RemoteAddr(), " ", "congestion bbr ", h.quicParams.BbrProfile)
				congestion.UseBBR(h.conn, bbr.Profile(h.quicParams.BbrProfile))
			case "brutal", "":
				if h.quicParams.BrutalUp == 0 || clientDown == 0 {
					errors.LogDebug(context.Background(), h.conn.RemoteAddr(), " ", "congestion bbr ", h.quicParams.BbrProfile)
					congestion.UseBBR(h.conn, bbr.Profile(h.quicParams.BbrProfile))
				} else {
					errors.LogDebug(context.Background(), h.conn.RemoteAddr(), " ", "congestion brutal bytes per second ", min(h.quicParams.BrutalUp, clientDown))
					congestion.UseBrutal(h.conn, min(h.quicParams.BrutalUp, clientDown))
				}
			case "force-brutal":
				errors.LogDebug(context.Background(), h.conn.RemoteAddr(), " ", "congestion brutal bytes per second ", h.quicParams.BrutalUp)
				congestion.UseBrutal(h.conn, h.quicParams.BrutalUp)
			default:
				errors.LogDebug(context.Background(), h.conn.RemoteAddr(), " ", "congestion reno")
			}

			if hyCtx.RequireDatagramFromContext(h.ctx) {
				udpSM := &udpSessionManagerServer{
					conn:           h.conn,
					m:              make(map[uint32]*InterUdpConn),
					addConn:        h.addConn,
					stopCh:         make(chan struct{}),
					udpIdleTimeout: time.Duration(h.config.UdpIdleTimeout) * time.Second,

					user: h.user,
				}
				go udpSM.clean()
				go udpSM.run()
			}

			w.Header().Set(ResponseHeaderUDPEnabled, strconv.FormatBool(hyCtx.RequireDatagramFromContext(h.ctx)))
			w.Header().Set(CommonHeaderCCRX, strconv.FormatUint(h.quicParams.BrutalDown, 10))
			w.Header().Set(CommonHeaderPadding, authResponsePadding.String())
			w.WriteHeader(StatusAuthOK)
			return
		}
	}

	h.masqHandler.ServeHTTP(w, r)
}

func (h *httpHandler) StreamDispatcher(ft http3.FrameType, stream *quic.Stream, err error) (bool, error) {
	if err != nil || !h.auth {
		return false, nil
	}

	switch ft {
	case FrameTypeTCPRequest:
		if _, err := quicvarint.Read(quicvarint.NewReader(stream)); err != nil {
			return false, err
		}

		h.addConn(&interConn{
			stream: stream,
			local:  h.conn.LocalAddr(),
			remote: h.conn.RemoteAddr(),

			user: h.user,
		})
		return true, nil
	default:
		return false, nil
	}
}

type Listener struct {
	ctx      context.Context
	pktConn  net.PacketConn
	listener *quic.Listener
	addConn  internet.ConnHandler

	config      *Config
	quicParams  *internet.QuicParams
	validator   *account.Validator
	masqHandler http.Handler
}

func (l *Listener) handleClient(conn *quic.Conn) {
	handler := &httpHandler{
		ctx:     l.ctx,
		conn:    conn,
		addConn: l.addConn,

		config:      l.config,
		quicParams:  l.quicParams,
		validator:   l.validator,
		masqHandler: l.masqHandler,
	}
	h3 := http3.Server{
		Handler:          handler,
		StreamDispatcher: handler.StreamDispatcher,
	}
	err := h3.ServeQUICConn(conn)
	_ = conn.CloseWithError(closeErrCodeOK, "")
	errors.LogDebug(context.Background(), conn.RemoteAddr(), " disconnected with err ", err)
}

func (l *Listener) keepAccepting() {
	for {
		conn, err := l.listener.Accept(context.Background())
		if err != nil {
			errors.LogInfoInner(context.Background(), err, "failed to accept QUIC connection")
			break
		}
		go l.handleClient(conn)
	}
}

func (l *Listener) Addr() net.Addr {
	return l.listener.Addr()
}

func (l *Listener) Close() error {
	err := l.listener.Close()
	_ = l.pktConn.Close()
	return err
}

func Listen(ctx context.Context, address net.Address, port net.Port, streamSettings *internet.MemoryStreamConfig, handler internet.ConnHandler) (internet.Listener, error) {
	if address.Family().IsDomain() {
		return nil, errors.New("address is domain")
	}

	tlsConfig := tls.ConfigFromStreamSettings(streamSettings)
	if tlsConfig == nil {
		return nil, errors.New("tls config is nil")
	}

	config := streamSettings.ProtocolSettings.(*Config)

	validator := hyCtx.ValidatorFromContext(ctx)

	if config.Auth == "" && validator == nil {
		return nil, errors.New("validator is nil")
	}

	var masqHandler http.Handler
	switch strings.ToLower(config.MasqType) {
	case "", "404":
		masqHandler = http.NotFoundHandler()
	case "file":
		masqHandler = http.FileServer(http.Dir(config.MasqFile))
	case "proxy":
		u, err := url.Parse(config.MasqUrl)
		if err != nil {
			return nil, err
		}
		transport := http.DefaultTransport.(*http.Transport)
		if config.MasqUrlInsecure {
			transport = transport.Clone()
			transport.TLSClientConfig = &gotls.Config{
				InsecureSkipVerify: true,
			}
		}
		masqHandler = &httputil.ReverseProxy{
			Rewrite: func(pr *httputil.ProxyRequest) {
				pr.SetURL(u)
				if !config.MasqUrlRewriteHost {
					pr.Out.Host = pr.In.Host
				}
			},
			Transport: transport,
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				w.WriteHeader(http.StatusBadGateway)
			},
		}
	case "string":
		masqHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for k, v := range config.MasqStringHeaders {
				w.Header().Set(k, v)
			}
			if config.MasqStringStatusCode != 0 {
				w.WriteHeader(int(config.MasqStringStatusCode))
			} else {
				w.WriteHeader(http.StatusOK)
			}
			_, _ = w.Write([]byte(config.MasqString))
		})
	default:
		return nil, errors.New("unknown masq type")
	}

	raw, err := internet.ListenSystemPacket(context.Background(), &net.UDPAddr{IP: address.IP(), Port: int(port)}, streamSettings.SocketSettings)
	if err != nil {
		return nil, err
	}

	var pktConn net.PacketConn
	pktConn = raw

	if streamSettings.UdpmaskManager != nil {
		pktConn, err = streamSettings.UdpmaskManager.WrapPacketConnServer(raw)
		if err != nil {
			raw.Close()
			return nil, errors.New("mask err").Base(err)
		}
	}

	quicParams := streamSettings.QuicParams
	if quicParams == nil {
		quicParams = &internet.QuicParams{
			BbrProfile: string(bbr.ProfileStandard),
			UdpHop:     &internet.UdpHop{},
		}
	}

	quicConfig := &quic.Config{
		InitialStreamReceiveWindow:     quicParams.InitStreamReceiveWindow,
		MaxStreamReceiveWindow:         quicParams.MaxStreamReceiveWindow,
		InitialConnectionReceiveWindow: quicParams.InitConnReceiveWindow,
		MaxConnectionReceiveWindow:     quicParams.MaxConnReceiveWindow,
		MaxIdleTimeout:                 time.Duration(quicParams.MaxIdleTimeout) * time.Second,
		MaxIncomingStreams:             quicParams.MaxIncomingStreams,
		DisablePathMTUDiscovery:        quicParams.DisablePathMtuDiscovery,
		EnableDatagrams:                true,
		MaxDatagramFrameSize:           MaxDatagramFrameSize,
		DisablePathManager:             true,
	}
	if quicParams.InitStreamReceiveWindow == 0 {
		quicConfig.InitialStreamReceiveWindow = 8388608
	}
	if quicParams.MaxStreamReceiveWindow == 0 {
		quicConfig.MaxStreamReceiveWindow = 8388608
	}
	if quicParams.InitConnReceiveWindow == 0 {
		quicConfig.InitialConnectionReceiveWindow = 8388608 * 5 / 2
	}
	if quicParams.MaxConnReceiveWindow == 0 {
		quicConfig.MaxConnectionReceiveWindow = 8388608 * 5 / 2
	}
	if quicParams.MaxIdleTimeout == 0 {
		quicConfig.MaxIdleTimeout = 30 * time.Second
	}
	if quicParams.MaxIncomingStreams == 0 {
		quicConfig.MaxIncomingStreams = 1024
	}

	qListener, err := quic.Listen(pktConn, tlsConfig.GetTLSConfig(), quicConfig)
	if err != nil {
		_ = pktConn.Close()
		return nil, err
	}

	listener := &Listener{
		ctx:      ctx,
		pktConn:  pktConn,
		listener: qListener,
		addConn:  handler,

		config:      config,
		quicParams:  quicParams,
		validator:   validator,
		masqHandler: masqHandler,
	}

	go listener.keepAccepting()

	return listener, nil
}

func init() {
	common.Must(internet.RegisterTransportListener(protocolName, Listen))
}
