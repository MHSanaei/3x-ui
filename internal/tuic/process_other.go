//go:build !windows

package tuic

import "os/exec"

func attachChildLifetime(_ *exec.Cmd) {}
