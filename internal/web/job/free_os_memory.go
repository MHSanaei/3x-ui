package job

import "runtime/debug"

// MemoryReleaseJob returns freed heap spans to the OS so steady-state RSS tracks
// the live heap between the bursty traffic-collection jobs, instead of lingering
// at the high-water mark until the scavenger lazily reclaims it.
type MemoryReleaseJob struct{}

// NewMemoryReleaseJob creates a new memory-release job instance.
func NewMemoryReleaseJob() *MemoryReleaseJob {
	return new(MemoryReleaseJob)
}

// Run forces a GC and returns as much free memory to the OS as possible. It is
// scheduled on a minutes cadence because FreeOSMemory triggers a full GC.
func (j *MemoryReleaseJob) Run() {
	debug.FreeOSMemory()
}
