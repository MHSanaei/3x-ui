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

	regex := regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) \[([^\]]+)\] (.+)$`)
	messages := strings.Split(message, "\n")

	for _, msg := range messages {
		matches := regex.FindStringSubmatch(msg)

		if len(matches) > 3 {
			level := matches[2]
			msgBody := matches[3]

			// Map the level to the appropriate logger function
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
			lw.lastLine = ""
		} else if msg != "" {
			logger.Debug("XRAY: " + msg)
			lw.lastLine = msg
		}
	}

	return len(m), nil
}
