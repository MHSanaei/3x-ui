package logger

import (
	"os"
	"sync"

	"github.com/op/go-logging"
)

var (
	logger *logging.Logger
	mu     sync.Mutex
)

func init() {
	InitLogger(logging.INFO)
}

func InitLogger(level logging.Level) {
	mu.Lock()
	defer mu.Unlock()

	if logger != nil {
		return
	}

	format := logging.MustStringFormatter(
		`%{time:2006/01/02 15:04:05} %{level} - %{message}`,
	)
	newLogger := logging.MustGetLogger("x-ui")
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, format)
	backendLeveled := logging.AddModuleLevel(backendFormatter)
	backendLeveled.SetLevel(level, "")
	newLogger.SetBackend(logging.MultiLogger(backendLeveled))

	logger = newLogger
}

func Debug(args ...interface{}) {
	if logger != nil {
		logger.Debug(args...)
	}
}

func Debugf(format string, args ...interface{}) {
	if logger != nil {
		logger.Debugf(format, args...)
	}
}

func Info(args ...interface{}) {
	if logger != nil {
		logger.Info(args...)
	}
}

func Infof(format string, args ...interface{}) {
	if logger != nil {
		logger.Infof(format, args...)
	}
}

func Warning(args ...interface{}) {
	if logger != nil {
		logger.Warning(args...)
	}
}

func Warningf(format string, args ...interface{}) {
	if logger != nil {
		logger.Warningf(format, args...)
	}
}

func Error(args ...interface{}) {
	if logger != nil {
		logger.Error(args...)
	}
}

func Errorf(format string, args ...interface{}) {
	if logger != nil {
		logger.Errorf(format, args...)
	}
}

func Notice(args ...interface{}) {
	if logger != nil {
		logger.Notice(args...)
	}
}

func Noticef(format string, args ...interface{}) {
	if logger != nil {
		logger.Noticef(format, args...)
	}
}
