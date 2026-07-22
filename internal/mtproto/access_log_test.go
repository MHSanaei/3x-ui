package mtproto

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendAccessLog(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "access.log")
	event := AccessEvent{
		Timestamp:   time.Date(2026, time.July, 22, 9, 41, 9, 123456000, time.Local),
		FromAddress: "203.0.113.7:54321",
		ToAddress:   "149.154.167.50:443",
		Inbound:     "mtproto-in",
		Outbound:    "proxy-a",
		Email:       "alice@example.com",
	}
	if err := AppendAccessLog(path, []AccessEvent{event}); err != nil {
		t.Fatalf("append access event: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read access log: %v", err)
	}
	want := "2026/07/22 09:41:09.123456 from 203.0.113.7:54321 accepted tcp:149.154.167.50:443 [mtproto-in >> proxy-a] email: alice@example.com\n"
	if string(data) != want {
		t.Fatalf("access log = %q, want %q", data, want)
	}

	event.Email = "mallory\nforged"
	if line := formatAccessLogLine(event); strings.Contains(line, "\n") || !strings.Contains(line, "mallory_forged") {
		t.Fatalf("access fields must not inject lines: %q", line)
	}

	event.FromAddress = "[2001:db8::7]:54321"
	if line := formatAccessLogLine(event); !strings.Contains(line, "from [2001:db8::7]:54321") {
		t.Fatalf("IPv6 client address must be bracketed: %q", line)
	}
}
