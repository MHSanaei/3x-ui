//go:build !windows

package psiphon

import "os/exec"

func attachChildLifetime(_ *exec.Cmd) {}
