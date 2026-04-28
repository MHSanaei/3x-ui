package websocket

import (
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/op/go-logging"
)

func TestMain(m *testing.M) {
	// Initialize logger so hub.go calls don't panic on nil global.
	logger.InitLogger(logging.CRITICAL)
	code := m.Run()
	logger.CloseLogger()
	// Clean up the log directory created by InitLogger so the test leaves
	// no artefacts in the working tree.
	os.RemoveAll("log")
	os.Exit(code)
}

// TestFanoutNoDeadlockOnSlowClients verifies that the hub does NOT self-deadlock
// when many clients have full Send buffers simultaneously. Regression guard for
// the bug where fanout called Unregister() on each slow client, the unregister
// channel filled (cap 64), and the hub blocked on its own consumer.
func TestFanoutNoDeadlockOnSlowClients(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	// Spawn 200 clients but never read from their Send channels — all are "slow".
	// 200 > unregister channel capacity (64), which would have triggered the
	// deadlock in the old code.
	const n = 200
	clients := make([]*Client, n)
	for i := 0; i < n; i++ {
		clients[i] = NewClient(string(rune('a' + i%26)))
		hub.Register(clients[i])
	}
	// Wait for registrations to be processed.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && hub.GetClientCount() < n {
		time.Sleep(10 * time.Millisecond)
	}
	if got := hub.GetClientCount(); got < n {
		t.Fatalf("only %d/%d clients registered after 2s", got, n)
	}

	// Fill every client's send buffer so the next broadcast triggers eviction.
	for _, c := range clients {
		for i := 0; i < clientSendQueue; i++ {
			select {
			case c.Send <- []byte("filler"):
			default:
			}
		}
	}

	// This broadcast should evict ALL clients without deadlocking the hub.
	hub.Broadcast(MessageTypeStatus, map[string]string{"x": "y"})

	// Wait for eviction with a hard cap — if the hub deadlocked, this hangs
	// past the timeout and t.Fatalf fires.
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && hub.GetClientCount() > 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if got := hub.GetClientCount(); got > 0 {
		t.Fatalf("deadlock: %d clients still registered after broadcast (expected 0)", got)
	}
}

// TestConcurrentBroadcastAndDisconnect stresses the hub with parallel
// Broadcast calls while clients connect and disconnect. Regression guard for
// races between fanout, removeClient, and shutdown.
func TestConcurrentBroadcastAndDisconnect(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Continuous broadcasters.
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					hub.Broadcast(MessageTypeStatus, map[string]int{"v": 1})
				}
			}
		}()
	}

	// Continuous register/unregister churn.
	var connected int64
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				c := NewClient("churn")
				hub.Register(c)
				atomic.AddInt64(&connected, 1)
				// Drain a few messages so we don't block.
				go func() {
					for range c.Send {
					}
				}()
				time.Sleep(time.Millisecond)
				hub.Unregister(c)
			}
		}
	}()

	time.Sleep(500 * time.Millisecond)
	close(stop)
	wg.Wait()

	if atomic.LoadInt64(&connected) == 0 {
		t.Fatal("no clients churned through hub")
	}
}

// TestThrottlingBlocksBurstButLetsRealtimeThrough verifies that real-time
// message types (status, traffic) are NEVER throttled, while inbounds bursts
// are throttled.
func TestThrottlingBlocksBurstButLetsRealtimeThrough(t *testing.T) {
	hub := NewHub()

	if hub.shouldThrottle(MessageTypeStatus) {
		t.Error("status must never be throttled")
	}
	if hub.shouldThrottle(MessageTypeTraffic) {
		t.Error("traffic must never be throttled")
	}
	if hub.shouldThrottle(MessageTypeNotification) {
		t.Error("notification must never be throttled")
	}
	if hub.shouldThrottle(MessageTypeInvalidate) {
		t.Error("invalidate must never be throttled")
	}

	// First inbounds broadcast goes through, immediate retry is throttled.
	if hub.shouldThrottle(MessageTypeInbounds) {
		t.Error("first inbounds broadcast must pass")
	}
	if !hub.shouldThrottle(MessageTypeInbounds) {
		t.Error("second inbounds broadcast within window must throttle")
	}

	// After the window passes, throttle releases.
	time.Sleep(minBroadcastInterval + 10*time.Millisecond)
	if hub.shouldThrottle(MessageTypeInbounds) {
		t.Error("inbounds broadcast after window must pass")
	}
}

// TestHubStopUnblocksWaiters ensures that pending Broadcast/Register/Unregister
// calls don't leak goroutines after Stop().
func TestHubStopUnblocksWaiters(t *testing.T) {
	hub := NewHub()
	// Don't start Run — leave channels unfeed so any blocking call would hang.

	hub.Stop()

	done := make(chan struct{})
	go func() {
		// All these should return promptly since ctx is cancelled.
		hub.Register(NewClient("x"))
		hub.Unregister(NewClient("x"))
		hub.Broadcast(MessageTypeStatus, "data")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("calls did not return after Stop()")
	}
}
