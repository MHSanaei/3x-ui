package reverse

import (
	"context"
	"time"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/mux"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/common/signal"
	"github.com/xtls/xray-core/common/task"
	"github.com/xtls/xray-core/features/routing"
	"github.com/xtls/xray-core/transport"
	"github.com/xtls/xray-core/transport/pipe"
	"google.golang.org/protobuf/proto"
)

// Bridge is a component in reverse proxy, that relays connections from Portal to local address.
type Bridge struct {
	dispatcher  routing.Dispatcher
	tag         string
	domain      string
	workers     []*BridgeWorker
	monitorTask *task.Periodic
}

// NewBridge creates a new Bridge instance.
func NewBridge(config *BridgeConfig, dispatcher routing.Dispatcher) (*Bridge, error) {
	if config.Tag == "" {
		return nil, errors.New("bridge tag is empty")
	}
	if config.Domain == "" {
		return nil, errors.New("bridge domain is empty")
	}

	b := &Bridge{
		dispatcher: dispatcher,
		tag:        config.Tag,
		domain:     config.Domain,
	}
	b.monitorTask = &task.Periodic{
		Execute:  b.monitor,
		Interval: time.Second * 2,
	}
	return b, nil
}

func (b *Bridge) cleanup() {
	var activeWorkers []*BridgeWorker

	for _, w := range b.workers {
		if w.IsActive() {
			activeWorkers = append(activeWorkers, w)
		}
		if w.Closed() {
			if w.Timer != nil {
				w.Timer.SetTimeout(0)
			}
		}
	}

	if len(activeWorkers) != len(b.workers) {
		b.workers = activeWorkers
	}
}

func (b *Bridge) monitor() error {
	b.cleanup()

	var numConnections uint32
	var numWorker uint32

	for _, w := range b.workers {
		if w.IsActive() {
			numConnections += w.Connections()
			numWorker++
		}
	}

	if numWorker == 0 || numConnections/numWorker > 16 {
		worker, err := NewBridgeWorker(b.domain, b.tag, b.dispatcher)
		if err != nil {
			errors.LogWarningInner(context.Background(), err, "failed to create bridge worker")
			return nil
		}
		b.workers = append(b.workers, worker)
	}

	return nil
}

func (b *Bridge) Start() error {
	return b.monitorTask.Start()
}

func (b *Bridge) Close() error {
	return b.monitorTask.Close()
}

type BridgeWorker struct {
	Tag        string
	Worker     *mux.ServerWorker
	Dispatcher routing.Dispatcher
	State      Control_State
	Timer      *signal.ActivityTimer
}

func NewBridgeWorker(domain string, tag string, d routing.Dispatcher) (*BridgeWorker, error) {
	ctx := context.Background()
	ctx = session.ContextWithInbound(ctx, &session.Inbound{
		Tag: tag,
	})
	link, err := d.Dispatch(ctx, net.Destination{
		Network: net.Network_TCP,
		Address: net.DomainAddress(domain),
		Port:    0,
	})
	if err != nil {
		return nil, err
	}

	w := &BridgeWorker{
		Dispatcher: d,
		Tag:        tag,
	}

	worker, err := mux.NewServerWorker(context.Background(), w, link)
	if err != nil {
		return nil, err
	}
	w.Worker = worker

	terminate := func() {
		worker.Close()
	}
	w.Timer = signal.CancelAfterInactivity(ctx, terminate, 60*time.Second)
	return w, nil
}

func (w *BridgeWorker) Type() interface{} {
	return routing.DispatcherType()
}

func (w *BridgeWorker) Start() error {
	return nil
}

func (w *BridgeWorker) Close() error {
	return nil
}

func (w *BridgeWorker) IsActive() bool {
	return w.State == Control_ACTIVE && !w.Worker.Closed()
}

func (w *BridgeWorker) Closed() bool {
	return w.Worker.Closed()
}

func (w *BridgeWorker) Connections() uint32 {
	return w.Worker.ActiveConnections()
}

func (w *BridgeWorker) handleInternalConn(link *transport.Link) {
	reader := link.Reader
	for {
		mb, err := reader.ReadMultiBuffer()
		if err != nil {
			if w.Timer != nil {
				if w.Closed() {
					w.Timer.SetTimeout(0)
				} else {
					w.Timer.SetTimeout(24 * time.Hour)
				}
			}
			return
		}
		if w.Timer != nil {
			w.Timer.Update()
		}
		for _, b := range mb {
			var ctl Control
			if err := proto.Unmarshal(b.Bytes(), &ctl); err != nil {
				errors.LogInfoInner(context.Background(), err, "failed to parse proto message")
				if w.Timer != nil {
					w.Timer.SetTimeout(0)
				}
				return
			}
			if ctl.State != w.State {
				w.State = ctl.State
			}
		}
	}
}

func (w *BridgeWorker) Dispatch(ctx context.Context, dest net.Destination) (*transport.Link, error) {
	if !isInternalDomain(dest) {
		if session.InboundFromContext(ctx) == nil {
			ctx = session.ContextWithInbound(ctx, &session.Inbound{
				Tag: w.Tag,
			})
		}
		return w.Dispatcher.Dispatch(ctx, dest)
	}

	opt := []pipe.Option{pipe.WithSizeLimit(16 * 1024)}
	uplinkReader, uplinkWriter := pipe.New(opt...)
	downlinkReader, downlinkWriter := pipe.New(opt...)

	go w.handleInternalConn(&transport.Link{
		Reader: downlinkReader,
		Writer: uplinkWriter,
	})

	return &transport.Link{
		Reader: uplinkReader,
		Writer: downlinkWriter,
	}, nil
}

func (w *BridgeWorker) DispatchLink(ctx context.Context, dest net.Destination, link *transport.Link) error {
	if !isInternalDomain(dest) {
		if session.InboundFromContext(ctx) == nil {
			ctx = session.ContextWithInbound(ctx, &session.Inbound{
				Tag: w.Tag,
			})
		}
		return w.Dispatcher.DispatchLink(ctx, dest, link)
	}
	w.handleInternalConn(link)

	return nil
}
