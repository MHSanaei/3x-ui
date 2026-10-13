package service

import (
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const (
	// Per-poll read cap; a bigger backlog is skipped to its newest bytes.
	accessLogMaxReadBytes int64 = 4 << 20
	// First poll looks this far back so a panel restart keeps recent attribution.
	accessLogSeedBytes  int64 = 256 << 10
	accessLogSeedWindow       = 2 * time.Minute
	// The access-log path comes from parsing config.json, so cache it.
	accessLogPathTTL = 30 * time.Second
)

// parseAccessLogClientInbound reuses the log viewer's parser; ok is false for
// lines without both an inbound tag and a client email (api calls, anonymous).
func parseAccessLogClientInbound(line string) (email, tag string, ts time.Time, ok bool) {
	entry := parseAccessLogFields(line)
	if entry.Email == "" || entry.Inbound == "" || entry.Inbound == "api" {
		return "", "", time.Time{}, false
	}
	return entry.Email, entry.Inbound, entry.DateTime, true
}

// accessLogTailer incrementally reads new Xray access-log lines; it reports
// nothing when no access log is configured.
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

// poll returns email -> inbound tags from lines appended since the last call,
// and whether an access log is configured at all.
func (t *accessLogTailer) poll() (map[string][]string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	path := t.resolvePath(now)
	if path == "" {
		return nil, false
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, true
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, true
	}
	size := info.Size()
	if size < t.offset {
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
		return nil, true
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, true
	}
	buf, err := io.ReadAll(io.LimitReader(f, size-start))
	if err != nil {
		logger.Debug("access log tail read failed:", err)
		return nil, true
	}
	seeding := !t.seeded
	t.offset = start + int64(len(buf))
	t.seeded = true

	lines := strings.Split(t.partial+string(buf), "\n")
	t.partial = lines[len(lines)-1]
	lines = lines[:len(lines)-1]
	if seeding && start > 0 && len(lines) > 0 {
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
		return nil, true
	}
	return out, true
}

// RefreshLocalClientInbounds folds new access-log pairs into the online view;
// call it after RefreshLocalOnlineClients. Without an access log it clears the view.
func (s *InboundService) RefreshLocalClientInbounds() {
	process := currentXrayProcess()
	if process == nil {
		return
	}
	pairs, enabled := clientInboundTailer.poll()
	if !enabled {
		process.ClearLocalClientInbounds()
		return
	}
	process.RefreshLocalClientInbounds(pairs, time.Now().UnixMilli(), onlineGracePeriodMs)
}

// GetClientInboundsByGuid returns email -> attributed inbound tags per panelGuid.
// Only this panel's clients, and only with an access log; absence means unknown.
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
