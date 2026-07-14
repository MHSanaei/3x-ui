package service

import (
	"testing"
	"time"
)

// lockInbound must not hold the global registry mutex while it waits on a busy
// inbound's own mutex, otherwise one slow client operation on a single inbound
// freezes client mutations on every other inbound panel-wide.
func TestLockInboundReleasesRegistryMutexWhileWaiting(t *testing.T) {
	const id = 990006
	held := lockInbound(id)

	parked := make(chan struct{})
	go func() {
		close(parked)
		lockInbound(id).Unlock()
	}()
	<-parked
	time.Sleep(50 * time.Millisecond)

	if !inboundMutationLocksMu.TryLock() {
		held.Unlock()
		t.Fatal("registry mutex is held while a lockInbound caller waits on a busy inbound")
	}
	inboundMutationLocksMu.Unlock()
	held.Unlock()
}
