package eventbus

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/op/go-logging"
)

func TestMain(m *testing.M) {
	logger.InitLogger(logging.ERROR)
	m.Run()
}

func TestBusPublishSubscribe(t *testing.T) {
	b := New(16)
	defer b.Stop()

	var received Event
	var wg sync.WaitGroup
	wg.Add(1)

	b.Subscribe("test", func(e Event) {
		received = e
		wg.Done()
	})

	b.Publish(Event{Type: EventOutboundDown, Source: "my-proxy"})

	select {
	case <-waitDone(&wg):
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive event")
	}

	if received.Type != EventOutboundDown {
		t.Errorf("got type %q, want %q", received.Type, EventOutboundDown)
	}
	if received.Source != "my-proxy" {
		t.Errorf("got source %q, want %q", received.Source, "my-proxy")
	}
	if received.Timestamp.IsZero() {
		t.Error("timestamp not set")
	}
}

func TestBusMultipleSubscribers(t *testing.T) {
	b := New(16)
	defer b.Stop()

	var count atomic.Int32
	var wg sync.WaitGroup
	wg.Add(2)

	b.Subscribe("a", func(e Event) {
		count.Add(1)
		wg.Done()
	})
	b.Subscribe("b", func(e Event) {
		count.Add(1)
		wg.Done()
	})

	b.Publish(Event{Type: EventXrayCrash})

	select {
	case <-waitDone(&wg):
	case <-time.After(time.Second):
		t.Fatal("subscribers did not receive event")
	}

	if count.Load() != 2 {
		t.Errorf("got %d calls, want 2", count.Load())
	}
}

func TestBusUnsubscribe(t *testing.T) {
	b := New(16)
	defer b.Stop()

	var count atomic.Int32

	b.Subscribe("test", func(e Event) {
		count.Add(1)
	})
	b.Unsubscribe("test")

	b.Publish(Event{Type: EventOutboundUp})
	time.Sleep(50 * time.Millisecond)

	if count.Load() != 0 {
		t.Errorf("got %d calls after unsubscribe, want 0", count.Load())
	}
}

func TestBusReplaceSubscriber(t *testing.T) {
	b := New(16)
	defer b.Stop()

	var last string
	var wg sync.WaitGroup
	wg.Add(1)

	b.Subscribe("test", func(e Event) {
		last = "old"
	})
	b.Subscribe("test", func(e Event) {
		last = "new"
		wg.Done()
	})

	b.Publish(Event{Type: EventOutboundDown})

	select {
	case <-waitDone(&wg):
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive event")
	}

	if last != "new" {
		t.Errorf("got %q, want %q", last, "new")
	}
}

func TestBusPanicRecovery(t *testing.T) {
	b := New(16)
	defer b.Stop()

	var wg sync.WaitGroup
	wg.Add(1)

	b.Subscribe("panicker", func(e Event) {
		panic("oops")
	})
	b.Subscribe("after", func(e Event) {
		wg.Done()
	})

	b.Publish(Event{Type: EventOutboundDown})

	select {
	case <-waitDone(&wg):
	case <-time.After(time.Second):
		t.Fatal("subscriber after panicker did not receive event")
	}
}

func TestBusBufferFull(t *testing.T) {
	b := New(2)
	defer b.Stop()

	b.Subscribe("slow", func(e Event) {
		time.Sleep(100 * time.Millisecond)
	})

	b.Publish(Event{Type: EventOutboundDown})
	b.Publish(Event{Type: EventOutboundUp})
	b.Publish(Event{Type: EventXrayCrash})

	time.Sleep(50 * time.Millisecond)
}

func TestBusZeroTimestamp(t *testing.T) {
	b := New(16)
	defer b.Stop()

	var received Event
	var wg sync.WaitGroup
	wg.Add(1)

	b.Subscribe("test", func(e Event) {
		received = e
		wg.Done()
	})

	b.Publish(Event{Type: EventOutboundDown})

	select {
	case <-waitDone(&wg):
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive event")
	}

	if received.Timestamp.IsZero() {
		t.Error("timestamp should be set automatically")
	}
}

func waitDone(wg *sync.WaitGroup) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		wg.Wait()
		close(ch)
	}()
	return ch
}
