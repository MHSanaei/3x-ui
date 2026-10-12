package service

import (
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const (
	// accessLogMaxReadBytes caps what one poll reads. A poll normally sees a few
	// KB; if the log grew past the cap (tailer stalled, huge burst) only the
	// newest bytes are scanned so the traffic job is never held up.
	accessLogMaxReadBytes int64 = 4 << 20
	// accessLogSeedBytes is how far back the very first poll looks, so a panel
	// restart still attributes clients that connected just before it.
	accessLogSeedBytes int64 = 256 << 10
	// accessLogSeedWindow ignores lines older than this on the first poll.
	accessLogSeedWindow = 2 * time.Minute
	// accessLogPathTTL is how long the resolved access-log path is cached; the
	// path comes from parsing config.json, which is too heavy for every poll.
	accessLogPathTTL = 30 * time.Second
)

var (
	accessLogInboundRe = regexp.MustCompile(`\[([^\s\]]+)\s*(?:>>|->)\s*[^\]]*\]`)
	accessLogEmailRe   = regexp.MustCompile(`\bemail:\s*(\S+)`)
)

// parseAccessLogClientInbound extracts the inbound tag and client email from one
// Xray access-log line, e.g.
//
//	2026/10/12 10:00:01.123456 from 1.2.3.4:5678 accepted tcp:example.com:443 [inbound-443 >> direct] email: alice
//
// Lines without both fields (api calls, dns, anonymous inbounds) report ok=false.
func parseAccessLogClientInbound(line string) (email, tag string, ts time.Time, ok bool) {
	if strings.Contains(line, "api -> api") {
		return "", "", time.Time{}, false
	}
	e := accessLogEmailRe.FindStringSubmatch(line)
	t := accessLogInboundRe.FindStringSubmatch(line)
	if e == nil || t == nil {
		return "", "", time.Time{}, false
	}
	fields := strings.Fields(line)
	if len(fields) > 1 {
		if parsed, err := time.ParseInLocation("2006/01/02 15:04:05.999999", fields[0]+" "+fields[1], time.Local); err == nil {
			ts = parsed
		}
	}
	return e[1], t[1], ts, true
}

// accessLogTailer incrementally reads new lines appended to the Xray access log
// and reports which inbound each client opened connections on. It is only active
// when an access log is configured; with "none" it returns nothing and the
// panel keeps using the traffic-delta heuristic.
type accessLogTailer struct {
	mu          sync.Mutex
	path        string
	pathChecked time.Time
	offset      int64
	seeded      bool
	partial     string
}

var clientInboundTailer = &accessLogTailer{}

func (t *accessLogTailer) resolvePath(now time.Time) string {
	if now.Sub(t.pathChecked) < accessLogPathTTL {
		return t.path
	}
	t.pathChecked = now
	path, err := xray.GetAccessLogPath()
	if err != nil || path == "none" {
		path = ""
	}
	if path != t.path {
		t.path = path
		t.offset = 0
		t.seeded = false
		t.partial = ""
	}
	return t.path
}

// poll returns the email -> inbound tags observed in log lines appended since
// the previous call. nil means nothing new (or no access log).
func (t *accessLogTailer) poll() map[string][]string {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	path := t.resolvePath(now)
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil
	}
	size := info.Size()
	if size < t.offset {
		// Truncated or rotated (clear_logs_job truncates it): start over.
		t.offset = 0
		t.partial = ""
	}

	start := t.offset
	if !t.seeded {
		start = max(size-accessLogSeedBytes, 0)
		t.partial = ""
	} else if size-start > accessLogMaxReadBytes {
		start = size - accessLogMaxReadBytes
		t.partial = ""
	}
	if size == start {
		t.offset = size
		t.seeded = true
		return nil
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil
	}
	buf, err := io.ReadAll(io.LimitReader(f, size-start))
	if err != nil {
		logger.Debug("access log tail read failed:", err)
		return nil
	}
	seeding := !t.seeded
	t.offset = start + int64(len(buf))
	t.seeded = true

	chunk := t.partial + string(buf)
	lines := strings.Split(chunk, "\n")
	// The last element is an unterminated line still being written.
	t.partial = lines[len(lines)-1]
	lines = lines[:len(lines)-1]
	if seeding && start > 0 && len(lines) > 0 {
		// Seeded mid-file: the first line is probably cut in half.
		lines = lines[1:]
	}

	out := make(map[string][]string)
	seen := make(map[string]struct{})
	for _, line := range lines {
		email, tag, ts, ok := parseAccessLogClientInbound(strings.TrimSpace(line))
		if !ok {
			continue
		}
		if seeding && !ts.IsZero() && now.Sub(ts) > accessLogSeedWindow {
			continue
		}
		key := email + "\x00" + tag
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out[email] = append(out[email], tag)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// RefreshLocalClientInbounds reads the access log lines appended since the last
// poll and folds the (client, inbound) pairs into the local online view. Call it
// after RefreshLocalOnlineClients in the same traffic tick. With no access log
// configured it only prunes, leaving the attribution map empty.
func (s *InboundService) RefreshLocalClientInbounds() {
	process := currentXrayProcess()
	if process == nil {
		return
	}
	process.RefreshLocalClientInbounds(clientInboundTailer.poll(), time.Now().UnixMilli(), onlineGracePeriodMs)
}

// GetClientInboundsByGuid returns, keyed by the hosting panel's GUID, which
// inbound tags each online client is attributed to (email -> tags). Only this
// panel's own clients are covered, and only when the Xray access log is enabled;
// a GUID or email absent from the result means "no attribution", and callers
// fall back to the coarser traffic-delta signal.
func (s *InboundService) GetClientInboundsByGuid() map[string]map[string][]string {
	out := map[string]map[string][]string{}
	process := currentXrayProcess()
	if process == nil {
		return out
	}
	local := process.GetLocalClientInbounds()
	if len(local) == 0 {
		return out
	}
	if guid := s.panelGuid(); guid != "" {
		out[guid] = local
	}
	return out
}
