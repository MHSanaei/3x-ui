//go:build darwin
// +build darwin

package sys

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/shirou/gopsutil/v4/net"
	"golang.org/x/sys/unix"
)

func GetTCPCount() (int, error) {
	stats, err := net.Connections("tcp")
	if err != nil {
		return 0, err
	}
	return len(stats), nil
}

func GetUDPCount() (int, error) {
	stats, err := net.Connections("udp")
	if err != nil {
		return 0, err
	}
	return len(stats), nil
}

// --- CPU Utilization (macOS native) ---

// sysctl kern.cp_time returns an array of 5 longs: user, nice, sys, idle, intr.
// We compute utilization deltas without cgo.
var (
	cpuMu       sync.Mutex
	lastTotals  [5]uint64
	hasLastCPUT bool
)

func CPUPercentRaw() (float64, error) {
	raw, err := unix.SysctlRaw("kern.cp_time")
	if err != nil {
		return 0, err
	}
	// Expect either 5*8 bytes (uint64) or 5*4 bytes (uint32)
	var out [5]uint64
	switch len(raw) {
	case 5 * 8:
		for i := 0; i < 5; i++ {
			out[i] = binary.LittleEndian.Uint64(raw[i*8 : (i+1)*8])
		}
	case 5 * 4:
		for i := 0; i < 5; i++ {
			out[i] = uint64(binary.LittleEndian.Uint32(raw[i*4 : (i+1)*4]))
		}
	default:
		return 0, fmt.Errorf("unexpected kern.cp_time size: %d", len(raw))
	}

	// user, nice, sys, idle, intr
	user := out[0]
	nice := out[1]
	sysv := out[2]
	idle := out[3]
	intr := out[4]

	cpuMu.Lock()
	defer cpuMu.Unlock()

	if !hasLastCPUT {
		lastTotals = out
		hasLastCPUT = true
		return 0, nil
	}

	dUser := user - lastTotals[0]
	dNice := nice - lastTotals[1]
	dSys := sysv - lastTotals[2]
	dIdle := idle - lastTotals[3]
	dIntr := intr - lastTotals[4]

	lastTotals = out

	totald := dUser + dNice + dSys + dIdle + dIntr
	if totald == 0 {
		return 0, nil
	}
	busy := totald - dIdle
	pct := float64(busy) / float64(totald) * 100.0
	if pct > 100 {
		pct = 100
	}
	return pct, nil
}
