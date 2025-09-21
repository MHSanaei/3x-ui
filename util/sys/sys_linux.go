//go:build linux
// +build linux

package sys

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

func getLinesNum(filename string) (int, error) {
	file, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	sum := 0
	buf := make([]byte, 8192)
	for {
		n, err := file.Read(buf)

		var buffPosition int
		for {
			i := bytes.IndexByte(buf[buffPosition:n], '\n')
			if i < 0 {
				break
			}
			buffPosition += i + 1
			sum++
		}

		if err == io.EOF {
			break
		} else if err != nil {
			return 0, err
		}
	}
	return sum, nil
}

// GetTCPCount returns the number of active TCP connections by reading
// /proc/net/tcp and /proc/net/tcp6 when available.
func GetTCPCount() (int, error) {
	root := HostProc()

	tcp4, err := safeGetLinesNum(fmt.Sprintf("%v/net/tcp", root))
	if err != nil {
		return 0, err
	}
	tcp6, err := safeGetLinesNum(fmt.Sprintf("%v/net/tcp6", root))
	if err != nil {
		return 0, err
	}

	return tcp4 + tcp6, nil
}

func GetUDPCount() (int, error) {
	root := HostProc()

	udp4, err := safeGetLinesNum(fmt.Sprintf("%v/net/udp", root))
	if err != nil {
		return 0, err
	}
	udp6, err := safeGetLinesNum(fmt.Sprintf("%v/net/udp6", root))
	if err != nil {
		return 0, err
	}

	return udp4 + udp6, nil
}

// safeGetLinesNum returns 0 if the file does not exist, otherwise forwards
// to getLinesNum to count the number of lines.
func safeGetLinesNum(path string) (int, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	return getLinesNum(path)
}

// --- CPU Utilization (Linux native) ---

var (
	cpuMu       sync.Mutex
	lastTotal   uint64
	lastIdleAll uint64
	hasLast     bool
)

// CPUPercentRaw returns instantaneous total CPU utilization by reading /proc/stat.
// First call initializes and returns 0; subsequent calls return busy/total * 100.
func CPUPercentRaw() (float64, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	rd := bufio.NewReader(f)
	line, err := rd.ReadString('\n')
	if err != nil && err != io.EOF {
		return 0, err
	}
	// Expect line like: cpu  user nice system idle iowait irq softirq steal guest guest_nice
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, fmt.Errorf("unexpected /proc/stat format")
	}

	var nums []uint64
	for i := 1; i < len(fields); i++ {
		v, err := strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			break
		}
		nums = append(nums, v)
	}
	if len(nums) < 4 { // need at least user,nice,system,idle
		return 0, fmt.Errorf("insufficient cpu fields")
	}

	// Conform with standard Linux CPU accounting
	var user, nice, system, idle, iowait, irq, softirq, steal uint64
	user = nums[0]
	if len(nums) > 1 {
		nice = nums[1]
	}
	if len(nums) > 2 {
		system = nums[2]
	}
	if len(nums) > 3 {
		idle = nums[3]
	}
	if len(nums) > 4 {
		iowait = nums[4]
	}
	if len(nums) > 5 {
		irq = nums[5]
	}
	if len(nums) > 6 {
		softirq = nums[6]
	}
	if len(nums) > 7 {
		steal = nums[7]
	}

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
