package hysteria

import (
	"context"
	go_errors "errors"
	"io"
	"math/rand"

	"github.com/apernet/quic-go"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/common/signal"
	"github.com/xtls/xray-core/common/task"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/policy"
	hyCtx "github.com/xtls/xray-core/proxy/hysteria/ctx"
	"github.com/xtls/xray-core/transport"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/hysteria"
	"github.com/xtls/xray-core/transport/internet/stat"
)

type Client struct {
	server        *protocol.ServerSpec
	policyManager policy.Manager
}

func NewClient(ctx context.Context, config *ClientConfig) (*Client, error) {
	if config.Server == nil {
		return nil, errors.New(`no target server found`)
	}
	server, err := protocol.NewServerSpecFromPB(config.Server)
	if err != nil {
		return nil, errors.New("failed to get server spec").Base(err)
	}

	v := core.MustFromContext(ctx)
	client := &Client{
		server:        server,
		policyManager: v.GetFeature(policy.ManagerType()).(policy.Manager),
	}
	return client, nil
}

func (c *Client) Process(ctx context.Context, link *transport.Link, dialer internet.Dialer) error {
	outbounds := session.OutboundsFromContext(ctx)
	ob := outbounds[len(outbounds)-1]
	if !ob.Target.IsValid() {
		return errors.New("target not specified")
	}
	ob.Name = "hysteria"
	ob.CanSpliceCopy = 3
	target := ob.Target

	conn, err := dialer.Dial(hyCtx.ContextWithRequireDatagram(ctx, target.Network == net.Network_UDP), c.server.Destination)
	if err != nil {
		return errors.New("failed to find an available destination").AtWarning().Base(err)
	}
	defer conn.Close()
	errors.LogInfo(ctx, "tunneling request to ", target, " via ", target.Network, ":", c.server.Destination.NetAddr())

	var newCtx context.Context
	var newCancel context.CancelFunc
	if session.TimeoutOnlyFromContext(ctx) {
		newCtx, newCancel = context.WithCancel(context.Background())
	}

	sessionPolicy := c.policyManager.ForLevel(0)
	ctx, cancel := context.WithCancel(ctx)
	timer := signal.CancelAfterInactivity(ctx, func() {
		cancel()
		if newCancel != nil {
			newCancel()
		}
	}, sessionPolicy.Timeouts.ConnectionIdle)

	if newCtx != nil {
		ctx = newCtx
	}

	if target.Network == net.Network_TCP {
		requestDone := func() error {
			defer timer.SetTimeout(sessionPolicy.Timeouts.DownlinkOnly)
			bufferedWriter := buf.NewBufferedWriter(buf.NewWriter(conn))
			err := WriteTCPRequest(bufferedWriter, target.NetAddr())
			if err != nil {
				return errors.New("failed to write request").Base(err)
			}
			if err := bufferedWriter.SetBuffered(false); err != nil {
				return err
			}
			return buf.Copy(link.Reader, bufferedWriter, buf.UpdateActivity(timer))
		}

		responseDone := func() error {
			defer timer.SetTimeout(sessionPolicy.Timeouts.UplinkOnly)
			ok, msg, err := ReadTCPResponse(conn)
			if err != nil {
				return err
			}
			if !ok {
				return errors.New(msg)
			}
			return buf.Copy(buf.NewReader(conn), link.Writer, buf.UpdateActivity(timer))
		}

		responseDoneAndCloseWriter := task.OnSuccess(responseDone, task.Close(link.Writer))
		if err := task.Run(ctx, requestDone, responseDoneAndCloseWriter); err != nil {
			return errors.New("connection ends").Base(err)
		}

		return nil
	}

	if target.Network == net.Network_UDP {
		iConn := stat.TryUnwrapStatsConn(conn)
		_, ok := iConn.(*hysteria.InterUdpConn)
		if !ok {
			return errors.New("udp requires hysteria udp transport")
		}

		requestDone := func() error {
			defer timer.SetTimeout(sessionPolicy.Timeouts.DownlinkOnly)

			writer := &UDPWriter{
				Writer: conn,
				buf:    make([]byte, MaxUDPSize),
				addr:   target.NetAddr(),
			}

			if err := buf.Copy(link.Reader, writer, buf.UpdateActivity(timer)); err != nil {
				return errors.New("failed to transport all UDP request").Base(err)
			}

			return nil
		}

		responseDone := func() error {
			defer timer.SetTimeout(sessionPolicy.Timeouts.UplinkOnly)

			reader := &UDPReader{
				Reader: conn,
				buf:    make([]byte, MaxUDPSize),
				df:     &Defragger{},
			}

			if err := buf.Copy(reader, link.Writer, buf.UpdateActivity(timer)); err != nil {
				return errors.New("failed to transport all UDP response").Base(err)
			}

			return nil
		}

		responseDoneAndCloseWriter := task.OnSuccess(responseDone, task.Close(link.Writer))
		if err := task.Run(ctx, requestDone, responseDoneAndCloseWriter); err != nil {
			return errors.New("connection ends").Base(err)
		}

		return nil
	}

	return nil
}

func init() {
	common.Must(common.RegisterConfig((*ClientConfig)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return NewClient(ctx, config.(*ClientConfig))
	}))
}

type UDPWriter struct {
	Writer io.Writer
	buf    []byte
	addr   string
}

func (w *UDPWriter) sendMsg(msg *UDPMessage) error {
	msgN := msg.Serialize(w.buf)
	if msgN < 0 {
		return nil
	}
	_, err := w.Writer.Write(w.buf[:msgN])
	return err
}

func (w *UDPWriter) WriteMultiBuffer(mb buf.MultiBuffer) error {
	for {
		mb2, b := buf.SplitFirst(mb)
		mb = mb2
		if b == nil {
			break
		}

		addr := w.addr
		if b.UDP != nil {
			addr = b.UDP.NetAddr()
		}

		msg := &UDPMessage{
			SessionID: 0,
			PacketID:  0,
			FragID:    0,
			FragCount: 1,
			Addr:      addr,
			Data:      b.Bytes(),
		}

		err := w.sendMsg(msg)
		var errTooLarge *quic.DatagramTooLargeError
		if go_errors.As(err, &errTooLarge) {
			msg.PacketID = uint16(rand.Intn(0xFFFF)) + 1
			fMsgs := FragUDPMessage(msg, int(errTooLarge.MaxDatagramPayloadSize))
			for _, fMsg := range fMsgs {
				err := w.sendMsg(&fMsg)
				if err != nil {
					b.Release()
					buf.ReleaseMulti(mb)
					return err
				}
			}
		} else if err != nil {
			b.Release()
			buf.ReleaseMulti(mb)
			return err
		}

		b.Release()
	}

	return nil
}

type UDPReader struct {
	Reader    io.Reader
	buf       []byte
	df        *Defragger
	firstMsg  *UDPMessage
	firstDest *net.Destination
}

func (r *UDPReader) ReadMultiBuffer() (buf.MultiBuffer, error) {
	if r.firstMsg != nil {
		buffer := buf.New()
		buffer.Write(r.firstMsg.Data)
		buffer.UDP = r.firstDest

		r.firstMsg = nil

		return buf.MultiBuffer{buffer}, nil
	}
	for {
		n, err := r.Reader.Read(r.buf)
		if err != nil {
			return nil, err
		}

		msg, err := ParseUDPMessage(r.buf[:n])
		if err != nil {
			continue
		}

		dfMsg := r.df.Feed(msg)
		if dfMsg == nil {
			continue
		}

		dest, err := net.ParseDestination("udp:" + dfMsg.Addr)
		if err != nil {
			errors.LogDebug(context.Background(), dfMsg.Addr, " ParseDestination err ", err)
			continue
		}

		buffer := buf.New()
		buffer.Write(dfMsg.Data)
		buffer.UDP = &dest

		return buf.MultiBuffer{buffer}, nil
	}
}
