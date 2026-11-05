package childproc

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"
)

func Wait(done <-chan struct{}, timeout time.Duration) error {
	if done == nil {
		return nil
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
		return fmt.Errorf("child process did not exit within %s", timeout)
	}
}

func Stop(cmd *exec.Cmd, done <-chan struct{}, grace, force time.Duration) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if runtime.GOOS != "windows" {
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		if err := Wait(done, grace); err == nil {
			return nil
		}
	}
	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return Wait(done, force)
}
