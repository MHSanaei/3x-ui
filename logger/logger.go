// Package logger provides logging functionality for the 3x-ui panel with
// dual-backend logging (console/syslog and file) and buffered log storage for web UI.
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/op/go-logging"
)

const (
	maxLogBufferSize = 10240                 // Maximum log entries kept in memory
	logFileName      = "3xui.log"            // Log file name
	timeFormat       = "2006/01/02 15:04:05" // Log timestamp format
)

var (
	logger  *logging.Logger
	logFile *os.File

	// logBuffer maintains recent log entries in memory for web UI retrieval
	logBuffer []struct {
		time  string
		level logging.Level
		log   string
	}
)

// InitLogger initializes dual logging backends: console/syslog and file.
// Console logging uses the specified level, file logging always uses DEBUG level.
func InitLogger(level logging.Level) {
	newLogger := logging.MustGetLogger("x-ui")
	backends := make([]logging.Backend, 0, 2)

	// Console/syslog backend with configurable level
	if consoleBackend := initDefaultBackend(); consoleBackend != nil {
		leveledBackend := logging.AddModuleLevel(consoleBackend)
		leveledBackend.SetLevel(level, "x-ui")
		backends = append(backends, leveledBackend)
	}

	// File backend with DEBUG level for comprehensive logging
	if fileBackend := initFileBackend(); fileBackend != nil {
		leveledBackend := logging.AddModuleLevel(fileBackend)
		leveledBackend.SetLevel(logging.DEBUG, "x-ui")
		backends = append(backends, leveledBackend)
	}

	multiBackend := logging.MultiLogger(backends...)
	newLogger.SetBackend(multiBackend)
	logger = newLogger
}

// initDefaultBackend creates the console/syslog logging backend.
// Windows: Uses stderr directly (no syslog support)
// Unix-like: Attempts syslog, falls back to stderr
func initDefaultBackend() logging.Backend {
	var backend logging.Backend
	includeTime := false

	if runtime.GOOS == "windows" {
		// Windows: Use stderr directly (no syslog support)
		backend = logging.NewLogBackend(os.Stderr, "", 0)
		includeTime = true
	} else {
		// Unix-like: Try syslog, fallback to stderr
		if syslogBackend, err := logging.NewSyslogBackend(""); err != nil {
			fmt.Fprintf(os.Stderr, "syslog backend disabled: %v\n", err)
			backend = logging.NewLogBackend(os.Stderr, "", 0)
			includeTime = os.Getppid() > 0
		} else {
			backend = syslogBackend
		}
	}

	return logging.NewBackendFormatter(backend, newFormatter(includeTime))
}

// initFileBackend creates the file logging backend.
// Creates log directory and truncates log file on startup for fresh logs.
func initFileBackend() logging.Backend {
	logDir := config.GetLogFolder()
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log folder %s: %v\n", logDir, err)
		return nil
	}

	logPath := filepath.Join(logDir, logFileName)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o660)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open log file %s: %v\n", logPath, err)
		return nil
	}

	// Close previous log file if exists
	if logFile != nil {
		_ = logFile.Close()
	}
	logFile = file

	backend := logging.NewLogBackend(file, "", 0)
	return logging.NewBackendFormatter(backend, newFormatter(true))
}

// newFormatter creates a log formatter with optional timestamp.
func newFormatter(withTime bool) logging.Formatter {
	format := `%{level} - %{message}`
	if withTime {
		format = `%{time:` + timeFormat + `} %{level} - %{message}`
	}
	return logging.MustStringFormatter(format)
}

// CloseLogger closes the log file and cleans up resources.
// Should be called during application shutdown.
func CloseLogger() {
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
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

// addToBuffer adds a log entry to the in-memory ring buffer for web UI retrieval.
func addToBuffer(level string, newLog string) {
	t := time.Now()
	if len(logBuffer) >= maxLogBufferSize {
		logBuffer = logBuffer[1:]
	}

	logLevel, _ := logging.LogLevel(level)
	logBuffer = append(logBuffer, struct {
		time  string
		level logging.Level
		log   string
	}{
		time:  t.Format(timeFormat),
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
