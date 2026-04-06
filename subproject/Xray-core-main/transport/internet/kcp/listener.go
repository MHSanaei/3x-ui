package kcp

import (
	"context"
	gotls "crypto/tls"
	"sync"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/stat"
	"github.com/xtls/xray-core/transport/internet/tls"
	"github.com/xtls/xray-core/transport/internet/udp"
)

type ConnectionID struct {
	Remote net.Address
	Port   net.Port
	Conv   uint16
}

// Listener defines a server listening for connections
type Listener struct {
	sync.Mutex
	sessions  map[ConnectionID]*Connection
	hub       *udp.Hub
	tlsConfig *gotls.Config
	config    *Config
	reader    PacketReader
	addConn   internet.ConnHandler
}

func NewListener(ctx context.Context, address net.Address, port net.Port, streamSettings *internet.MemoryStreamConfig, addConn internet.ConnHandler) (*Listener, error) {
	kcpSettings := streamSettings.ProtocolSettings.(*Config)

	l := &Listener{
		reader:   &KCPPacketReader{},
		sessions: make(map[ConnectionID]*Connection),
		config:   kcpSettings,
		addConn:  addConn,
	}

	if config := tls.ConfigFromStreamSettings(streamSettings); config != nil {
		l.tlsConfig = config.GetTLSConfig()
	}

	hub, err := udp.ListenUDP(ctx, address, port, streamSettings, udp.HubCapacity(1024))
	if err != nil {
		return nil, err
	}
	l.Lock()
	l.hub = hub
	l.Unlock()
	errors.LogInfo(ctx, "listening on ", address, ":", port)

	go l.handlePackets()

	return l, nil
}

func (l *Listener) handlePackets() {
	receive := l.hub.Receive()
	for payload := range receive {
		l.OnReceive(payload.Payload, payload.Source)
	}
}

func (l *Listener) OnReceive(payload *buf.Buffer, src net.Destination) {
	segments := l.reader.Read(payload.Bytes())
	payload.Release()

	if len(segments) == 0 {
		errors.LogInfo(context.Background(), "discarding invalid payload from ", src)
		return
	}

	conv := segments[0].Conversation()
	cmd := segments[0].Command()

	id := ConnectionID{
		Remote: src.Address,
		Port:   src.Port,
		Conv:   conv,
	}

	l.Lock()
	defer l.Unlock()

	conn, found := l.sessions[id]

	if !found {
		if cmd == CommandTerminate {
			return
		}
		writer := &Writer{
			id:       id,
			hub:      l.hub,
			dest:     src,
			listener: l,
		}
		remoteAddr := &net.UDPAddr{
			IP:   src.Address.IP(),
			Port: int(src.Port),
		}
		localAddr := l.hub.Addr()
		conn = NewConnection(ConnMetadata{
			LocalAddr:    localAddr,
			RemoteAddr:   remoteAddr,
			Conversation: conv,
		}, writer, writer, l.config)
		var netConn stat.Connection = conn
		if l.tlsConfig != nil {
			netConn = tls.Server(conn, l.tlsConfig)
		}

		l.addConn(netConn)
		l.sessions[id] = conn
	}
	conn.Input(segments)
}

func (l *Listener) Remove(id ConnectionID) {
	l.Lock()
	delete(l.sessions, id)
	l.Unlock()
}

// Close stops listening on the UDP address. Already Accepted connections are not closed.
func (l *Listener) Close() error {
	l.hub.Close()

	l.Lock()
	defer l.Unlock()

	for _, conn := range l.sessions {
		go conn.Terminate()
	}

	return nil
}

func (l *Listener) ActiveConnections() int {
	l.Lock()
	defer l.Unlock()

	return len(l.sessions)
}

// Addr returns the listener's network address, The Addr returned is shared by all invocations of Addr, so do not modify it.
func (l *Listener) Addr() net.Addr {
	return l.hub.Addr()
}

type Writer struct {
	id       ConnectionID
	dest     net.Destination
	hub      *udp.Hub
	listener *Listener
}

func (w *Writer) Write(payload []byte) (int, error) {
	return w.hub.WriteTo(payload, w.dest)
}

func (w *Writer) Close() error {
	w.listener.Remove(w.id)
	return nil
}

func ListenKCP(ctx context.Context, address net.Address, port net.Port, streamSettings *internet.MemoryStreamConfig, addConn internet.ConnHandler) (internet.Listener, error) {
	return NewListener(ctx, address, port, streamSettings, addConn)
}

func init() {
	common.Must(internet.RegisterTransportListener(protocolName, ListenKCP))
}
