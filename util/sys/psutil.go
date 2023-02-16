package sys

import (
	_ "unsafe"
)

//go:linkname HostProc github.com/shirou/gopsutil/v3/internal/common.HostProc
func HostProc(combineWith ...string) string
