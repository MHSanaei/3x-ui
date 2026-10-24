package sys

import (
	"sync"
	"time"
)

type LXCStats struct {
	Scope               string
	CPU                 float64
	CPUs                int
	SwapUsed, SwapTotal uint64
	Uptime              uint64
	Unavailable         []string
}

type LXCReader struct {
	ProcRoot, CgroupRoot string
	mu                   sync.Mutex
	lastCPU              uint64
	lastAt               time.Time
	lastCapacity         float64
	sampled              bool
}
