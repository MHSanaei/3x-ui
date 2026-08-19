//go:build !windows

package tor

import "os/exec"

func attachChildLifetime(_ *exec.Cmd) {}
