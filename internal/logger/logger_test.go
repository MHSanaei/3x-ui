package logger

import (
	"fmt"
	"sync"
	"testing"

	golog "github.com/op/go-logging"
)

// TestGetLogs_ReturnsAtMostC guards the documented "up to c entries" contract.
// The loop condition must cap output at c (ERROR entries are queried at "debug"
// level so the level filter passes all of them, isolating the count).
func TestGetLogs_ReturnsAtMostC(t *testing.T) {
	logBufferMu.Lock()
	logBuffer = nil
	logBufferMu.Unlock()
	for i := range 5 {
		addToBuffer("ERROR", fmt.Sprintf("m%d", i))
	}

	cases := []struct{ c, want int }{
		{0, 0},
		{2, 2},
		{5, 5},
		{10, 5}, // capped at what's available
	}
	for _, tc := range cases {
		if got := GetLogs(tc.c, "debug"); len(got) != tc.want {
			t.Errorf("GetLogs(%d) returned %d entries, want %d", tc.c, len(got), tc.want)
		}
	}
}

// InitLogger replaces the package logger while other goroutines are already
// logging — CI caught that as a data race between InitLogger and Warningf.
func TestInitLoggerConcurrentWithLogging(t *testing.T) {
	t.Setenv("XUI_LOG_FOLDER", t.TempDir())

	stop := make(chan struct{})
	var logging sync.WaitGroup
	logging.Add(1)
	go func() {
		defer logging.Done()
		for {
			select {
			case <-stop:
				return
			default:
				Warningf("concurrent %s", "log")
			}
		}
	}()

	for range 10 {
		InitLogger(golog.CRITICAL)
	}
	close(stop)
	logging.Wait()
}
