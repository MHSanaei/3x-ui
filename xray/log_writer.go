package xray

import (
	"regexp"
	"strings"

	"x-ui/logger"
)

func NewLogWriter() *LogWriter {
	return &LogWriter{}
}

type LogWriter struct {
	lastLine string
}

func (lw *LogWriter) Write(m []byte) (n int, err error) {
	crashRegex := regexp.MustCompile(`(?i)(panic|exception|stack trace|fatal error)`)

	// Convert the data to a string
	message := strings.TrimSpace(string(m))

	// Check if the message contains a crash
	if crashRegex.MatchString(message) {
		logger.Debug("Core crash detected:\n", message)
		lw.lastLine = message
		err1 := writeCrachReport(m)
		if err1 != nil {
			logger.Error("Unable to write crash report:", err1)
		}
		return len(m), nil
	}

	regex := regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{6}) \[([^\]]+)\] (.+)$`)
	messages := strings.SplitSeq(message, "\n")

	for msg := range messages {
		matches := regex.FindStringSubmatch(msg)

		if len(matches) > 3 {
			level := matches[2]
			msgBody := matches[3]
			msgBodyLower := strings.ToLower(msgBody)

			if strings.Contains(msgBodyLower, "tls handshake error") ||
				strings.Contains(msgBodyLower, "connection ends") {
				logger.Debug("XRAY: " + msgBody)
				lw.lastLine = ""
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
			lw.lastLine = ""
		} else if msg != "" {
			msgLower := strings.ToLower(msg)

			if strings.Contains(msgLower, "tls handshake error") ||
				strings.Contains(msgLower, "connection ends") {
				logger.Debug("XRAY: " + msg)
				lw.lastLine = msg
				continue
			}

			if strings.Contains(msgLower, "failed") {
				logger.Error("XRAY: " + msg)
			} else {
				logger.Debug("XRAY: " + msg)
			}
			lw.lastLine = msg
		}
	}

	return len(m), nil
}
