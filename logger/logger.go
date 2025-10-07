// Package logger provides logging functionality for the 3x-ui panel with
// buffered log storage and multiple log levels.
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/op/go-logging"
)

var (
	logger  *logging.Logger
	logFile *os.File

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

// InitLogger initializes the logger backends with the specified logging level.
func InitLogger(level logging.Level) {
	newLogger := logging.MustGetLogger("x-ui")
	backends := make([]logging.Backend, 0, 2)

	if defaultBackend := initDefaultBackend(); defaultBackend != nil {
		backends = append(backends, defaultBackend)
	}
	if fileBackend := initFileBackend(); fileBackend != nil {
		backends = append(backends, fileBackend)
	}

	multiBackend := logging.MultiLogger(backends...)
	multiBackend.SetLevel(level, "x-ui")

	newLogger.SetBackend(multiBackend)
	logger = newLogger
}

func initDefaultBackend() logging.Backend {
	backendSyslog, err := logging.NewSyslogBackend("")
	var backend logging.Backend = backendSyslog
	includeTime := false
	if err != nil {
		fmt.Fprintf(os.Stderr, "syslog backend disabled: %v\n", err)
		ppid := os.Getppid()
		backend = logging.NewLogBackend(os.Stderr, "", 0)
		includeTime = ppid > 0
	}
	return logging.NewBackendFormatter(backend, newFormatter(includeTime))
}

func initFileBackend() logging.Backend {
	logDir := config.GetLogFolder()
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log folder %s: %v\n", logDir, err)
		return nil
	}

	logPath := filepath.Join(logDir, "3xui.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o660)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open log file %s: %v\n", logPath, err)
		return nil
	}

	if logFile != nil {
		_ = logFile.Close()
	}
	logFile = file

	backend := logging.NewLogBackend(file, "", 0)
	return logging.NewBackendFormatter(backend, newFormatter(true))
}

func newFormatter(withTime bool) logging.Formatter {
	if withTime {
		return logging.MustStringFormatter(`%{time:2006/01/02 15:04:05} %{level} - %{message}`)
	}
	return logging.MustStringFormatter(`%{level} - %{message}`)
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
