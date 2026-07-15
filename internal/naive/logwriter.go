package naive

import (
	"strings"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

type LogWriter struct {
	tag      string
	maxLines int
	mu       sync.RWMutex
	lines    []string
}

func NewLogWriter(tag string) *LogWriter {
	return &LogWriter{tag: tag, maxLines: 1000}
}

func (w *LogWriter) Write(data []byte) (int, error) {
	text := strings.TrimSpace(string(data))
	if text == "" {
		return len(data), nil
	}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		prefixed := "[naive/" + w.tag + "] " + trimmed
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "error") || strings.Contains(lower, "fail") {
			logger.Error(prefixed)
		} else {
			logger.Info(prefixed)
		}
		w.mu.Lock()
		w.lines = append(w.lines, prefixed)
		if len(w.lines) > w.maxLines {
			w.lines = append([]string(nil), w.lines[len(w.lines)-w.maxLines:]...)
		}
		w.mu.Unlock()
	}
	return len(data), nil
}

func (w *LogWriter) GetLogs(rows int) []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if rows <= 0 || len(w.lines) <= rows {
		return append([]string(nil), w.lines...)
	}
	return append([]string(nil), w.lines[len(w.lines)-rows:]...)
}
