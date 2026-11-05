//go:build windows

package mtproto

import (
	"os/exec"

	"github.com/mhsanaei/3x-ui/v3/internal/childproc"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func attachChildLifetime(cmd *exec.Cmd) {
	if err := childproc.Attach(cmd); err != nil {
		logger.Warningf("mtproto: child job attach failed: %v", err)
	}
}
