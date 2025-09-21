//go:build windows
// +build windows

package sys

import (
	"errors"
	"sync"
	"syscall"
	"unsafe"

	"github.com/shirou/gopsutil/v4/net"
)

// GetConnectionCount returns the number of active connections for the specified protocol ("tcp" or "udp").
func GetConnectionCount(proto string) (int, error) {
	if proto != "tcp" && proto != "udp" {
		return 0, errors.New("invalid protocol")
	}

	stats, err := net.Connections(proto)
	if err != nil {
		return 0, err
	}
	return len(stats), nil
}

// GetTCPCount returns the number of active TCP connections.
func GetTCPCount() (int, error) {
	return GetConnectionCount("tcp")
}

// GetUDPCount returns the number of active UDP connections.
func GetUDPCount() (int, error) {
	return GetConnectionCount("udp")
}

// --- CPU Utilization (Windows native) ---

var (
	modKernel32        = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemTimes = modKernel32.NewProc("GetSystemTimes")

	cpuMu      sync.Mutex
	lastIdle   uint64
	lastKernel uint64
	lastUser   uint64
	hasLast    bool
)

type filetime struct {
	LowDateTime  uint32
	HighDateTime uint32
}

// ftToUint64 converts a Windows FILETIME-like struct to a uint64 for
// arithmetic and delta calculations used by CPUPercentRaw.
func ftToUint64(ft filetime) uint64 {
	return (uint64(ft.HighDateTime) << 32) | uint64(ft.LowDateTime)
}

// CPUPercentRaw returns the instantaneous total CPU utilization percentage using
// Windows GetSystemTimes across all logical processors. The first call returns 0
// as it initializes the baseline. Subsequent calls compute deltas.
func CPUPercentRaw() (float64, error) {
	var idleFT, kernelFT, userFT filetime
	r1, _, e1 := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idleFT)),
		uintptr(unsafe.Pointer(&kernelFT)),
		uintptr(unsafe.Pointer(&userFT)),
	)
	if r1 == 0 { // failure
		if e1 != nil {
			return 0, e1
		}
		return 0, syscall.GetLastError()
	}

	idle := ftToUint64(idleFT)
	kernel := ftToUint64(kernelFT)
	user := ftToUint64(userFT)

	cpuMu.Lock()
	defer cpuMu.Unlock()

	if !hasLast {
		lastIdle = idle
		lastKernel = kernel
		lastUser = user
		hasLast = true
		return 0, nil
	}

	idleDelta := idle - lastIdle
	kernelDelta := kernel - lastKernel
	userDelta := user - lastUser

	// Update for next call
	lastIdle = idle
	lastKernel = kernel
	lastUser = user

	total := kernelDelta + userDelta
	if total == 0 {
		return 0, nil
	}
	// On Windows, kernel time includes idle time; busy = total - idle
	busy := total - idleDelta

	pct := float64(busy) / float64(total) * 100.0
	// lower bound not needed; ratios of uint64 are non-negative
	if pct > 100 {
		pct = 100
	}
	return pct, nil
}
