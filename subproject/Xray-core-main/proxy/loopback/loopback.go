package loopback

import (
	"context"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/net/cnc"
	"github.com/xtls/xray-core/common/retry"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/common/task"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/routing"
	"github.com/xtls/xray-core/transport"
	"github.com/xtls/xray-core/transport/internet"
)

type Loopback struct {
	config             *Config
	dispatcherInstance routing.Dispatcher
}

func (l *Loopback) Process(ctx context.Context, link *transport.Link, _ internet.Dialer) error {
	outbounds := session.OutboundsFromContext(ctx)
	ob := outbounds[len(outbounds)-1]
	if !ob.Target.IsValid() {
		return errors.New("target not specified.")
	}
	ob.Name = "loopback"
	destination := ob.Target

	errors.LogInfo(ctx, "opening connection to ", destination)

	input := link.Reader
	output := link.Writer

	var conn net.Conn
	err := retry.ExponentialBackoff(2, 100).On(func() error {
		dialDest := destination

		content := new(session.Content)
		content.SkipDNSResolve = true

		ctx = session.ContextWithContent(ctx, content)

		inbound := session.InboundFromContext(ctx)
		if inbound == nil {
			inbound = &session.Inbound{}
		}

		inbound.Tag = l.config.InboundTag

		ctx = session.ContextWithInbound(ctx, inbound)

		rawConn, err := l.dispatcherInstance.Dispatch(ctx, dialDest)
		if err != nil {
			return err
		}

		var readerOpt cnc.ConnectionOption
		if dialDest.Network == net.Network_TCP {
			readerOpt = cnc.ConnectionOutputMulti(rawConn.Reader)
		} else {
			readerOpt = cnc.ConnectionOutputMultiUDP(rawConn.Reader)
		}

		conn = cnc.NewConnection(cnc.ConnectionInputMulti(rawConn.Writer), readerOpt)
		return nil
	})
	if err != nil {
		return errors.New("failed to open connection to ", destination).Base(err)
	}
	defer conn.Close()

	requestDone := func() error {
		var writer buf.Writer
		if destination.Network == net.Network_TCP {
			writer = buf.NewWriter(conn)
		} else {
			writer = &buf.SequentialWriter{Writer: conn}
		}

		if err := buf.Copy(input, writer); err != nil {
			return errors.New("failed to process request").Base(err)
		}

		return nil
	}

	responseDone := func() error {
		var reader buf.Reader
		if destination.Network == net.Network_TCP {
			reader = buf.NewReader(conn)
		} else {
			reader = buf.NewPacketReader(conn)
		}
		if err := buf.Copy(reader, output); err != nil {
			return errors.New("failed to process response").Base(err)
		}

		return nil
	}

	if err := task.Run(ctx, requestDone, task.OnSuccess(responseDone, task.Close(output))); err != nil {
		return errors.New("connection ends").Base(err)
	}

	return nil
}

func (l *Loopback) init(config *Config, dispatcherInstance routing.Dispatcher) error {
	l.dispatcherInstance = dispatcherInstance
	l.config = config
	return nil
}

func init() {
	common.Must(common.RegisterConfig((*Config)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		l := new(Loopback)
		err := core.RequireFeatures(ctx, func(dispatcherInstance routing.Dispatcher) error {
			return l.init(config.(*Config), dispatcherInstance)
		})
		return l, err
	}))
}
