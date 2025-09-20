// Package logger provides logging functionality for the 3x-ui panel with
// buffered log storage and multiple log levels.
package logger

import (
	"fmt"
	"os"
	"time"

	"github.com/op/go-logging"
)

var (
	logger *logging.Logger

	// addToBuffer appends a log entry into the in-memory ring buffer used for
	// retrieving recent logs via the web UI. It keeps the buffer bounded to avoid
	// uncontrolled growth.
	logBuffer []struct {
		time  string
		level logging.Level
		log   string
	}
)

func init() {
	InitLogger(logging.INFO)
}

// InitLogger initializes the logger with the specified logging level.
func InitLogger(level logging.Level) {
	newLogger := logging.MustGetLogger("x-ui")
	var err error
	var backend logging.Backend
	var format logging.Formatter
	ppid := os.Getppid()

	backend, err = logging.NewSyslogBackend("")
	if err != nil {
		println(err)
		backend = logging.NewLogBackend(os.Stderr, "", 0)
	}
	if ppid > 0 && err != nil {
		format = logging.MustStringFormatter(`%{time:2006/01/02 15:04:05} %{level} - %{message}`)
	} else {
		format = logging.MustStringFormatter(`%{level} - %{message}`)
	}

	backendFormatter := logging.NewBackendFormatter(backend, format)
	backendLeveled := logging.AddModuleLevel(backendFormatter)
	backendLeveled.SetLevel(level, "x-ui")
	newLogger.SetBackend(backendLeveled)

	logger = newLogger
}

// Debug logs a debug message and adds it to the log buffer.
func Debug(args ...any) {
	logger.Debug(args...)
	addToBuffer("DEBUG", fmt.Sprint(args...))
}

// Debugf logs a formatted debug message and adds it to the log buffer.
func Debugf(format string, args ...any) {
	logger.Debugf(format, args...)
	addToBuffer("DEBUG", fmt.Sprintf(format, args...))
}

// Info logs an info message and adds it to the log buffer.
func Info(args ...any) {
	logger.Info(args...)
	addToBuffer("INFO", fmt.Sprint(args...))
}

// Infof logs a formatted info message and adds it to the log buffer.
func Infof(format string, args ...any) {
	logger.Infof(format, args...)
	addToBuffer("INFO", fmt.Sprintf(format, args...))
}

// Notice logs a notice message and adds it to the log buffer.
func Notice(args ...any) {
	logger.Notice(args...)
	addToBuffer("NOTICE", fmt.Sprint(args...))
}

// Noticef logs a formatted notice message and adds it to the log buffer.
func Noticef(format string, args ...any) {
	logger.Noticef(format, args...)
	addToBuffer("NOTICE", fmt.Sprintf(format, args...))
}

// Warning logs a warning message and adds it to the log buffer.
func Warning(args ...any) {
	logger.Warning(args...)
	addToBuffer("WARNING", fmt.Sprint(args...))
}

// Warningf logs a formatted warning message and adds it to the log buffer.
func Warningf(format string, args ...any) {
	logger.Warningf(format, args...)
	addToBuffer("WARNING", fmt.Sprintf(format, args...))
}

// Error logs an error message and adds it to the log buffer.
func Error(args ...any) {
	logger.Error(args...)
	addToBuffer("ERROR", fmt.Sprint(args...))
}

// Errorf logs a formatted error message and adds it to the log buffer.
func Errorf(format string, args ...any) {
	logger.Errorf(format, args...)
	addToBuffer("ERROR", fmt.Sprintf(format, args...))
}

func addToBuffer(level string, newLog string) {
	t := time.Now()
	if len(logBuffer) >= 10240 {
		logBuffer = logBuffer[1:]
	}

	logLevel, _ := logging.LogLevel(level)
	logBuffer = append(logBuffer, struct {
		time  string
		level logging.Level
		log   string
	}{
		time:  t.Format("2006/01/02 15:04:05"),
		level: logLevel,
		log:   newLog,
	})
}

// GetLogs retrieves up to c log entries from the buffer that are at or below the specified level.
func GetLogs(c int, level string) []string {
	var output []string
	logLevel, _ := logging.LogLevel(level)

	for i := len(logBuffer) - 1; i >= 0 && len(output) <= c; i-- {
		if logBuffer[i].level <= logLevel {
			output = append(output, fmt.Sprintf("%s %s - %s", logBuffer[i].time, logBuffer[i].level, logBuffer[i].log))
		}
	}
	return output
}
