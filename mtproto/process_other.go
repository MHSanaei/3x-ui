//go:build !windows

package mtproto

import "os/exec"

func attachChildLifetime(_ *exec.Cmd) {}
