package eventbus

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/op/go-logging"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
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

func TestBusBlockingSubscriberDoesNotStallOthers(t *testing.T) {
	b := New(16)
	defer b.Stop()

	release := make(chan struct{})
	b.Subscribe("blocking", func(e Event) {
		<-release
	})

	fast := make(chan struct{}, 1)
	b.Subscribe("fast", func(e Event) {
		fast <- struct{}{}
	})

	b.Publish(Event{Type: EventXrayCrash})

	select {
	case <-fast:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("a blocking subscriber stalled event delivery to another subscriber")
	}
	close(release)
}

func TestBusSubscriberRunsSerially(t *testing.T) {
	b := New(16)
	defer b.Stop()

	var inFlight atomic.Int32
	var maxSeen atomic.Int32
	var wg sync.WaitGroup
	const n = 8
	wg.Add(n)

	b.Subscribe("serial", func(Event) {
		cur := inFlight.Add(1)
		for {
			m := maxSeen.Load()
			if cur <= m || maxSeen.CompareAndSwap(m, cur) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		inFlight.Add(-1)
		wg.Done()
	})

	for i := 0; i < n; i++ {
		b.Publish(Event{Type: EventXrayCrash})
	}

	select {
	case <-waitDone(&wg):
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber did not process all events")
	}
	if got := maxSeen.Load(); got != 1 {
		t.Fatalf("subscriber ran concurrently with itself: max in-flight = %d, want 1", got)
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

func TestBusSubscribeAfterStopIsNoop(t *testing.T) {
	b := New(4)
	b.Stop()

	b.Subscribe("late", func(Event) {})

	b.mu.RLock()
	n := len(b.subs)
	b.mu.RUnlock()
	if n != 0 {
		t.Fatalf("Subscribe after Stop registered %d subscriber(s), want 0 (a stopped bus must not accept new subscribers, and must not call wg.Add after wg.Wait has been entered)", n)
	}
}
