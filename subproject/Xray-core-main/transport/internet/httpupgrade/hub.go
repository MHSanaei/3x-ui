package httpupgrade

import (
	"bufio"
	"context"
	"crypto/tls"
	"net/http"
	"strings"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	http_proto "github.com/xtls/xray-core/common/protocol/http"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/stat"
	v2tls "github.com/xtls/xray-core/transport/internet/tls"
)

type server struct {
	config         *Config
	addConn        internet.ConnHandler
	innnerListener net.Listener
	socketSettings *internet.SocketConfig
}

func (s *server) Close() error {
	return s.innnerListener.Close()
}

func (s *server) Addr() net.Addr {
	return nil
}

func (s *server) Handle(conn net.Conn) {
	upgradedConn, err := s.upgrade(conn)
	if err != nil {
		common.CloseIfExists(conn)
		errors.LogInfoInner(context.Background(), err, "failed to handle request")
		return
	}
	s.addConn(upgradedConn)
}

// upgrade execute a fake websocket upgrade process and return the available connection
func (s *server) upgrade(conn net.Conn) (stat.Connection, error) {
	connReader := bufio.NewReader(conn)
	req, err := http.ReadRequest(connReader)
	if err != nil {
		return nil, err
	}

	if s.config != nil {
		host := req.Host
		if len(s.config.Host) > 0 && !internet.IsValidHTTPHost(host, s.config.Host) {
			return nil, errors.New("bad host: ", host)
		}
		path := s.config.GetNormalizedPath()
		if req.URL.Path != path {
			return nil, errors.New("bad path: ", req.URL.Path)
		}
	}

	connection := strings.ToLower(req.Header.Get("Connection"))
	upgrade := strings.ToLower(req.Header.Get("Upgrade"))
	if connection != "upgrade" || upgrade != "websocket" {
		return nil, errors.New("unrecognized request")
	}
	resp := &http.Response{
		Status:     "101 Switching Protocols",
		StatusCode: 101,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     http.Header{},
	}
	resp.Header.Set("Connection", "Upgrade")
	resp.Header.Set("Upgrade", "websocket")
	err = resp.Write(conn)
	if err != nil {
		return nil, err
	}

	var forwardedAddrs []net.Address
	if s.socketSettings != nil && len(s.socketSettings.TrustedXForwardedFor) > 0 {
		for _, key := range s.socketSettings.TrustedXForwardedFor {
			if len(req.Header.Values(key)) > 0 {
				forwardedAddrs = http_proto.ParseXForwardedFor(req.Header)
				break
			}
		}
	} else {
		forwardedAddrs = http_proto.ParseXForwardedFor(req.Header)
	}
	remoteAddr := conn.RemoteAddr()
	if len(forwardedAddrs) > 0 && forwardedAddrs[0].Family().IsIP() {
		remoteAddr = &net.TCPAddr{
			IP:   forwardedAddrs[0].IP(),
			Port: int(0),
		}
	}

	return stat.Connection(newConnection(conn, remoteAddr)), nil
}

func (s *server) keepAccepting() {
	for {
		conn, err := s.innnerListener.Accept()
		if err != nil {
			return
		}
		go s.Handle(conn)
	}
}

func ListenHTTPUpgrade(ctx context.Context, address net.Address, port net.Port, streamSettings *internet.MemoryStreamConfig, addConn internet.ConnHandler) (internet.Listener, error) {
	transportConfiguration := streamSettings.ProtocolSettings.(*Config)
	if transportConfiguration != nil {
		if streamSettings.SocketSettings == nil {
			streamSettings.SocketSettings = &internet.SocketConfig{}
		}
		streamSettings.SocketSettings.AcceptProxyProtocol = transportConfiguration.AcceptProxyProtocol || streamSettings.SocketSettings.AcceptProxyProtocol
	}
	var listener net.Listener
	var err error
	if port == net.Port(0) { // unix
		listener, err = internet.ListenSystem(ctx, &net.UnixAddr{
			Name: address.Domain(),
			Net:  "unix",
		}, streamSettings.SocketSettings)
		if err != nil {
			return nil, errors.New("failed to listen unix domain socket(for HttpUpgrade) on ", address).Base(err)
		}
		errors.LogInfo(ctx, "listening unix domain socket(for HttpUpgrade) on ", address)
	} else { // tcp
		listener, err = internet.ListenSystem(ctx, &net.TCPAddr{
			IP:   address.IP(),
			Port: int(port),
		}, streamSettings.SocketSettings)
		if err != nil {
			return nil, errors.New("failed to listen TCP(for HttpUpgrade) on ", address, ":", port).Base(err)
		}
		errors.LogInfo(ctx, "listening TCP(for HttpUpgrade) on ", address, ":", port)
	}

	if streamSettings.TcpmaskManager != nil {
		listener, _ = streamSettings.TcpmaskManager.WrapListener(listener)
	}

	if streamSettings.SocketSettings != nil && streamSettings.SocketSettings.AcceptProxyProtocol {
		errors.LogWarning(ctx, "accepting PROXY protocol")
	}

	if config := v2tls.ConfigFromStreamSettings(streamSettings); config != nil {
		if tlsConfig := config.GetTLSConfig(); tlsConfig != nil {
			listener = tls.NewListener(listener, tlsConfig)
		}
	}

	serverInstance := &server{
		config:         transportConfiguration,
		addConn:        addConn,
		innnerListener: listener,
		socketSettings: streamSettings.SocketSettings,
	}
	go serverInstance.keepAccepting()
	return serverInstance, nil
}

func init() {
	common.Must(internet.RegisterTransportListener(protocolName, ListenHTTPUpgrade))
}
