package xray

import (
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
	// Convert the data to a string
	message := strings.TrimSpace(string(m))
	messages := strings.Split(message, "\n")
	lw.lastLine = messages[len(messages)-1]

	for _, msg := range messages {
		messageBody := msg

		// Remove timestamp
		splittedMsg := strings.SplitN(msg, " ", 3)
		if len(splittedMsg) > 2 {
			messageBody = strings.TrimSpace(strings.SplitN(msg, " ", 3)[2])
		}

		// Find level in []
		startIndex := strings.Index(messageBody, "[")
		endIndex := strings.Index(messageBody, "]")
		if startIndex != -1 && endIndex != -1 {
			level := strings.TrimSpace(messageBody[startIndex+1 : endIndex])
			msgBody := "XRAY: " + strings.TrimSpace(messageBody[endIndex+1:])

			// Map the level to the appropriate logger function
			switch level {
			case "Debug":
				logger.Debug(msgBody)
			case "Info":
				logger.Info(msgBody)
			case "Warning":
				logger.Warning(msgBody)
			case "Error":
				logger.Error(msgBody)
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
