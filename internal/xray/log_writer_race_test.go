package xray

import (
	"sync"
	"testing"
)

// TestLogWriterLastLineConcurrent exercises the LogWriter from multiple
// goroutines: Xray drives Write while another goroutine (Process.GetResult)
// reads the last line. Run under `go test -race` this fails on an unguarded
// lastLine field and passes once the access is serialized.
func TestLogWriterLastLineConcurrent(t *testing.T) {
	lw := NewLogWriter()
	const writers, readers, iterations = 4, 4, 500

	var wg sync.WaitGroup
	wg.Add(writers + readers)

	for i := 0; i < writers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _ = lw.Write([]byte("2024/01/01 00:00:00.000000 [Info] connection accepted"))
			}
		}()
	}
	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = lw.LastLine()
			}
		}()
	}
	wg.Wait()
}
