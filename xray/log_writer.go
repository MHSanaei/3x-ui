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
	regex := regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) \[([^\]]+)\] (.+)$`)
	// Convert the data to a string
	message := strings.TrimSpace(string(m))
	messages := strings.Split(message, "\n")
	lw.lastLine = messages[len(messages)-1]

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
		} else if msg != "" {
			logger.Debug("XRAY: " + msg)
			return len(m), nil
		}
	}

	return len(m), nil
}
