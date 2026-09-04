//go:build linux

package tuic

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func readProcIO(pid int) (int64, int64, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/io", pid))
	if err != nil {
		return 0, 0, err
	}
	var rchar, wchar int64
	lines := strings.Split(string(data), "\n")
	for _, l := range lines {
		if strings.HasPrefix(l, "rchar: ") {
			rchar, _ = strconv.ParseInt(strings.TrimSpace(l[7:]), 10, 64)
		} else if strings.HasPrefix(l, "wchar: ") {
			wchar, _ = strconv.ParseInt(strings.TrimSpace(l[7:]), 10, 64)
		}
	}
	return rchar, wchar, nil
}
