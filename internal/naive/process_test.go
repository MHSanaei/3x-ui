package naive

import (
	"testing"
	"time"
)

func TestWaitDoneClosed(t *testing.T) {
	done := make(chan struct{})
	close(done)
	if !waitDone(done, 10*time.Millisecond) {
		t.Fatal("waitDone returned false for closed channel")
	}
}

func TestWaitDoneTimeout(t *testing.T) {
	done := make(chan struct{})
	if waitDone(done, 10*time.Millisecond) {
		t.Fatal("waitDone returned true for open channel")
	}
}
