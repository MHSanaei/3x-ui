package websocket

import (
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/op/go-logging"

	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("XUI_LOG_FOLDER", os.TempDir())
	xuilogger.InitLogger(logging.ERROR)
	os.Exit(m.Run())
}

func TestNewClient_HasBufferedSendChannel(t *testing.T) {
	c := NewClient("client-1")
	if c.ID != "client-1" {
		t.Fatalf("ID = %q, want client-1", c.ID)
	}
	if cap(c.Send) != clientSendQueue {
		t.Fatalf("Send cap = %d, want %d", cap(c.Send), clientSendQueue)
	}
}

func TestHub_NilReceiver_DoesNotPanic(t *testing.T) {
	var h *Hub
	if h.GetClientCount() != 0 {
		t.Fatal("nil hub GetClientCount should return 0")
	}
	h.Broadcast(MessageTypeStatus, "anything")
	h.Register(NewClient("x"))
	h.Unregister(NewClient("x"))
	h.Stop()
}

func TestHub_BroadcastDropsWhenNoClients(t *testing.T) {
	h := NewHub()
	defer h.Stop()
	go h.Run()

	h.Broadcast(MessageTypeStatus, "payload")

	select {
	case <-h.broadcast:
		t.Fatal("Broadcast should drop when client count is zero")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestHub_BroadcastDropsNilPayload(t *testing.T) {
	h := NewHub()
	defer h.Stop()
	go h.Run()

	c := NewClient("c1")
	h.Register(c)
	waitClientCount(t, h, 1)

	h.Broadcast(MessageTypeStatus, nil)

	select {
	case <-c.Send:
		t.Fatal("nil payload should be dropped, not delivered")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestHub_BroadcastDeliversToClient(t *testing.T) {
	h := NewHub()
	defer h.Stop()
	go h.Run()

	c := NewClient("c1")
	h.Register(c)
	waitClientCount(t, h, 1)

	h.Broadcast(MessageTypeStatus, map[string]string{"k": "v"})

	select {
	case raw := <-c.Send:
		var m Message
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("payload is not valid JSON: %v\n%s", err, raw)
		}
		if m.Type != MessageTypeStatus {
			t.Fatalf("Type = %q, want %q", m.Type, MessageTypeStatus)
		}
		if m.Time == 0 {
			t.Fatal("Time should be set to a non-zero unix-millis value")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for broadcast to reach client")
	}
}

func TestHub_UnregisterClosesSendAndDecrementsCount(t *testing.T) {
	h := NewHub()
	defer h.Stop()
	go h.Run()

	c := NewClient("c1")
	h.Register(c)
	waitClientCount(t, h, 1)

	h.Unregister(c)
	waitClientCount(t, h, 0)

	select {
	case _, ok := <-c.Send:
		if ok {
			t.Fatal("expected Send channel to be closed after Unregister")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Send channel was not closed after Unregister")
	}
}

func TestHub_StopClosesAllClients(t *testing.T) {
	h := NewHub()
	go h.Run()

	c1 := NewClient("c1")
	c2 := NewClient("c2")
	h.Register(c1)
	h.Register(c2)
	waitClientCount(t, h, 2)

	h.Stop()

	for _, c := range []*Client{c1, c2} {
		select {
		case _, ok := <-c.Send:
			if ok {
				t.Fatalf("client %s Send should be closed after Stop", c.ID)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("client %s Send not closed after Stop", c.ID)
		}
	}
}

func TestHub_ShouldThrottle(t *testing.T) {
	h := NewHub()
	defer h.Stop()

	if h.shouldThrottle(MessageTypeStatus) {
		t.Fatal("non-gated message type should never throttle")
	}
	if h.shouldThrottle(MessageTypeStatus) {
		t.Fatal("non-gated message type should never throttle on second call")
	}

	if h.shouldThrottle(MessageTypeTraffic) {
		t.Fatal("first call for gated type should not throttle")
	}
	if !h.shouldThrottle(MessageTypeTraffic) {
		t.Fatal("immediate second call for gated type should throttle")
	}
}

func TestHub_ShouldThrottle_DistinctTypesIndependent(t *testing.T) {
	h := NewHub()
	defer h.Stop()

	if h.shouldThrottle(MessageTypeTraffic) {
		t.Fatal("first Traffic call should not throttle")
	}
	if h.shouldThrottle(MessageTypeInbounds) {
		t.Fatal("first Inbounds call should not throttle even after Traffic")
	}
}

func TestTrySend_SucceedsWithRoom(t *testing.T) {
	c := &Client{ID: "c", Send: make(chan []byte, 1)}
	if !trySend(c, []byte("hi")) {
		t.Fatal("trySend should succeed when buffer has room")
	}
}

func TestTrySend_FailsWhenFull(t *testing.T) {
	c := &Client{ID: "c", Send: make(chan []byte, 1)}
	c.Send <- []byte("first")
	if trySend(c, []byte("second")) {
		t.Fatal("trySend should fail when buffer is full")
	}
}

func TestTrySend_FailsOnClosedChannel(t *testing.T) {
	c := &Client{ID: "c", Send: make(chan []byte, 1)}
	close(c.Send)
	if trySend(c, []byte("after-close")) {
		t.Fatal("trySend should fail (not panic) when channel is closed")
	}
}

func TestHub_FanoutEvictsSlowClient(t *testing.T) {
	h := NewHub()
	defer h.Stop()
	go h.Run()

	slow := &Client{ID: "slow", Send: make(chan []byte, 1)}
	slow.Send <- []byte("buffer-already-full")
	h.Register(slow)
	waitClientCount(t, h, 1)

	h.Broadcast(MessageTypeStatus, "payload")
	waitClientCount(t, h, 0)

	select {
	case _, ok := <-slow.Send:
		if ok {
			_, ok = <-slow.Send
			if ok {
				t.Fatal("slow client Send should eventually be closed by fanout eviction")
			}
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("slow client Send channel was not closed")
	}
}

func TestHub_ConcurrentRegisterUnregister(t *testing.T) {
	h := NewHub()
	defer h.Stop()
	go h.Run()

	const n = 50
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			c := NewClient("c")
			h.Register(c)
			h.Unregister(c)
		}(i)
	}
	wg.Wait()
	waitClientCount(t, h, 0)
}

func waitClientCount(t *testing.T, h *Hub, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if h.GetClientCount() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("client count never reached %d (last seen %d)", want, h.GetClientCount())
}
