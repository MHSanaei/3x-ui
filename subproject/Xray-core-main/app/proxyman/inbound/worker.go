package inbound

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xtls/xray-core/app/proxyman"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	c "github.com/xtls/xray-core/common/ctx"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/common/signal/done"
	"github.com/xtls/xray-core/common/task"
	"github.com/xtls/xray-core/features/routing"
	"github.com/xtls/xray-core/features/stats"
	"github.com/xtls/xray-core/proxy"
	"github.com/xtls/xray-core/proxy/hysteria/account"
	hyCtx "github.com/xtls/xray-core/proxy/hysteria/ctx"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/stat"
	"github.com/xtls/xray-core/transport/internet/tcp"
	"github.com/xtls/xray-core/transport/internet/udp"
	"github.com/xtls/xray-core/transport/pipe"
)

type worker interface {
	Start() error
	Close() error
	Port() net.Port
	Proxy() proxy.Inbound
}

type tcpWorker struct {
	address         net.Address
	port            net.Port
	proxy           proxy.Inbound
	stream          *internet.MemoryStreamConfig
	recvOrigDest    bool
	tag             string
	dispatcher      routing.Dispatcher
	sniffingConfig  *proxyman.SniffingConfig
	uplinkCounter   stats.Counter
	downlinkCounter stats.Counter

	hub internet.Listener

	ctx context.Context
}

func getTProxyType(s *internet.MemoryStreamConfig) internet.SocketConfig_TProxyMode {
	if s == nil || s.SocketSettings == nil {
		return internet.SocketConfig_Off
	}
	return s.SocketSettings.Tproxy
}

func (w *tcpWorker) callback(conn stat.Connection) {
	ctx, cancel := context.WithCancel(w.ctx)
	sid := session.NewID()
	ctx = c.ContextWithID(ctx, sid)

	outbounds := []*session.Outbound{{}}
	if w.recvOrigDest {
		var dest net.Destination
		switch getTProxyType(w.stream) {
		case internet.SocketConfig_Redirect:
			d, err := tcp.GetOriginalDestination(conn)
			if err != nil {
				errors.LogInfoInner(ctx, err, "failed to get original destination")
			} else {
				dest = d
			}
		case internet.SocketConfig_TProxy:
			dest = net.DestinationFromAddr(conn.LocalAddr())
		}

		if dest.IsValid() {
			// Check if try to connect to this inbound itself (can cause loopback)
			var isLoopBack bool
			if w.address == net.AnyIP || w.address == net.AnyIPv6 {
				if dest.Port.Value() == w.port.Value() && IsLocal(dest.Address.IP()) {
					isLoopBack = true
				}
			} else {
				if w.hub.Addr().String() == dest.NetAddr() {
					isLoopBack = true
				}
			}
			if isLoopBack {
				cancel()
				conn.Close()
				errors.LogError(ctx, errors.New("loopback connection detected"))
				return
			}
			outbounds[0].Target = dest
		}
	}
	ctx = session.ContextWithOutbounds(ctx, outbounds)

	if w.uplinkCounter != nil || w.downlinkCounter != nil {
		conn = &stat.CounterConnection{
			Connection:   conn,
			ReadCounter:  w.uplinkCounter,
			WriteCounter: w.downlinkCounter,
		}
	}
	ctx = session.ContextWithInbound(ctx, &session.Inbound{
		Source:  net.DestinationFromAddr(conn.RemoteAddr()),
		Local:   net.DestinationFromAddr(conn.LocalAddr()),
		Gateway: net.TCPDestination(w.address, w.port),
		Tag:     w.tag,
		Conn:    conn,
	})

	content := new(session.Content)
	if w.sniffingConfig != nil {
		content.SniffingRequest.Enabled = w.sniffingConfig.Enabled
		content.SniffingRequest.OverrideDestinationForProtocol = w.sniffingConfig.DestinationOverride
		content.SniffingRequest.ExcludeForDomain = w.sniffingConfig.DomainsExcluded
		content.SniffingRequest.MetadataOnly = w.sniffingConfig.MetadataOnly
		content.SniffingRequest.RouteOnly = w.sniffingConfig.RouteOnly
	}
	ctx = session.ContextWithContent(ctx, content)

	if err := w.proxy.Process(ctx, net.Network_TCP, conn, w.dispatcher); err != nil {
		errors.LogInfoInner(ctx, err, "connection ends")
	}
	cancel()
	conn.Close()
}

func (w *tcpWorker) Proxy() proxy.Inbound {
	return w.proxy
}

func (w *tcpWorker) Start() error {
	ctx := context.Background()

	type HysteriaInboundValidator interface{ HysteriaInboundValidator() *account.Validator }
	if v, ok := w.proxy.(HysteriaInboundValidator); ok {
		ctx = hyCtx.ContextWithRequireDatagram(ctx, true)
		ctx = hyCtx.ContextWithValidator(ctx, v.HysteriaInboundValidator())
	}

	hub, err := internet.ListenTCP(ctx, w.address, w.port, w.stream, func(conn stat.Connection) {
		go w.callback(conn)
	})
	if err != nil {
		return errors.New("failed to listen TCP on ", w.port).AtWarning().Base(err)
	}
	w.hub = hub
	return nil
}

func (w *tcpWorker) Close() error {
	var errs []interface{}
	if w.hub != nil {
		if err := common.Close(w.hub); err != nil {
			errs = append(errs, err)
		}
		if err := common.Close(w.proxy); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.New("failed to close all resources").Base(errors.New(serial.Concat(errs...)))
	}

	return nil
}

func (w *tcpWorker) Port() net.Port {
	return w.port
}

type udpConn struct {
	lastActivityTime int64 // in seconds
	reader           buf.Reader
	writer           buf.Writer
	output           func([]byte) (int, error)
	remote           net.Addr
	local            net.Addr
	done             *done.Instance
	uplink           stats.Counter
	downlink         stats.Counter
	inactive         bool
	cancel           context.CancelFunc
}

func (c *udpConn) setInactive() {
	c.inactive = true
}

func (c *udpConn) updateActivity() {
	atomic.StoreInt64(&c.lastActivityTime, time.Now().Unix())
}

// ReadMultiBuffer implements buf.Reader
func (c *udpConn) ReadMultiBuffer() (buf.MultiBuffer, error) {
	mb, err := c.reader.ReadMultiBuffer()
	if err != nil {
		return nil, err
	}
	c.updateActivity()

	if c.uplink != nil {
		c.uplink.Add(int64(mb.Len()))
	}

	return mb, nil
}

func (c *udpConn) Read(buf []byte) (int, error) {
	panic("not implemented")
}

// Write implements io.Writer.
func (c *udpConn) Write(buf []byte) (int, error) {
	n, err := c.output(buf)
	if c.downlink != nil {
		c.downlink.Add(int64(n))
	}
	if err == nil {
		c.updateActivity()
	}
	return n, err
}

func (c *udpConn) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	common.Must(c.done.Close())
	common.Must(common.Close(c.writer))
	return nil
}

func (c *udpConn) RemoteAddr() net.Addr {
	return c.remote
}

func (c *udpConn) LocalAddr() net.Addr {
	return c.local
}

func (*udpConn) SetDeadline(time.Time) error {
	return nil
}

func (*udpConn) SetReadDeadline(time.Time) error {
	return nil
}

func (*udpConn) SetWriteDeadline(time.Time) error {
	return nil
}

type connID struct {
	src  net.Destination
	dest net.Destination
}

type udpWorker struct {
	sync.RWMutex

	proxy           proxy.Inbound
	hub             *udp.Hub
	address         net.Address
	port            net.Port
	tag             string
	stream          *internet.MemoryStreamConfig
	dispatcher      routing.Dispatcher
	sniffingConfig  *proxyman.SniffingConfig
	uplinkCounter   stats.Counter
	downlinkCounter stats.Counter

	checker    *task.Periodic
	activeConn map[connID]*udpConn

	ctx  context.Context
	cone bool
}

func (w *udpWorker) getConnection(id connID) (*udpConn, bool) {
	w.Lock()
	defer w.Unlock()

	if conn, found := w.activeConn[id]; found && !conn.done.Done() {
		conn.updateActivity()
		return conn, true
	}

	pReader, pWriter := pipe.New(pipe.DiscardOverflow(), pipe.WithSizeLimit(16*1024))
	conn := &udpConn{
		reader: pReader,
		writer: pWriter,
		output: func(b []byte) (int, error) {
			return w.hub.WriteTo(b, id.src)
		},
		remote: &net.UDPAddr{
			IP:   id.src.Address.IP(),
			Port: int(id.src.Port),
		},
		local: &net.UDPAddr{
			IP:   w.address.IP(),
			Port: int(w.port),
		},
		done:     done.New(),
		uplink:   w.uplinkCounter,
		downlink: w.downlinkCounter,
	}
	w.activeConn[id] = conn

	conn.updateActivity()
	return conn, false
}

func (w *udpWorker) callback(b *buf.Buffer, source net.Destination, originalDest net.Destination) {
	id := connID{
		src: source,
	}
	if originalDest.IsValid() {
		if !w.cone {
			id.dest = originalDest
		}
		b.UDP = &originalDest
	}
	conn, existing := w.getConnection(id)

	// payload will be discarded in pipe is full.
	conn.writer.WriteMultiBuffer(buf.MultiBuffer{b})

	if !existing {
		common.Must(w.checker.Start())

		go func() {
			ctx, cancel := context.WithCancel(w.ctx)
			conn.cancel = cancel
			sid := session.NewID()
			ctx = c.ContextWithID(ctx, sid)

			outbounds := []*session.Outbound{{}}
			if originalDest.IsValid() {
				outbounds[0].Target = originalDest
			}
			ctx = session.ContextWithOutbounds(ctx, outbounds)
			local := net.DestinationFromAddr(w.hub.Addr())
			if local.Address == net.AnyIP || local.Address == net.AnyIPv6 {
				if source.Address.Family().IsIPv4() {
					local.Address = net.AnyIP
				} else if source.Address.Family().IsIPv6() {
					local.Address = net.AnyIPv6
				}
			}

			ctx = session.ContextWithInbound(ctx, &session.Inbound{
				Source:  source,
				Local:   local, // Due to some limitations, in UDP connections, localIP is always equal to listen interface IP
				Gateway: net.UDPDestination(w.address, w.port),
				Tag:     w.tag,
			})
			content := new(session.Content)
			if w.sniffingConfig != nil {
				content.SniffingRequest.Enabled = w.sniffingConfig.Enabled
				content.SniffingRequest.OverrideDestinationForProtocol = w.sniffingConfig.DestinationOverride
				content.SniffingRequest.ExcludeForDomain = w.sniffingConfig.DomainsExcluded
				content.SniffingRequest.MetadataOnly = w.sniffingConfig.MetadataOnly
				content.SniffingRequest.RouteOnly = w.sniffingConfig.RouteOnly
			}
			ctx = session.ContextWithContent(ctx, content)
			if err := w.proxy.Process(ctx, net.Network_UDP, conn, w.dispatcher); err != nil {
				errors.LogInfoInner(ctx, err, "connection ends")
			}
			conn.Close()
			// conn not removed by checker TODO may be lock worker here is better
			if !conn.inactive {
				conn.setInactive()
				w.removeConn(id)
			}
		}()
	}
}

func (w *udpWorker) removeConn(id connID) {
	w.Lock()
	delete(w.activeConn, id)
	w.Unlock()
}

func (w *udpWorker) handlePackets() {
	receive := w.hub.Receive()
	for payload := range receive {
		w.callback(payload.Payload, payload.Source, payload.Target)
	}
}

func (w *udpWorker) clean() error {
	nowSec := time.Now().Unix()
	w.Lock()
	defer w.Unlock()

	if len(w.activeConn) == 0 {
		return errors.New("no more connections. stopping...")
	}

	for addr, conn := range w.activeConn {
		if nowSec-atomic.LoadInt64(&conn.lastActivityTime) > 2*60 {
			if !conn.inactive {
				conn.setInactive()
				delete(w.activeConn, addr)
			}
			conn.Close()
		}
	}

	if len(w.activeConn) == 0 {
		w.activeConn = make(map[connID]*udpConn, 16)
	}

	return nil
}

func (w *udpWorker) Start() error {
	w.activeConn = make(map[connID]*udpConn, 16)
	ctx := context.Background()
	h, err := udp.ListenUDP(ctx, w.address, w.port, w.stream, udp.HubCapacity(256))
	if err != nil {
		return err
	}

	w.cone = w.ctx.Value("cone").(bool)

	w.checker = &task.Periodic{
		Interval: time.Minute,
		Execute:  w.clean,
	}

	w.hub = h
	go w.handlePackets()
	return nil
}

func (w *udpWorker) Close() error {
	w.Lock()
	defer w.Unlock()

	var errs []interface{}

	if w.hub != nil {
		if err := w.hub.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if w.checker != nil {
		if err := w.checker.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if err := common.Close(w.proxy); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.New("failed to close all resources").Base(errors.New(serial.Concat(errs...)))
	}
	return nil
}

func (w *udpWorker) Port() net.Port {
	return w.port
}

func (w *udpWorker) Proxy() proxy.Inbound {
	return w.proxy
}

type dsWorker struct {
	address         net.Address
	proxy           proxy.Inbound
	stream          *internet.MemoryStreamConfig
	tag             string
	dispatcher      routing.Dispatcher
	sniffingConfig  *proxyman.SniffingConfig
	uplinkCounter   stats.Counter
	downlinkCounter stats.Counter

	hub internet.Listener

	ctx context.Context
}

func (w *dsWorker) callback(conn stat.Connection) {
	ctx, cancel := context.WithCancel(w.ctx)
	sid := session.NewID()
	ctx = c.ContextWithID(ctx, sid)

	if w.uplinkCounter != nil || w.downlinkCounter != nil {
		conn = &stat.CounterConnection{
			Connection:   conn,
			ReadCounter:  w.uplinkCounter,
			WriteCounter: w.downlinkCounter,
		}
	}
	ctx = session.ContextWithInbound(ctx, &session.Inbound{
		Source:  net.DestinationFromAddr(conn.RemoteAddr()),
		Local:   net.DestinationFromAddr(conn.LocalAddr()),
		Gateway: net.UnixDestination(w.address),
		Tag:     w.tag,
		Conn:    conn,
	})

	content := new(session.Content)
	if w.sniffingConfig != nil {
		content.SniffingRequest.Enabled = w.sniffingConfig.Enabled
		content.SniffingRequest.OverrideDestinationForProtocol = w.sniffingConfig.DestinationOverride
		content.SniffingRequest.ExcludeForDomain = w.sniffingConfig.DomainsExcluded
		content.SniffingRequest.MetadataOnly = w.sniffingConfig.MetadataOnly
		content.SniffingRequest.RouteOnly = w.sniffingConfig.RouteOnly
	}
	ctx = session.ContextWithContent(ctx, content)

	if err := w.proxy.Process(ctx, net.Network_UNIX, conn, w.dispatcher); err != nil {
		errors.LogInfoInner(ctx, err, "connection ends")
	}
	cancel()
	if err := conn.Close(); err != nil {
		errors.LogInfoInner(ctx, err, "failed to close connection")
	}
}

func (w *dsWorker) Proxy() proxy.Inbound {
	return w.proxy
}

func (w *dsWorker) Port() net.Port {
	return net.Port(0)
}

func (w *dsWorker) Start() error {
	ctx := context.Background()
	hub, err := internet.ListenUnix(ctx, w.address, w.stream, func(conn stat.Connection) {
		go w.callback(conn)
	})
	if err != nil {
		return errors.New("failed to listen Unix Domain Socket on ", w.address).AtWarning().Base(err)
	}
	w.hub = hub
	return nil
}

func (w *dsWorker) Close() error {
	var errs []interface{}
	if w.hub != nil {
		if err := common.Close(w.hub); err != nil {
			errs = append(errs, err)
		}
		if err := common.Close(w.proxy); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.New("failed to close all resources").Base(errors.New(serial.Concat(errs...)))
	}

	return nil
}

func IsLocal(ip net.IP) bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			if ipnet.IP.Equal(ip) {
				return true
			}
		}
	}
	return false
}
