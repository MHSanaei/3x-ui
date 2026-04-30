//go:build !linux

package service

import "os/exec"

func setDetachedProcess(cmd *exec.Cmd) {}
