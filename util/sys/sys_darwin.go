//go:build darwin

package sys

import (
	"encoding/binary"
	"fmt"
	"sync"
	"syscall"

	"github.com/shirou/gopsutil/v4/net"
	"golang.org/x/sys/unix"
)

var SIGUSR1 = syscall.SIGUSR1

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

// sysctl kern.cp_time returns 5 longs in the BSD CPUSTATES order:
// user, nice, sys, intr, idle (CP_INTR=3, CP_IDLE=4). gopsutil reads the
// same layout in cpu_darwin_nocgo.go.
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
		for i := range 5 {
			out[i] = binary.LittleEndian.Uint64(raw[i*8 : (i+1)*8])
		}
	case 5 * 4:
		for i := range 5 {
			out[i] = uint64(binary.LittleEndian.Uint32(raw[i*4 : (i+1)*4]))
		}
	default:
		return 0, fmt.Errorf("unexpected kern.cp_time size: %d", len(raw))
	}

	cpuMu.Lock()
	defer cpuMu.Unlock()

	if !hasLastCPUT {
		lastTotals = out
		hasLastCPUT = true
		return 0, nil
	}

	var deltas [5]uint64
	var totald uint64
	for i := range 5 {
		deltas[i] = out[i] - lastTotals[i]
		totald += deltas[i]
	}
	lastTotals = out

	if totald == 0 {
		return 0, nil
	}
	idleDelta := deltas[4]
	busy := totald - idleDelta
	pct := float64(busy) / float64(totald) * 100.0
	if pct > 100 {
		pct = 100
	}
	return pct, nil
}
