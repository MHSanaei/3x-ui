//go:build !windows

package childproc

import "os/exec"

func Attach(_ *exec.Cmd) error { return nil }
