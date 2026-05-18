//go:build linux

package sys

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

var SIGUSR1 = syscall.SIGUSR1

// countConnections returns the number of entries in a /proc/net/{tcp,udp}[6]
// file. Returns 0 if the file is absent (e.g. /proc/net/tcp6 when IPv6 is
// disabled) and excludes the column header line.
func countConnections(path string) (int, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		n++
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	if n > 0 {
		n-- // first line is the column header
	}
	return n, nil
}

// GetTCPCount returns the number of active TCP connections by reading
// /proc/net/tcp and /proc/net/tcp6 when available.
func GetTCPCount() (int, error) {
	root := HostProc()
	tcp4, err := countConnections(root + "/net/tcp")
	if err != nil {
		return 0, err
	}
	tcp6, err := countConnections(root + "/net/tcp6")
	if err != nil {
		return 0, err
	}
	return tcp4 + tcp6, nil
}

// GetUDPCount returns the number of active UDP connections by reading
// /proc/net/udp and /proc/net/udp6 when available.
func GetUDPCount() (int, error) {
	root := HostProc()
	udp4, err := countConnections(root + "/net/udp")
	if err != nil {
		return 0, err
	}
	udp6, err := countConnections(root + "/net/udp6")
	if err != nil {
		return 0, err
	}
	return udp4 + udp6, nil
}

// --- CPU Utilization (Linux native) ---

var (
	cpuMu       sync.Mutex
	lastTotal   uint64
	lastIdleAll uint64
	hasLast     bool
)

// CPUPercentRaw returns instantaneous total CPU utilization by reading
// /proc/stat. First call initializes and returns 0; subsequent calls return
// busy/total * 100. Uses HostProc so HOST_PROC overrides (containers) apply.
func CPUPercentRaw() (float64, error) {
	f, err := os.Open(HostProc("stat"))
	if err != nil {
		return 0, err
	}
	defer f.Close()

	rd := bufio.NewReader(f)
	line, err := rd.ReadString('\n')
	if err != nil && err != io.EOF {
		return 0, err
	}
	// Expect: cpu  user nice system idle iowait irq softirq steal guest guest_nice
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, fmt.Errorf("unexpected /proc/stat format")
	}

	nums := make([]uint64, 0, len(fields)-1)
	for i := 1; i < len(fields); i++ {
		v, err := strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			break
		}
		nums = append(nums, v)
	}
	if len(nums) < 4 {
		return 0, fmt.Errorf("insufficient cpu fields")
	}
	for len(nums) < 8 {
		nums = append(nums, 0)
	}

	user, nice, system, idle := nums[0], nums[1], nums[2], nums[3]
	iowait, irq, softirq, steal := nums[4], nums[5], nums[6], nums[7]

	idleAll := idle + iowait
	nonIdle := user + nice + system + irq + softirq + steal
	total := idleAll + nonIdle

	cpuMu.Lock()
	defer cpuMu.Unlock()

	if !hasLast {
		lastTotal = total
		lastIdleAll = idleAll
		hasLast = true
		return 0, nil
	}

	totald := total - lastTotal
	idled := idleAll - lastIdleAll
	lastTotal = total
	lastIdleAll = idleAll

	if totald == 0 {
		return 0, nil
	}
	busy := totald - idled
	pct := float64(busy) / float64(totald) * 100.0
	if pct > 100 {
		pct = 100
	}
	return pct, nil
}
