package amneziawgnet

import (
	"fmt"
	"os"
	"runtime"

	"github.com/vishvananda/netlink"
)

// sysModuleAmneziaWGPath is the sysfs path checked for the kernel module.
var sysModuleAmneziaWGPath = "/sys/module/amneziawg"

// osGOOS allows overriding runtime.GOOS during unit testing.
var osGOOS = runtime.GOOS

// probeNetlinkFunc allows overriding the netlink permission probe for tests.
var probeNetlinkFunc = probeNetlinkPermissions

// IsKernelModuleLoaded checks if the amneziawg kernel module is loaded in sysfs.
func IsKernelModuleLoaded() bool {
	if osGOOS != "linux" {
		return false
	}
	info, err := os.Stat(sysModuleAmneziaWGPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// probeNetlinkPermissions checks whether the process can communicate with netlink.
func probeNetlinkPermissions() (bool, error) {
	if runtime.GOOS != "linux" {
		return false, fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
	if os.Geteuid() != 0 {
		return false, fmt.Errorf("insufficient permissions: root or CAP_NET_ADMIN required (UID %d)", os.Geteuid())
	}
	h, err := netlink.NewHandle()
	if err != nil {
		return false, fmt.Errorf("netlink handle open error: %w", err)
	}
	defer h.Close()
	return true, nil
}

// DetectKernelSupport checks whether native amneziawg kernel module can be used.
func DetectKernelSupport() (bool, string) {
	if osGOOS != "linux" {
		return false, fmt.Sprintf("amneziawg kernel module is only supported on linux (current OS: %s)", osGOOS)
	}
	if !IsKernelModuleLoaded() {
		return false, fmt.Sprintf("amneziawg kernel module is not loaded (%s not found)", sysModuleAmneziaWGPath)
	}
	ok, err := probeNetlinkFunc()
	if !ok || err != nil {
		if err != nil {
			return false, fmt.Sprintf("netlink preflight check failed: %v", err)
		}
		return false, "netlink preflight check failed"
	}
	return true, "kernel module detected and ready"
}
