package kcp

import (
	"context"
	"io"
	reflect "reflect"
	"sync/atomic"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/dice"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/net/cnc"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/stat"
	"github.com/xtls/xray-core/transport/internet/tls"
)

var globalConv = uint32(dice.RollUint16())

func fetchInput(_ context.Context, input io.Reader, reader PacketReader, conn *Connection) {
	cache := make(chan *buf.Buffer, 1024)
	go func() {
		for {
			payload := buf.New()
			if _, err := payload.ReadFrom(input); err != nil {
				payload.Release()
				close(cache)
				return
			}
			select {
			case cache <- payload:
			default:
				payload.Release()
			}
		}
	}()

	for payload := range cache {
		segments := reader.Read(payload.Bytes())
		payload.Release()
		if len(segments) > 0 {
			conn.Input(segments)
		}
	}
}

// DialKCP dials a new KCP connections to the specific destination.
func DialKCP(ctx context.Context, dest net.Destination, streamSettings *internet.MemoryStreamConfig) (stat.Connection, error) {
	dest.Network = net.Network_UDP
	errors.LogInfo(ctx, "dialing mKCP to ", dest)

	conn, err := internet.DialSystem(ctx, dest, streamSettings.SocketSettings)
	if err != nil {
		return nil, errors.New("failed to dial to dest: ", err).AtWarning().Base(err)
	}

	if streamSettings.UdpmaskManager != nil {
		switch c := conn.(type) {
		case *internet.PacketConnWrapper:
			pktConn, err := streamSettings.UdpmaskManager.WrapPacketConnClient(c.PacketConn)
			if err != nil {
				conn.Close()
				return nil, errors.New("mask err").Base(err)
			}
			c.PacketConn = pktConn
		case *net.UDPConn:
			pktConn, err := streamSettings.UdpmaskManager.WrapPacketConnClient(c)
			if err != nil {
				conn.Close()
				return nil, errors.New("mask err").Base(err)
			}
			conn = &internet.PacketConnWrapper{
				PacketConn: pktConn,
				Dest:       c.RemoteAddr().(*net.UDPAddr),
			}
		case *cnc.Connection:
			fakeConn := &internet.FakePacketConn{Conn: c}
			pktConn, err := streamSettings.UdpmaskManager.WrapPacketConnClient(fakeConn)
			if err != nil {
				conn.Close()
				return nil, errors.New("mask err").Base(err)
			}
			conn = &internet.PacketConnWrapper{
				PacketConn: pktConn,
				Dest: &net.UDPAddr{
					IP:   []byte{0, 0, 0, 0},
					Port: 0,
				},
			}
		default:
			conn.Close()
			return nil, errors.New("unknown conn ", reflect.TypeOf(c))
		}
	}

	kcpSettings := streamSettings.ProtocolSettings.(*Config)

	reader := &KCPPacketReader{}

	conv := uint16(atomic.AddUint32(&globalConv, 1))
	session := NewConnection(ConnMetadata{
		LocalAddr:    conn.LocalAddr(),
		RemoteAddr:   conn.RemoteAddr(),
		Conversation: conv,
	}, conn, conn, kcpSettings)

	go fetchInput(ctx, conn, reader, session)

	var iConn stat.Connection = session

	if config := tls.ConfigFromStreamSettings(streamSettings); config != nil {
		iConn = tls.Client(iConn, config.GetTLSConfig(tls.WithDestination(dest)))
	}

	return iConn, nil
}

func init() {
	common.Must(internet.RegisterTransportDialer(protocolName, DialKCP))
}
