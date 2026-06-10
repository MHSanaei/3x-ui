//go:build !linux

package panel

import "os/exec"

func setDetachedProcess(cmd *exec.Cmd) {}
