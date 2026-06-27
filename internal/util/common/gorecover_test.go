package common

import (
	"os"
	"testing"
	"time"

	"github.com/op/go-logging"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func TestMain(m *testing.M) {
	logger.InitLogger(logging.ERROR)
	os.Exit(m.Run())
}

func TestGoRecover_RunsFn(t *testing.T) {
	done := make(chan struct{})
	GoRecover("test-run", func() { close(done) })
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("fn did not run")
	}
}

func TestGoRecover_RecoversPanic(t *testing.T) {
	done := make(chan struct{})
	// If GoRecover did not recover, this panic would crash the test binary.
	GoRecover("test-panic", func() {
		defer close(done)
		panic("boom")
	})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine did not complete")
	}
	// Let the deferred recover+log run before the test ends.
	time.Sleep(50 * time.Millisecond)
}
