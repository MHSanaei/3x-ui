package xray

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
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

	for range writers {
		go func() {
			defer wg.Done()
			for range iterations {
				_, _ = lw.Write([]byte("2024/01/01 00:00:00.000000 [Info] connection accepted\n"))
			}
		}()
	}
	for range readers {
		go func() {
			defer wg.Done()
			for range iterations {
				_ = lw.LastLine()
			}
		}()
	}
	wg.Wait()
}

func crashReportsIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var contents []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), crashReportPrefix) {
			b, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			contents = append(contents, string(b))
		}
	}
	return contents
}

// The Go runtime prints "panic: " with its own write(2), so the pipe hands it
// over as a chunk of its own; the report must still hold the whole traceback.
func TestLogWriterCrashReportHoldsWholePanic(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XUI_LOG_FOLDER", dir)
	chunks := []string{
		"panic: ",
		"runtime error: index out of range [3] with length 1\n",
		"\ngoroutine 1 [running]:\nmain.main()\n\t/src/main.go:5 +0x1d\n",
		"exit status 2",
	}
	lw := NewLogWriter()
	for _, c := range chunks {
		_, _ = lw.Write([]byte(c))
	}
	lw.Flush()

	got := crashReportsIn(t, dir)
	want := strings.Join(chunks, "")
	if len(got) != 1 || got[0] != want {
		t.Fatalf("crash reports = %q, want exactly [%q]", got, want)
	}
	if line := lw.LastLine(); line != "panic: runtime error: index out of range [3] with length 1" {
		t.Fatalf("LastLine = %q", line)
	}
}

func TestLogWriterOrdinaryLineMentioningPanicIsNotACrash(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XUI_LOG_FOLDER", dir)
	lw := NewLogWriter()
	_, _ = lw.Write([]byte("2026/07/12 15:16:00.000000 [Info] [42] proxy/vless: sniffed domain panic.example.com\n"))
	lw.Flush()

	if got := crashReportsIn(t, dir); len(got) != 0 {
		t.Fatalf("ordinary log line produced crash reports %q", got)
	}
}

func TestWriteCrashReportKeepsReportsFromTheSameSecond(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XUI_LOG_FOLDER", dir)
	for _, m := range []string{"panic: first", "panic: second"} {
		if err := writeCrashReport([]byte(m)); err != nil {
			t.Fatal(err)
		}
	}
	got := crashReportsIn(t, dir)
	slices.Sort(got)
	if !slices.Equal(got, []string{"panic: first", "panic: second"}) {
		t.Fatalf("crash reports = %q, want both", got)
	}
}
