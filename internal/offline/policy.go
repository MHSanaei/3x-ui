package offline

import (
	"errors"
	"net/netip"
	"strings"
	"sync/atomic"
)

var running atomic.Bool

var ErrRemote = errors.New("remote service is disabled in the offline panel")

// BeginRuntime enables offline policy for the panel and subscription servers.
func BeginRuntime() { running.Store(true) }

func Running() bool { return running.Load() }

// LocalHost accepts literal private or loopback addresses without DNS.
func LocalHost(host string) bool {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	ip = ip.Unmap()
	return ip.IsPrivate() || ip.IsLoopback()
}
