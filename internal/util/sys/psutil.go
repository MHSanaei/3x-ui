// Package sys provides system utilities for monitoring network connections and CPU usage.
// Platform-specific implementations are provided for Windows, Linux, and macOS.
package sys

import (
	_ "unsafe"
)

//go:linkname HostProc github.com/shirou/gopsutil/v4/internal/common.HostProc
func HostProc(combineWith ...string) string
