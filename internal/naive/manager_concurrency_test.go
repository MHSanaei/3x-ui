package naive

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestRunConcurrentRunsAllTagsInParallel(t *testing.T) {
	var active atomic.Int32
	var peak atomic.Int32
	release := make(chan struct{})
	started := make(chan struct{}, 3)

	done := make(chan error, 1)
	go func() {
		done <- runConcurrent([]string{"one", "two", "three"}, func(string) error {
			current := active.Add(1)
			for {
				previous := peak.Load()
				if current <= previous || peak.CompareAndSwap(previous, current) {
					break
				}
			}
			started <- struct{}{}
			<-release
			active.Add(-1)
			return nil
		})
	}()

	for range 3 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("bulk lifecycle did not start all operations concurrently")
		}
	}
	close(release)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runConcurrent returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("bulk lifecycle did not complete")
	}
	if peak.Load() != 3 {
		t.Fatalf("peak concurrency = %d, want 3", peak.Load())
	}
}
