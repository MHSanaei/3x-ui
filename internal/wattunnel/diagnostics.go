package wattunnel

import (
	"strings"
	"sync"
)

type processDiagnostics struct {
	mu     sync.Mutex
	tail   string
	reason string
}

func (d *processDiagnostics) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.tail += string(p)
	if len(d.tail) > 4096 {
		d.tail = d.tail[len(d.tail)-4096:]
	}
	lower := strings.ToLower(d.tail)
	for _, rule := range []struct{ match, reason string }{
		{"address already in use", "listen address already in use"},
		{"only one usage of each socket address", "listen address already in use"},
		{"permission denied", "insufficient socket or file permissions"},
		{"operation not permitted", "missing operating-system privileges"},
		{"no such file", "required file is missing"},
		{"failed to find any pem data", "invalid TLS certificate or key"},
	} {
		if strings.Contains(lower, rule.match) {
			d.reason = rule.reason
			break
		}
	}
	return len(p), nil
}

func (d *processDiagnostics) Reason() string {
	if d == nil {
		return ""
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.reason
}
