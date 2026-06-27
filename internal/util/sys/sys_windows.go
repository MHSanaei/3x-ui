//go:build windows

package sys

import (
	"errors"
	"sync"
	"syscall"
	"unsafe"

	"github.com/shirou/gopsutil/v4/net"
	"golang.org/x/sys/windows"
)

var SIGUSR1 = syscall.Signal(0)

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
	// NewLazySystemDLL forces the load from %SystemRoot%\System32 so a
	// kernel32.dll planted next to the binary can't hijack the call.
	modKernel32        = windows.NewLazySystemDLL("kernel32.dll")
	procGetSystemTimes = modKernel32.NewProc("GetSystemTimes")

	cpuMu      sync.Mutex
	lastIdle   uint64
	lastKernel uint64
	lastUser   uint64
	hasLast    bool
)

func ftToUint64(ft windows.Filetime) uint64 {
	return (uint64(ft.HighDateTime) << 32) | uint64(ft.LowDateTime)
}

// CPUPercentRaw returns instantaneous total CPU utilization across all
// logical processors via Windows GetSystemTimes. The first call returns 0
// while it initializes the baseline; subsequent calls compute deltas.
func CPUPercentRaw() (float64, error) {
	var idleFT, kernelFT, userFT windows.Filetime
	r1, _, e1 := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idleFT)),
		uintptr(unsafe.Pointer(&kernelFT)),
		uintptr(unsafe.Pointer(&userFT)),
	)
	if r1 == 0 {
		var errno syscall.Errno
		if errors.As(e1, &errno) && errno != 0 {
			return 0, errno
		}
		return 0, errors.New("GetSystemTimes failed")
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

	lastIdle = idle
	lastKernel = kernel
	lastUser = user

	total := kernelDelta + userDelta
	if total == 0 {
		return 0, nil
	}
	// kernel time includes idle on Windows; busy = total - idle
	busy := total - idleDelta

	pct := float64(busy) / float64(total) * 100.0
	if pct > 100 {
		pct = 100
	}
	return pct, nil
}
