package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseAccessLogClientInbound(t *testing.T) {
	cases := []struct {
		name      string
		line      string
		wantOK    bool
		wantEmail string
		wantTag   string
	}{
		{"direct", "2026/10/12 10:00:01.123456 from 1.2.3.4:5678 accepted tcp:example.com:443 [inbound-443 >> direct] email: alice", true, "alice", "inbound-443"},
		{"routed arrow", "2026/10/12 10:00:01.123456 from 1.2.3.4:5678 accepted udp:1.1.1.1:53 [in-8443 -> proxy] email: bob@example.com", true, "bob@example.com", "in-8443"},
		{"api call", "2026/10/12 10:00:01 from 127.0.0.1:1 accepted tcp:127.0.0.1:62789 [api -> api]", false, "", ""},
		{"no email", "2026/10/12 10:00:01 from 1.2.3.4:5 accepted tcp:a.com:80 [in-1 >> direct]", false, "", ""},
		{"garbage", "not a log line", false, "", ""},
	}
	for _, c := range cases {
		email, tag, _, ok := parseAccessLogClientInbound(c.line)
		if ok != c.wantOK || email != c.wantEmail || tag != c.wantTag {
			t.Errorf("%s: got (%q,%q,%v), want (%q,%q,%v)", c.name, email, tag, ok, c.wantEmail, c.wantTag, c.wantOK)
		}
	}
}

func TestAccessLogTailerReadsOnlyNewLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "access.log")
	line := func(email, tag string) string {
		return time.Now().Format("2006/01/02 15:04:05.000000") + " from 1.2.3.4:5 accepted tcp:a.com:443 [" + tag + " >> direct] email: " + email + "\n"
	}
	if err := os.WriteFile(path, []byte(line("alice", "in-1")), 0o600); err != nil {
		t.Fatal(err)
	}
	tail := &accessLogTailer{path: path, pathChecked: time.Now()}

	if got := tail.poll(); len(got["alice"]) != 1 || got["alice"][0] != "in-1" {
		t.Fatalf("seed poll: got %v", got)
	}
	if got := tail.poll(); got != nil {
		t.Fatalf("nothing new, got %v", got)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	// A complete line plus a half-written one: only the complete one counts now.
	half := line("carol", "in-3")
	if _, err := f.WriteString(line("bob", "in-2") + half[:20]); err != nil {
		t.Fatal(err)
	}
	got := tail.poll()
	if len(got) != 1 || got["bob"][0] != "in-2" {
		t.Fatalf("append poll: got %v", got)
	}
	if _, err := f.WriteString(half[20:]); err != nil {
		t.Fatal(err)
	}
	if got := tail.poll(); got["carol"][0] != "in-3" {
		t.Fatalf("completed line should be picked up, got %v", got)
	}

	// Truncation (clear_logs_job) restarts from the top.
	if err := os.WriteFile(path, []byte(line("dave", "in-4")), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := tail.poll(); got["dave"][0] != "in-4" {
		t.Fatalf("after truncation: got %v", got)
	}
}
