package xray

import (
	"errors"
	"io/fs"
	"os"
	"sync"

	"golang.org/x/sys/unix"
)

// The files the core's WireGuard kernel TUN opens before it creates the device
// (proxy/wireguard/tun_linux.go); a missing one is skipped there too.
var kernelTunSysctls = []string{
	"/proc/sys/net/ipv4/conf/all/rp_filter",
	"/proc/sys/net/ipv6/conf/all/disable_ipv6",
	"/proc/sys/net/ipv6/conf/all/rp_filter",
}

// KernelTunBlocked reports that the core would put a WireGuard outbound on a kernel TUN
// (it holds CAP_NET_ADMIN) where a read-only /proc/sys or a missing /dev/net/tun aborts its start.
var KernelTunBlocked = sync.OnceValue(func() bool {
	return hasEffectiveNetAdmin() && !kernelTunReachable()
})

func hasEffectiveNetAdmin() bool {
	hdr := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var data [2]unix.CapUserData
	if err := unix.Capget(&hdr, &data[0]); err != nil {
		return false
	}
	return data[0].Effective&(1<<unix.CAP_NET_ADMIN) != 0
}

func kernelTunReachable() bool {
	for _, path := range kernelTunSysctls {
		f, err := os.OpenFile(path, os.O_WRONLY, 0)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return false
		}
		_ = f.Close()
	}
	tun, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
	if err != nil {
		return false
	}
	_ = tun.Close()
	return true
}
