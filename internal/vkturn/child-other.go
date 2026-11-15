//go:build !linux

package vkturn

import "os/exec"

func configureChild(_ *exec.Cmd) {}
