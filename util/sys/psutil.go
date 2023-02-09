package sys

import (
	_ "unsafe"
)

//go:linkname HostProc github.com/shirou/gopsutil/internal/common.HostProc
func HostProc(combineWith ...string) string
