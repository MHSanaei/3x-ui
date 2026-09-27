//go:build !windows

package logger

import (
	"fmt"
	"os"

	"github.com/op/go-logging"
)

// newConsoleBackend prefers syslog and falls back to stderr when it is unavailable.
func newConsoleBackend() (backend logging.Backend, includeTime bool) {
	syslogBackend, err := logging.NewSyslogBackend("")
	if err == nil {
		return syslogBackend, false
	}
	fmt.Fprintf(os.Stderr, "syslog backend disabled: %v\n", err)
	return logging.NewLogBackend(os.Stderr, "", 0), os.Getppid() > 0
}
