package service

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

func TestParseAccessLogFields(t *testing.T) {
	malformed := []string{
		"",
		"singletoken",
		"2024/01/02",
		"2024/01/02 15:04:05.000000 from",
		"2024/01/02 15:04:05.000000 accepted",
		"2024/01/02 15:04:05.000000 email:",
	}
	for _, line := range malformed {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("parseAccessLogFields panicked on %q: %v", line, r)
				}
			}()
			_ = parseAccessLogFields(line)
		}()
	}

	line := "2024/01/02 15:04:05.123456 from 1.2.3.4:555 accepted tcp:example.com:443 [inbound-tag >> outbound-tag] email: alice@example.com"
	entry := parseAccessLogFields(line)
	if entry.FromAddress != "1.2.3.4:555" {
		t.Errorf("FromAddress = %q, want %q", entry.FromAddress, "1.2.3.4:555")
	}
	if entry.ToAddress != "tcp:example.com:443" {
		t.Errorf("ToAddress = %q, want %q", entry.ToAddress, "tcp:example.com:443")
	}
	if entry.Inbound != "inbound-tag" {
		t.Errorf("Inbound = %q, want %q", entry.Inbound, "inbound-tag")
	}
	if entry.Outbound != "outbound-tag" {
		t.Errorf("Outbound = %q, want %q", entry.Outbound, "outbound-tag")
	}
	if entry.Email != "alice@example.com" {
		t.Errorf("Email = %q, want %q", entry.Email, "alice@example.com")
	}
	if entry.DateTime.IsZero() {
		t.Error("DateTime was not parsed from a well-formed line")
	}
}

// writeAccessLog points Xray's config at a fresh access log holding the lines
// that line(i) yields for i in [0, n).
func writeAccessLog(t *testing.T, n int, line func(i int) string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", dir)
	accessLog := filepath.Join(dir, "access.log")
	if err := os.WriteFile(filepath.Join(dir, "config.json"), fmt.Appendf(nil, `{"log":{"access":%q}}`, accessLog), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(accessLog)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	for i := range n {
		_, _ = writer.WriteString(line(i) + "\n")
	}
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
}

func readAccessLog(count string) []LogEntry {
	return (&ServerService{}).GetXrayLogs(count, "", "true", "true", "true", []string{"direct"}, []string{"blocked"})
}

func TestGetXrayLogsReturnsTheNewestEntriesInFileOrder(t *testing.T) {
	writeAccessLog(t, 250, func(i int) string {
		return fmt.Sprintf("2024/01/02 15:04:05.000000 from 1.2.3.4:%d accepted tcp:example.com:443 [in-1 >> direct]", i)
	})

	tests := []struct {
		count string
		want  int
	}{
		{"1", 1},
		{"20", 20},
		{"250", 250},
		{"1000", 250},
		{"0", 100},
		{"-1", 100},
		{"10001", 100},
		{"not-a-number", 100},
	}
	for _, tt := range tests {
		entries := readAccessLog(tt.count)
		if len(entries) != tt.want {
			t.Errorf("count %q: got %d entries, want %d", tt.count, len(entries), tt.want)
			continue
		}
		for i, entry := range entries {
			if want := fmt.Sprintf("1.2.3.4:%d", 250-tt.want+i); entry.FromAddress != want {
				t.Errorf("count %q: entry %d is from %q, want %q", tt.count, i, entry.FromAddress, want)
				break
			}
		}
	}
}

// The file grows with every request Xray refuses, so what a read of it holds in
// memory must follow the requested count, not the size of the file.
func TestGetXrayLogsMemoryDoesNotGrowWithTheFile(t *testing.T) {
	line := "2024/01/02 15:04:05.000000 from 1.2.3.4:1 accepted http://example.com/?" + strings.Repeat("a", 16<<10) + " [in-1 >> direct]"
	writeAccessLog(t, (64<<20)/len(line), func(int) string { return line })

	// A low GC target keeps garbage from the scan itself out of the measurement.
	defer debug.SetGCPercent(debug.SetGCPercent(10))
	heap := func() uint64 {
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		return stats.HeapAlloc
	}
	runtime.GC()
	before, peak := heap(), uint64(0)

	done := make(chan []LogEntry)
	go func() { done <- readAccessLog("20") }()
	for {
		select {
		case entries := <-done:
			if len(entries) != 20 {
				t.Fatalf("got %d entries, want 20", len(entries))
			}
			if grown := int64(peak) - int64(before); grown > 32<<20 {
				t.Fatalf("heap grew by %d MiB while 20 lines of a 64 MiB log were read", grown>>20)
			}
			return
		case <-time.After(time.Millisecond):
			peak = max(peak, heap())
		}
	}
}
