package logview

import (
	"encoding/json"
	"maps"
	"net"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	LevelError   = "error"
	LevelWarning = "warning"
	LevelInfo    = "info"
	LevelDebug   = "debug"
	// LevelUnknown selects lines with no recognisable severity; such lines carry an empty Level.
	LevelUnknown = "unknown"

	statusAccepted = "accepted"
	statusRejected = "rejected"
)

var combinedLogRe = regexp.MustCompile(`^(\S+) \S+ (\S+) \[([^\]]+)\] "([^"]*)" (\d{3}) \S+`)

// parseLine extracts what it can from one log line. Lines are attacker-influenced
// and may be cut mid-token, so anything unrecognised stays in Message and Raw only.
func parseLine(raw string) LogLine {
	l := LogLine{Raw: raw}
	text := strings.TrimSpace(raw)
	if strings.HasPrefix(text, "{") && parseJSONLine(text, &l) {
		return l
	}
	if parseCombinedLine(text, &l) {
		return l
	}
	if ts, rest, ok := leadingTime(text); ok {
		l.Time = ts.UnixMilli()
		text = rest
	}
	text = takeLevel(text, &l)
	l.Message = text
	l.Category = leadingCategory(text)
	scanTokens(text, &l)
	if name := proxyProtocol(text); name != "" {
		l.Protocol = name
	}
	if l.Level == "" {
		switch l.Status {
		case statusAccepted:
			l.Level = LevelInfo
		case statusRejected:
			l.Level = LevelWarning
		}
	}
	return l
}

func parseJSONLine(text string, l *LogLine) bool {
	var fields map[string]any
	if err := json.Unmarshal([]byte(text), &fields); err != nil {
		return false
	}
	// Sorted so synonyms like host/sni resolve the same way on every read of a line.
	for _, key := range slices.Sorted(maps.Keys(fields)) {
		value := ""
		switch v := fields[key].(type) {
		case string:
			value = v
		case float64:
			value = strconv.FormatFloat(v, 'f', -1, 64)
		default:
			continue
		}
		switch strings.ToLower(key) {
		case "msg", "message":
			if l.Message == "" {
				l.Message = value
			}
		default:
			setField(key, value, l)
		}
	}
	if l.Message == "" {
		l.Message = text
	}
	return true
}

func parseCombinedLine(text string, l *LogLine) bool {
	if !strings.Contains(text, `] "`) {
		return false
	}
	m := combinedLogRe.FindStringSubmatch(text)
	if m == nil {
		return false
	}
	ts, err := time.Parse("02/Jan/2006:15:04:05 -0700", m[3])
	if err != nil {
		return false
	}
	l.Time = ts.UnixMilli()
	setIP(m[1], l)
	if m[2] != "-" {
		l.User = m[2]
	}
	l.Message = m[4]
	l.Status = m[5]
	if strings.Contains(m[4], " HTTP/") {
		l.Protocol = "http"
	}
	switch m[5][0] {
	case '5':
		l.Level = LevelError
	case '4':
		l.Level = LevelWarning
	default:
		l.Level = LevelInfo
	}
	return true
}

func cutField(s string) (string, string) {
	s = strings.TrimLeft(s, " \t")
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i], strings.TrimLeft(s[i:], " \t")
	}
	return s, ""
}

func leadingTime(text string) (time.Time, string, bool) {
	if strings.HasPrefix(text, "[") {
		if end := strings.IndexByte(text, ']'); end > 0 {
			if ts, ok := parseStamp(text[1:end]); ok {
				return ts, strings.TrimLeft(text[end+1:], " \t"), true
			}
		}
		return time.Time{}, "", false
	}
	first, rest := cutField(text)
	if len(first) == 10 && (first[4] == '/' || first[4] == '-') {
		second, after := cutField(rest)
		if ts, ok := parseStamp(first + " " + second); ok {
			return ts, after, true
		}
		return time.Time{}, "", false
	}
	if ts, ok := parseStamp(first); ok {
		return ts, rest, true
	}
	if len(first) == 3 {
		day, afterDay := cutField(rest)
		clock, after := cutField(afterDay)
		if ts, err := time.ParseInLocation("Jan 2 15:04:05", first+" "+day+" "+clock, time.Local); err == nil {
			return syslogYear(ts, time.Now()), after, true
		}
	}
	return time.Time{}, "", false
}

// parseStamp relies on time.Parse accepting a fractional second the layout omits.
func parseStamp(s string) (time.Time, bool) {
	if len(s) < 19 {
		return time.Time{}, false
	}
	var ts time.Time
	var err error
	switch {
	case s[4] == '/':
		ts, err = time.ParseInLocation("2006/01/02 15:04:05", s, time.Local)
	case s[4] == '-' && s[10] == 'T':
		if ts, err = time.Parse(time.RFC3339, s); err != nil {
			ts, err = time.ParseInLocation("2006-01-02T15:04:05", s, time.Local)
		}
	case s[4] == '-':
		ts, err = time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
	default:
		return time.Time{}, false
	}
	return ts, err == nil
}

// syslogYear dates a yearless stamp; a day of slack tolerates clock skew between writer and reader.
func syslogYear(stamp, now time.Time) time.Time {
	ts := time.Date(now.Year(), stamp.Month(), stamp.Day(), stamp.Hour(), stamp.Minute(), stamp.Second(), 0, stamp.Location())
	if ts.After(now.Add(24 * time.Hour)) {
		return ts.AddDate(-1, 0, 0)
	}
	return ts
}

func normalizeLevel(word string) string {
	switch strings.ToLower(word) {
	case "debug", "trace":
		return LevelDebug
	case "info", "notice":
		return LevelInfo
	case "warn", "warning":
		return LevelWarning
	case "err", "error", "fatal", "crit", "critical", "panic", "emerg", "alert":
		return LevelError
	}
	return ""
}

// takeLevel only looks at the leading word: "error" later in a sentence is prose, not a severity.
func takeLevel(text string, l *LogLine) string {
	first, rest := cutField(text)
	level := normalizeLevel(strings.Trim(first, "[]():<>|"))
	if level == "" {
		return text
	}
	l.Level = level
	rest = strings.TrimPrefix(rest, "- ")
	if id, after := cutField(rest); isSessionID(id) {
		return after
	}
	return rest
}

func isSessionID(token string) bool {
	if len(token) < 3 || token[0] != '[' || token[len(token)-1] != ']' {
		return false
	}
	_, err := strconv.ParseUint(token[1:len(token)-1], 10, 64)
	return err == nil
}

// leadingCategory reads a panel "[TAG]" or an Xray "app/dns:" component off the start of the text.
func leadingCategory(text string) string {
	first, _ := cutField(text)
	if len(first) > 2 && first[0] == '[' && first[len(first)-1] == ']' {
		tag := first[1 : len(first)-1]
		if strings.ToUpper(tag) == tag {
			return tag
		}
		return ""
	}
	if strings.HasSuffix(first, ":") && strings.Contains(first, "/") {
		return first[:len(first)-1]
	}
	return ""
}

func proxyProtocol(text string) string {
	i := strings.Index(text, "proxy/")
	if i < 0 || (i > 0 && text[i-1] != ' ') {
		return ""
	}
	name := text[i+len("proxy/"):]
	if end := strings.IndexAny(name, "/: "); end >= 0 {
		name = name[:end]
	}
	return strings.ToLower(name)
}

func scanTokens(text string, l *LogLine) {
	tokens := strings.Fields(text)
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		next := ""
		if i+1 < len(tokens) {
			next = tokens[i+1]
		}
		switch {
		case tok == "from" && next != "":
			i++
			if setIP(next, l) && i+1 < len(tokens) && (tokens[i+1] == statusAccepted || tokens[i+1] == statusRejected) {
				l.Status = tokens[i+1]
				i++
			}
		case (tok == "email:" && next != "") || (tok == "Client" && strings.HasSuffix(next, ":")):
			setOnce(&l.User, trimValue(next))
			i++
		case next == "=" && i+2 < len(tokens):
			setField(tok, tokens[i+2], l)
			i += 2
		case strings.HasPrefix(tok, "[") && (next == ">>" || next == "->") && i+2 < len(tokens) && strings.HasSuffix(tokens[i+2], "]"):
			setOnce(&l.Inbound, tok[1:])
			setOnce(&l.Outbound, strings.TrimSuffix(tokens[i+2], "]"))
			i += 2
		case strings.HasPrefix(tok, "tcp:") || strings.HasPrefix(tok, "udp:"):
			setOnce(&l.Protocol, tok[:3])
			setOnce(&l.Host, addrHost(tok))
		default:
			if key, value, ok := strings.Cut(tok, "="); ok && key != "" {
				setField(key, value, l)
			} else if l.IP == "" {
				if ip := tokenIP(tok); ip != "" {
					l.IP = ip
				}
			}
		}
	}
}

func setField(key, value string, l *LogLine) {
	value = trimValue(value)
	if value == "" {
		return
	}
	switch strings.ToLower(key) {
	case "ip", "client_ip", "clientip", "remote_ip", "remoteip", "src_ip", "srcip", "source_ip", "remote_addr", "remoteaddr", "from":
		setIP(value, l)
	case "user", "username", "email", "client_email":
		setOnce(&l.User, value)
	case "host", "domain", "sni", "server_name", "target", "dest", "destination":
		setOnce(&l.Host, addrHost(value))
	case "protocol", "proto", "network":
		setOnce(&l.Protocol, strings.ToLower(value))
	case "inbound", "inbound_tag", "inboundtag", "tag":
		setOnce(&l.Inbound, value)
	case "outbound", "outbound_tag", "outboundtag":
		setOnce(&l.Outbound, value)
	case "status", "status_code", "statuscode", "code":
		setOnce(&l.Status, value)
	case "level", "lvl", "severity":
		if l.Level == "" {
			l.Level = normalizeLevel(value)
		}
	case "time", "ts", "timestamp", "@timestamp":
		if l.Time == 0 {
			l.Time = parseTimeValue(value)
		}
	}
}

func setOnce(field *string, value string) {
	if *field == "" {
		*field = value
	}
}

func trimValue(value string) string {
	return strings.Trim(value, `"',;:`)
}

func parseTimeValue(value string) int64 {
	if n, err := strconv.ParseInt(value, 10, 64); err == nil {
		switch {
		case n <= 0:
			return 0
		case n >= 1e12:
			return n
		default:
			return n * 1000
		}
	}
	if ts, ok := parseStamp(value); ok {
		return ts.UnixMilli()
	}
	return 0
}

// addrHost strips a network prefix and port: "tcp:[::1]:443" and "tcp:a.example:443" reduce to the host.
func addrHost(addr string) string {
	addr = strings.TrimLeft(addr, "/")
	if strings.HasPrefix(addr, "tcp:") || strings.HasPrefix(addr, "udp:") {
		addr = addr[len("tcp:"):]
	}
	addr = strings.TrimRight(addr, ",;")
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return strings.Trim(addr, "[]")
}

// setIP accepts Xray's masked addresses ("1.2.*.*") as well, since those are still a client's identity.
func setIP(addr string, l *LogLine) bool {
	host := addrHost(addr)
	_, err := netip.ParseAddr(host)
	masked := strings.Contains(host, "*") && strings.ContainsAny(host, ".:")
	if err != nil && !masked {
		return false
	}
	setOnce(&l.IP, host)
	return true
}

func tokenIP(token string) string {
	if !strings.ContainsAny(token, ".:") {
		return ""
	}
	host := addrHost(strings.Trim(token, `"'(),;:`))
	addr, err := netip.ParseAddr(host)
	if err != nil || addr.IsUnspecified() {
		return ""
	}
	return host
}
