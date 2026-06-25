package sys

import (
	"os"
	"sync"

	"github.com/shirou/gopsutil/v4/process"
)

var (
	selfProc     *process.Process
	selfProcOnce sync.Once
)

// SelfRSS returns the resident set size of the current process in bytes — the
// real physical memory the OS attributes to the panel. Unlike
// runtime.MemStats.Sys (a never-shrinking high-water mark of reserved address
// space that also counts memory already returned to the OS), RSS reflects current
// usage and drops as memory is released. Returns 0 when unavailable.
func SelfRSS() uint64 {
	selfProcOnce.Do(func() {
		if p, err := process.NewProcess(int32(os.Getpid())); err == nil {
			selfProc = p
		}
	})

	if selfProc == nil {
		return 0
	}
	if mi, err := selfProc.MemoryInfo(); err == nil && mi != nil {
		return mi.RSS
	}
	return 0
}
