package amneziawgnet

import (
	"os"
	"runtime"
)

// IsKernelModuleLoaded reports whether the amneziawg kernel module is currently
// loaded in the running Linux kernel by checking sysfs.
func IsKernelModuleLoaded() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	_, err := os.Stat("/sys/module/amneziawg")
	return err == nil
}

// DetectKernelSupport probes whether the current environment supports running
// native AmneziaWG interfaces via the Linux kernel module.
func DetectKernelSupport() (bool, string) {
	if runtime.GOOS != "linux" {
		return false, "non-linux operating system (" + runtime.GOOS + ")"
	}
	if !IsKernelModuleLoaded() {
		return false, "amneziawg kernel module not loaded (/sys/module/amneziawg not found)"
	}
	return true, "amneziawg kernel module detected"
}
