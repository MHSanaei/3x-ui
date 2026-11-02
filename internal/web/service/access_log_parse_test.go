package service

import (
	"bufio"
	"errors"
	"fmt"
	"io"
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

// One proxied request with a URL past 64 KiB stopped the scanner and blanked the whole view.
func TestGetXrayLogsSurvivesOverlongLine(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", dir)
	accessLog := filepath.Join(dir, "access.log")
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"log":{"access":"`+filepath.ToSlash(accessLog)+`"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	lines := []string{
		"2024/01/02 15:04:05.000000 from 1.2.3.4:1 accepted tcp:one.example:443 [in-1 >> direct] email: alice",
		"2024/01/02 15:04:06.000000 from 1.2.3.4:2 accepted http://two.example/?" + strings.Repeat("a", 300000) + " [in-2 >> blocked] email: mallory",
		"2024/01/02 15:04:07.000000 from 1.2.3.4:3 accepted tcp:three.example:443 [in-1 >> direct] email: alice",
	}
	if err := os.WriteFile(accessLog, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	get := func(filter string) []LogEntry {
		return (&ServerService{}).GetXrayLogs("100", filter, "true", "true", "true", []string{"direct"}, []string{"blocked"})
	}
	entries := get("")
	if len(entries) != len(lines) {
		t.Fatalf("got %d entries for %d log lines", len(entries), len(lines))
	}
	if entries[0].ToAddress != "tcp:one.example:443" || entries[2].ToAddress != "tcp:three.example:443" {
		t.Errorf("neighbours of the long line were not parsed: %q, %q", entries[0].ToAddress, entries[2].ToAddress)
	}

	long := entries[1]
	if !strings.HasPrefix(long.ToAddress, "http://two.example/?a") || len(long.ToAddress) > maxAccessLogLineBytes+accessLogLineEndBytes {
		t.Errorf("long ToAddress: %d bytes starting %.30q", len(long.ToAddress), long.ToAddress)
	}
	if long.FromAddress != "1.2.3.4:2" || long.Inbound != "in-2" || long.Outbound != "blocked" || long.Email != "mallory" || long.Event != 1 {
		t.Errorf("long line lost its attribution: from %q, [%q >> %q], email %q, event %d", long.FromAddress, long.Inbound, long.Outbound, long.Email, long.Event)
	}
	if got := get("mallory"); len(got) != 1 {
		t.Errorf("filter by the long line's email matched %d entries, want 1", len(got))
	}
}

func TestReadAccessLogLine(t *testing.T) {
	const size, end = 64, accessLogLineEndBytes
	fits := strings.Repeat("f", size-1)
	head, middle, tail := strings.Repeat("h", size), strings.Repeat("m", 3*end), strings.Repeat("t", end)

	// the last line fills the buffer exactly and ends the input without a newline
	r := bufio.NewReaderSize(strings.NewReader(fits+"\n"+head+"x\r\n\n"+head+middle+tail+"\n"+head), size)
	for i, want := range []string{fits, head + "x", "", head + tail, head} {
		got, err := readAccessLogLine(r)
		if err != nil || got != want {
			t.Errorf("line %d: got %d bytes %.20q (err %v), want %d bytes %.20q", i, len(got), got, err, len(want), want)
		}
	}
	if _, err := readAccessLogLine(r); !errors.Is(err, io.EOF) {
		t.Errorf("after the last line: err = %v, want io.EOF", err)
	}
}
