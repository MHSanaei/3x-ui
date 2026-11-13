//go:build !linux

package wattunnel

import (
	"os"
	"os/exec"
)

func canRawSocket() bool                                    { return false }
func prepareChild(_ *exec.Cmd, _ Config, _, _ string) error { return nil }

func openPreferredCarrier(path string) (*os.File, error) { return os.Open(path) }
