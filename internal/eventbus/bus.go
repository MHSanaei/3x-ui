package eventbus

import (
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// DefaultBufferSize is the number of events the bus can hold before Publish starts dropping.
const DefaultBufferSize = 256

// subscriber pairs an ID with its event handler.
type subscriber struct {
	id      string
	handler func(Event)
}

// Bus is a minimal in-process pub/sub event bus backed by a buffered channel.
// Producers call Publish (non-blocking) and every event is fanned out to all
// subscribers; per-event filtering is the subscriber's responsibility.
type Bus struct {
	ch   chan Event
	subs []subscriber
	mu   sync.RWMutex
	done chan struct{}
	wg   sync.WaitGroup
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

// Subscribe registers a handler that receives every published event.
// The id is used for Unsubscribe; it must be unique across active subscribers.
// Subscribing with an already-registered id replaces the previous handler.
func (b *Bus) Subscribe(id string, handler func(Event)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, s := range b.subs {
		if s.id == id {
			b.subs[i].handler = handler
			return
		}
	}
	b.subs = append(b.subs, subscriber{id: id, handler: handler})
}

// Unsubscribe removes a subscriber by id. Safe to call with unknown id.
func (b *Bus) Unsubscribe(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, s := range b.subs {
		if s.id == id {
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

// dispatch is the fan-out loop. It reads events from the channel and calls
// every subscriber's handler sequentially. Handlers run on the dispatch
// goroutine — they must not block.
func (b *Bus) dispatch() {
	defer b.wg.Done()
	for {
		select {
		case e, ok := <-b.ch:
			if !ok {
				return
			}
			b.mu.RLock()
			subs := make([]subscriber, len(b.subs))
			copy(subs, b.subs)
			b.mu.RUnlock()
			for _, s := range subs {
				safeCall(s.handler, e)
			}
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

// Stop shuts down the bus: the dispatch goroutine exits, in-flight handlers
// finish, and any events still buffered may be dropped. Safe to call once.
func (b *Bus) Stop() {
	close(b.done)
	b.wg.Wait()
}
