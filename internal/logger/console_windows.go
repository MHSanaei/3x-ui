//go:build windows

package logger

import (
	"os"

	"github.com/op/go-logging"
)

// newConsoleBackend logs to stderr: go-logging has no syslog on Windows.
func newConsoleBackend() (backend logging.Backend, includeTime bool) {
	return logging.NewLogBackend(os.Stderr, "", 0), true
}
