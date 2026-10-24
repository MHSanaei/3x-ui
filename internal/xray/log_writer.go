package xray

import (
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// Compiled once at package load: Write runs on every line Xray emits, so
// recompiling these per write is wasted work.
var (
	crashStartRegex = regexp.MustCompile(`^(panic|fatal error): `)
	logLineRegex    = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{6}) \[([^\]]+)\] (.+)$`)
)

// NewLogWriter returns a new LogWriter for processing Xray log output.
func NewLogWriter() *LogWriter {
	return &LogWriter{}
}

// LogWriter splits Xray's stdout/stderr into lines and forwards them to the
// panel log; once a Go panic or fatal error starts, all output is kept for one crash report.
type LogWriter struct {
	mu       sync.RWMutex
	pending  string
	crash    strings.Builder
	lastLine string
}

// LastLine returns the most recently processed Xray log line. It is safe for
// concurrent use: Process.GetResult reads it from a different goroutine than the
// one Xray drives Write from.
func (lw *LogWriter) LastLine() string {
	lw.mu.RLock()
	defer lw.mu.RUnlock()
	return lw.lastLine
}

// Write buffers chunks into whole lines: the pipe splits output at arbitrary
// points, e.g. the Go runtime writes "panic: " on its own before the message.
func (lw *LogWriter) Write(m []byte) (n int, err error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	if lw.crash.Len() > 0 {
		lw.crash.Write(m)
		return len(m), nil
	}
	lw.pending += string(m)
	for {
		i := strings.IndexByte(lw.pending, '\n')
		if i < 0 {
			break
		}
		line := lw.pending[:i]
		lw.pending = lw.pending[i+1:]
		if lw.startCrashLocked(line + "\n" + lw.pending) {
			lw.pending = ""
			break
		}
		lw.processLineLocked(line)
	}
	return len(m), nil
}

// Flush handles an unterminated final line and writes the crash report, if any;
// it must run after cmd.Wait, which guarantees every output chunk was delivered.
func (lw *LogWriter) Flush() {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	if rest := lw.pending; rest != "" {
		lw.pending = ""
		if !lw.startCrashLocked(rest) {
			lw.processLineLocked(rest)
		}
	}
	if lw.crash.Len() == 0 {
		return
	}
	report := lw.crash.String()
	lw.crash.Reset()
	logger.Debug("Core crash detected:\n", report)
	if err := writeCrashReport([]byte(report)); err != nil {
		logger.Error("Unable to write crash report:", err)
	}
}

// startCrashLocked begins capturing a crash when output opens with a Go runtime
// crash header; the header line becomes LastLine so GetResult shows the cause.
func (lw *LogWriter) startCrashLocked(output string) bool {
	if lw.crash.Len() > 0 || !crashStartRegex.MatchString(output) {
		return false
	}
	header, _, _ := strings.Cut(output, "\n")
	lw.lastLine = strings.TrimSpace(header)
	lw.crash.WriteString(output)
	return true
}

func (lw *LogWriter) processLineLocked(line string) {
	msg := strings.TrimSpace(line)
	if msg == "" {
		return
	}
	msgLower := strings.ToLower(msg)

	// Suppress noisy Windows process-kill signal that surfaces as exit status 1
	if runtime.GOOS == "windows" && strings.Contains(msgLower, "exit status 1") {
		return
	}

	matches := logLineRegex.FindStringSubmatch(msg)
	if len(matches) > 3 {
		level := matches[2]
		msgBody := matches[3]
		msgBodyLower := strings.ToLower(msgBody)

		if strings.Contains(msgBodyLower, "tls handshake error") ||
			strings.Contains(msgBodyLower, "connection ends") {
			logger.Debug("XRAY: " + msgBody)
			lw.lastLine = ""
			return
		}

		if strings.Contains(msgBodyLower, "failed") {
			logger.Error("XRAY: " + msgBody)
		} else {
			switch level {
			case "Debug":
				logger.Debug("XRAY: " + msgBody)
			case "Info":
				logger.Info("XRAY: " + msgBody)
			case "Warning":
				logger.Warning("XRAY: " + msgBody)
			case "Error":
				logger.Error("XRAY: " + msgBody)
			default:
				logger.Debug("XRAY: " + msg)
			}
		}
		lw.lastLine = ""
		return
	}

	if strings.Contains(msgLower, "tls handshake error") ||
		strings.Contains(msgLower, "connection ends") {
		logger.Debug("XRAY: " + msg)
		lw.lastLine = msg
		return
	}

	if strings.Contains(msgLower, "failed") {
		logger.Error("XRAY: " + msg)
	} else {
		logger.Debug("XRAY: " + msg)
	}
	lw.lastLine = msg
}
