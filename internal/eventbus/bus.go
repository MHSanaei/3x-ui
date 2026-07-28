package eventbus

import (
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// DefaultBufferSize is the number of events the bus can hold before Publish starts dropping.
const DefaultBufferSize = 256

// subscriberQueueSize bounds how many undelivered events a single subscriber may
// hold before the newest are dropped. Each subscriber drains its own queue on a
// dedicated worker goroutine, so a slow subscriber can neither stall delivery to
// the others nor make the bus spawn an unbounded number of goroutines.
const subscriberQueueSize = 64

// subscriber pairs an ID with its event handler and the per-subscriber worker
// state used to deliver events to it serially, without blocking the dispatch loop.
type subscriber struct {
	id      string
	handler func(Event)
	queue   chan Event
	quit    chan struct{}
}

// Bus is a minimal in-process pub/sub event bus backed by a buffered channel.
// Producers call Publish (non-blocking) and every event is fanned out to all
// subscribers; per-event filtering is the subscriber's responsibility.
type Bus struct {
	ch      chan Event
	subs    []*subscriber
	mu      sync.RWMutex
	done    chan struct{}
	wg      sync.WaitGroup
	stopped bool
}

// New creates a Bus with the given buffer size. Use 0 for DefaultBufferSize.
func New(bufSize int) *Bus {
	if bufSize <= 0 {
		bufSize = DefaultBufferSize
	}
	b := &Bus{
		ch:   make(chan Event, bufSize),
		done: make(chan struct{}),
	}
	b.wg.Add(1)
	go b.dispatch()
	return b
}

// Subscribe registers a handler that receives every published event on its own
// worker goroutine. The id is used for Unsubscribe; it must be unique across
// active subscribers. Subscribing with an already-registered id replaces the
// previous subscriber, stopping its worker.
func (b *Bus) Subscribe(id string, handler func(Event)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stopped {
		return
	}
	for i, s := range b.subs {
		if s.id == id {
			close(s.quit)
			b.subs = append(b.subs[:i], b.subs[i+1:]...)
			break
		}
	}
	s := &subscriber{
		id:      id,
		handler: handler,
		queue:   make(chan Event, subscriberQueueSize),
		quit:    make(chan struct{}),
	}
	b.subs = append(b.subs, s)
	b.wg.Add(1)
	go b.runWorker(s)
}

// Unsubscribe removes a subscriber by id and stops its worker. Safe to call with an unknown id.
func (b *Bus) Unsubscribe(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, s := range b.subs {
		if s.id == id {
			close(s.quit)
			b.subs = append(b.subs[:i], b.subs[i+1:]...)
			return
		}
	}
}

// Publish sends an event to all subscribers. Non-blocking — if the buffer is
// full the event is dropped and a warning is logged.
func (b *Bus) Publish(e Event) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	select {
	case b.ch <- e:
	default:
		logger.Warning("eventbus: buffer full, dropping event ", e.Type)
	}
}

// dispatch is the fan-out loop. It reads events from the channel and hands each
// one to every subscriber's queue with a non-blocking send, so a subscriber
// whose handler blocks on network I/O (the email and Telegram notifiers can
// block for tens of seconds) can neither stall delivery of unrelated, higher-
// value events such as xray.crash or node.down, nor force the bus to spawn an
// unbounded number of goroutines under load. A subscriber whose queue is full
// drops the event, keeping the bus non-blocking and its memory bounded.
func (b *Bus) dispatch() {
	defer b.wg.Done()
	for {
		select {
		case e, ok := <-b.ch:
			if !ok {
				return
			}
			b.mu.RLock()
			for _, s := range b.subs {
				select {
				case s.queue <- e:
				default:
					logger.Warning("eventbus: subscriber ", s.id, " queue full, dropping ", e.Type)
				}
			}
			b.mu.RUnlock()
		case <-b.done:
			return
		}
	}
}

// runWorker delivers queued events to one subscriber serially, so a subscriber
// never runs concurrently with itself and observes events in publication order.
func (b *Bus) runWorker(s *subscriber) {
	defer b.wg.Done()
	for {
		select {
		case e := <-s.queue:
			safeCall(s.handler, e)
		case <-s.quit:
			return
		case <-b.done:
			return
		}
	}
}

// safeCall invokes handler with panic recovery.
func safeCall(fn func(Event), e Event) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("eventbus: subscriber panicked on %s: %v", e.Type, r)
		}
	}()
	fn(e)
}

// Stop shuts down the bus: the dispatch loop and every subscriber worker exit
// after finishing any handler already in progress, and any events still buffered
// or queued may be dropped. Safe to call once. After Stop returns, Subscribe is
// a no-op — this also keeps Subscribe's wg.Add from ever racing with Wait below,
// since both are serialized through mu.
func (b *Bus) Stop() {
	b.mu.Lock()
	b.stopped = true
	b.mu.Unlock()
	close(b.done)
	b.wg.Wait()
}
