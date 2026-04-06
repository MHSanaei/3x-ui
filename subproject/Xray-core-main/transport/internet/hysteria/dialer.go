package hysteria

import (
	"context"
	go_tls "crypto/tls"
	"encoding/binary"
	"math/rand"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/apernet/quic-go"
	"github.com/apernet/quic-go/http3"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/net/cnc"
	"github.com/xtls/xray-core/common/task"
	hyCtx "github.com/xtls/xray-core/proxy/hysteria/ctx"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/finalmask"
	"github.com/xtls/xray-core/transport/internet/hysteria/congestion"
	"github.com/xtls/xray-core/transport/internet/hysteria/congestion/bbr"
	"github.com/xtls/xray-core/transport/internet/hysteria/udphop"
	"github.com/xtls/xray-core/transport/internet/stat"
	"github.com/xtls/xray-core/transport/internet/tls"
)

type udpSessionManagerClient struct {
	conn   *quic.Conn
	m      map[uint32]*InterUdpConn
	next   uint32
	closed bool
	mutex  sync.RWMutex
}

func (m *udpSessionManagerClient) close(udpConn *InterUdpConn) {
	if !udpConn.closed {
		udpConn.closed = true
		close(udpConn.ch)
		delete(m.m, udpConn.id)
	}
}

func (m *udpSessionManagerClient) run() {
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

	m.closed = true

	for _, udpConn := range m.m {
		m.close(udpConn)
	}
}

func (m *udpSessionManagerClient) udp() (*InterUdpConn, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.closed {
		return nil, errors.New("closed")
	}

	udpConn := &InterUdpConn{
		conn:   m.conn,
		local:  m.conn.LocalAddr(),
		remote: m.conn.RemoteAddr(),

		id: m.next,
		ch: make(chan []byte, udpMessageChanSize),
	}
	udpConn.closeFunc = func() {
		m.mutex.Lock()
		defer m.mutex.Unlock()
		m.close(udpConn)
	}
	m.m[m.next] = udpConn
	m.next++

	return udpConn, nil
}

func (m *udpSessionManagerClient) feed(id uint32, d []byte) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	udpConn, ok := m.m[id]
	if !ok {
		return
	}

	select {
	case udpConn.ch <- d:
	default:
	}
}

type client struct {
	ctx            context.Context
	dest           net.Destination
	pktConn        net.PacketConn
	conn           *quic.Conn
	config         *Config
	tlsConfig      *go_tls.Config
	socketConfig   *internet.SocketConfig
	udpmaskManager *finalmask.UdpmaskManager
	quicParams     *internet.QuicParams

	udpSM *udpSessionManagerClient
	mutex sync.Mutex
}

func (c *client) status() Status {
	if c.conn == nil {
		return StatusUnknown
	}
	select {
	case <-c.conn.Context().Done():
		return StatusInactive
	default:
		return StatusActive
	}
}

func (c *client) close() {
	_ = c.conn.CloseWithError(closeErrCodeOK, "")
	_ = c.pktConn.Close()
	c.pktConn = nil
	c.conn = nil
	c.udpSM = nil
}

func (c *client) dial() error {
	status := c.status()
	if status == StatusActive {
		return nil
	}
	if status == StatusInactive {
		c.close()
	}

	quicParams := c.quicParams
	if quicParams == nil {
		quicParams = &internet.QuicParams{
			BbrProfile: string(bbr.ProfileStandard),
			UdpHop:     &internet.UdpHop{},
		}
	}

	var index int
	if len(quicParams.UdpHop.Ports) > 0 {
		index = rand.Intn(len(quicParams.UdpHop.Ports))
		c.dest.Port = net.Port(quicParams.UdpHop.Ports[index])
	}

	raw, err := internet.DialSystem(c.ctx, c.dest, c.socketConfig)
	if err != nil {
		return errors.New("failed to dial to dest").Base(err)
	}

	var pktConn net.PacketConn
	var remote *net.UDPAddr

	switch conn := raw.(type) {
	case *internet.PacketConnWrapper:
		pktConn = conn.PacketConn
		remote = conn.RemoteAddr().(*net.UDPAddr)
	case *net.UDPConn:
		pktConn = conn
		remote = conn.RemoteAddr().(*net.UDPAddr)
	case *cnc.Connection:
		fakeConn := &internet.FakePacketConn{Conn: conn}
		pktConn = fakeConn
		remote = fakeConn.RemoteAddr().(*net.UDPAddr)

		if len(quicParams.UdpHop.Ports) > 0 {
			raw.Close()
			return errors.New("udphop requires being at the outermost level")
		}
	default:
		raw.Close()
		return errors.New("unknown conn ", reflect.TypeOf(conn))
	}

	if len(quicParams.UdpHop.Ports) > 0 {
		addr := &udphop.UDPHopAddr{
			IP:    remote.IP,
			Ports: quicParams.UdpHop.Ports,
		}
		pktConn, err = udphop.NewUDPHopPacketConn(addr, index, quicParams.UdpHop.IntervalMin, quicParams.UdpHop.IntervalMax, c.udphopDialer, pktConn)
		if err != nil {
			raw.Close()
			return errors.New("udphop err").Base(err)
		}
	}

	if c.udpmaskManager != nil {
		pktConn, err = c.udpmaskManager.WrapPacketConnClient(pktConn)
		if err != nil {
			raw.Close()
			return errors.New("mask err").Base(err)
		}
	}

	quicConfig := &quic.Config{
		InitialStreamReceiveWindow:     quicParams.InitStreamReceiveWindow,
		MaxStreamReceiveWindow:         quicParams.MaxStreamReceiveWindow,
		InitialConnectionReceiveWindow: quicParams.InitConnReceiveWindow,
		MaxConnectionReceiveWindow:     quicParams.MaxConnReceiveWindow,
		MaxIdleTimeout:                 time.Duration(quicParams.MaxIdleTimeout) * time.Second,
		KeepAlivePeriod:                time.Duration(quicParams.KeepAlivePeriod) * time.Second,
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
	// if quicParams.KeepAlivePeriod == 0 {
	// 	quicConfig.KeepAlivePeriod = 10 * time.Second
	// }

	var quicConn *quic.Conn
	rt := &http3.Transport{
		TLSClientConfig: c.tlsConfig,
		QUICConfig:      quicConfig,
		Dial: func(ctx context.Context, _ string, tlsCfg *go_tls.Config, cfg *quic.Config) (*quic.Conn, error) {
			qc, err := quic.DialEarly(ctx, pktConn, remote, tlsCfg, cfg)
			if err != nil {
				return nil, err
			}
			quicConn = qc
			return qc, nil
		},
	}
	req := &http.Request{
		Method: http.MethodPost,
		URL: &url.URL{
			Scheme: "https",
			Host:   URLHost,
			Path:   URLPath,
		},
		Header: http.Header{
			RequestHeaderAuth:   []string{c.config.Auth},
			CommonHeaderCCRX:    []string{strconv.FormatUint(quicParams.BrutalDown, 10)},
			CommonHeaderPadding: []string{authRequestPadding.String()},
		},
	}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		if quicConn != nil {
			_ = quicConn.CloseWithError(closeErrCodeProtocolError, "")
		}
		_ = pktConn.Close()
		return errors.New("RoundTrip err").Base(err)
	}
	if resp.StatusCode != StatusAuthOK {
		_ = quicConn.CloseWithError(closeErrCodeProtocolError, "")
		_ = pktConn.Close()
		return errors.New("auth failed")
	}
	_ = resp.Body.Close()

	serverUdp, _ := strconv.ParseBool(resp.Header.Get(ResponseHeaderUDPEnabled))
	serverAuto := resp.Header.Get(CommonHeaderCCRX)
	serverDown, _ := strconv.ParseUint(serverAuto, 10, 64)

	switch quicParams.Congestion {
	case "reno":
		errors.LogDebug(c.ctx, "congestion reno")
	case "bbr":
		errors.LogDebug(c.ctx, "congestion bbr ", quicParams.BbrProfile)
		congestion.UseBBR(quicConn, bbr.Profile(quicParams.BbrProfile))
	case "brutal", "":
		if serverAuto == "auto" || quicParams.BrutalUp == 0 || serverDown == 0 {
			errors.LogDebug(c.ctx, "congestion bbr ", quicParams.BbrProfile)
			congestion.UseBBR(quicConn, bbr.Profile(quicParams.BbrProfile))
		} else {
			errors.LogDebug(c.ctx, "congestion brutal bytes per second ", min(quicParams.BrutalUp, serverDown))
			congestion.UseBrutal(quicConn, min(quicParams.BrutalUp, serverDown))
		}
	case "force-brutal":
		errors.LogDebug(c.ctx, "congestion brutal bytes per second ", quicParams.BrutalUp)
		congestion.UseBrutal(quicConn, quicParams.BrutalUp)
	default:
		errors.LogDebug(c.ctx, "congestion reno")
	}

	c.pktConn = pktConn
	c.conn = quicConn
	if serverUdp {
		c.udpSM = &udpSessionManagerClient{
			conn: quicConn,
			m:    make(map[uint32]*InterUdpConn),
			next: 1,
		}
		go c.udpSM.run()
	}

	return nil
}

func (c *client) clean() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.status() == StatusInactive {
		c.close()
	}
}

func (c *client) tcp() (stat.Connection, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	err := c.dial()
	if err != nil {
		return nil, err
	}

	stream, err := c.conn.OpenStream()
	if err != nil {
		return nil, err
	}

	return &interConn{
		stream: stream,
		local:  c.conn.LocalAddr(),
		remote: c.conn.RemoteAddr(),

		client: true,
	}, nil
}

func (c *client) udp() (stat.Connection, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	err := c.dial()
	if err != nil {
		return nil, err
	}

	if c.udpSM == nil {
		return nil, errors.New("server does not support udp")
	}

	return c.udpSM.udp()
}

func (c *client) setCtx(ctx context.Context) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.ctx = ctx
}

func (c *client) udphopDialer(addr *net.UDPAddr) (net.PacketConn, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.status() != StatusActive {
		errors.LogDebug(context.Background(), "skip hop: disconnected QUIC")
		return nil, errors.New()
	}

	raw, err := internet.DialSystem(c.ctx, net.UDPDestination(net.IPAddress(addr.IP), net.Port(addr.Port)), c.socketConfig)
	if err != nil {
		errors.LogDebug(context.Background(), "skip hop: failed to dial to dest")
		raw.Close()
		return nil, errors.New()
	}

	var pktConn net.PacketConn

	switch conn := raw.(type) {
	case *internet.PacketConnWrapper:
		pktConn = conn.PacketConn
	case *net.UDPConn:
		pktConn = conn
	case *cnc.Connection:
		errors.LogDebug(context.Background(), "skip hop: udphop requires being at the outermost level")
		raw.Close()
		return nil, errors.New()
	default:
		errors.LogDebug(context.Background(), "skip hop: unknown conn ", reflect.TypeOf(conn))
		raw.Close()
		return nil, errors.New()
	}

	return pktConn, nil
}

type clientManager struct {
	m     map[string]*client
	mutex sync.Mutex
}

func (m *clientManager) clean() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, c := range m.m {
		c.clean()
	}
}

var manger *clientManager

func Dial(ctx context.Context, dest net.Destination, streamSettings *internet.MemoryStreamConfig) (stat.Connection, error) {
	tlsConfig := tls.ConfigFromStreamSettings(streamSettings)
	if tlsConfig == nil {
		return nil, errors.New("tls config is nil")
	}

	requireDatagram := hyCtx.RequireDatagramFromContext(ctx)
	addr := dest.NetAddr()
	config := streamSettings.ProtocolSettings.(*Config)

	manger.mutex.Lock()
	c, ok := manger.m[addr]
	if !ok {
		dest.Network = net.Network_UDP
		c = &client{
			ctx:            ctx,
			dest:           dest,
			config:         config,
			tlsConfig:      tlsConfig.GetTLSConfig(),
			socketConfig:   streamSettings.SocketSettings,
			udpmaskManager: streamSettings.UdpmaskManager,
			quicParams:     streamSettings.QuicParams,
		}
		manger.m[addr] = c
	}
	c.setCtx(ctx)
	manger.mutex.Unlock()

	if requireDatagram {
		return c.udp()
	}
	return c.tcp()
}

func init() {
	manger = &clientManager{
		m: make(map[string]*client),
	}
	(&task.Periodic{
		Interval: 30 * time.Second,
		Execute: func() error {
			manger.clean()
			return nil
		},
	}).Start()
}

func init() {
	common.Must(internet.RegisterTransportDialer(protocolName, Dial))
}
