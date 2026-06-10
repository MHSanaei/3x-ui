//go:build !windows

package xray

import "os/exec"

func attachChildLifetime(_ *exec.Cmd) {}
