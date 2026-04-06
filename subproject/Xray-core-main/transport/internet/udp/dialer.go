package udp

import (
	"context"
	reflect "reflect"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/net/cnc"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/stat"
)

func init() {
	common.Must(internet.RegisterTransportDialer(protocolName,
		func(ctx context.Context, dest net.Destination, streamSettings *internet.MemoryStreamConfig) (stat.Connection, error) {
			var sockopt *internet.SocketConfig
			if streamSettings != nil {
				sockopt = streamSettings.SocketSettings
			}
			conn, err := internet.DialSystem(ctx, dest, sockopt)
			if err != nil {
				return nil, err
			}

			if streamSettings != nil && streamSettings.UdpmaskManager != nil {
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

			// TODO: handle dialer options
			return conn, nil
		}))
}
