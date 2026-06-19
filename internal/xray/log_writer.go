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
	crashRegex   = regexp.MustCompile(`(?i)(panic|exception|stack trace|fatal error)`)
	logLineRegex = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{6}) \[([^\]]+)\] (.+)$`)
)

// NewLogWriter returns a new LogWriter for processing Xray log output.
func NewLogWriter() *LogWriter {
	return &LogWriter{}
}

// LogWriter processes and filters log output from the Xray process, handling crash detection and message filtering.
type LogWriter struct {
	mu       sync.RWMutex
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

func (lw *LogWriter) setLastLine(line string) {
	lw.mu.Lock()
	lw.lastLine = line
	lw.mu.Unlock()
}

// Write processes and filters log output from the Xray process, handling crash detection and message filtering.
func (lw *LogWriter) Write(m []byte) (n int, err error) {
	// Convert the data to a string
	message := strings.TrimSpace(string(m))
	msgLowerAll := strings.ToLower(message)

	// Suppress noisy Windows process-kill signal that surfaces as exit status 1
	if runtime.GOOS == "windows" && strings.Contains(msgLowerAll, "exit status 1") {
		return len(m), nil
	}

	// Check if the message contains a crash
	if crashRegex.MatchString(message) {
		logger.Debug("Core crash detected:\n", message)
		lw.setLastLine(message)
		err1 := writeCrashReport(m)
		if err1 != nil {
			logger.Error("Unable to write crash report:", err1)
		}
		return len(m), nil
	}

	messages := strings.SplitSeq(message, "\n")

	for msg := range messages {
		matches := logLineRegex.FindStringSubmatch(msg)

		if len(matches) > 3 {
			level := matches[2]
			msgBody := matches[3]
			msgBodyLower := strings.ToLower(msgBody)

			if strings.Contains(msgBodyLower, "tls handshake error") ||
				strings.Contains(msgBodyLower, "connection ends") {
				logger.Debug("XRAY: " + msgBody)
				lw.setLastLine("")
				continue
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
			lw.setLastLine("")
		} else if msg != "" {
			msgLower := strings.ToLower(msg)

			if strings.Contains(msgLower, "tls handshake error") ||
				strings.Contains(msgLower, "connection ends") {
				logger.Debug("XRAY: " + msg)
				lw.setLastLine(msg)
				continue
			}

			if strings.Contains(msgLower, "failed") {
				logger.Error("XRAY: " + msg)
			} else {
				logger.Debug("XRAY: " + msg)
			}
			lw.setLastLine(msg)
		}
	}

	return len(m), nil
}
