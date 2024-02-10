package xray

import (
	"strings"
	"x-ui/logger"
)

func NewLogWriter() *LogWriter {
	return &LogWriter{
		listeners: &[]func(line string){},
	}
}

type LogWriter struct {
	lastLine  string
	listeners *[]func(line string)
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
		if startIndex != -1 && endIndex != -1 && startIndex < endIndex {
			level := strings.TrimSpace(messageBody[startIndex+1 : endIndex])
			rawMsg := strings.TrimSpace(messageBody[endIndex+1:])
			msgBody := "XRAY: " + rawMsg

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

			// Notify listeners of the message
			for _, listener := range *lw.listeners {
				listener(messageBody)
			}
		} else if msg != "" {
			logger.Debug("XRAY: " + msg)
			return len(m), nil
		}
	}

	return len(m), nil
}

// SetListener adds a listener to the log writer
// The listener will be called with each line of the log
// that is written to the log writer
// We use this method to prevent reading the log file for better performance
func (lw *LogWriter) SetListener(listener func(line string)) {
	*lw.listeners = append(*lw.listeners, listener)
}
