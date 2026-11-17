package sys

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// GetConntrackUsage reads the host's connection tracking table usage.
// Missing files are expected on systems without Linux conntrack support.
func GetConntrackUsage() (count, max uint64, err error) {
	read := func(name string) (uint64, error) {
		data, err := os.ReadFile(HostProc("sys/net/netfilter", name))
		if err != nil {
			return 0, err
		}
		return strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	}
	count, err = read("nf_conntrack_count")
	if err != nil {
		return 0, 0, err
	}
	max, err = read("nf_conntrack_max")
	if err != nil {
		return 0, 0, err
	}
	if max == 0 {
		return 0, 0, fmt.Errorf("conntrack maximum is zero")
	}
	return count, max, nil
}
