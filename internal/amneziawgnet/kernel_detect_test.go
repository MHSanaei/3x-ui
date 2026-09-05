package amneziawgnet

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestDetectKernelSupport_NonLinux verifies non-Linux OS returns false with reason.
func TestDetectKernelSupport_NonLinux(t *testing.T) {
	origGOOS := osGOOS
	defer func() { osGOOS = origGOOS }()

	for _, nonLinuxOS := range []string{"darwin", "windows", "freebsd"} {
		t.Run(nonLinuxOS, func(t *testing.T) {
			osGOOS = nonLinuxOS
			if loaded := IsKernelModuleLoaded(); loaded {
				t.Fatalf("expected IsKernelModuleLoaded to be false on %s", nonLinuxOS)
			}
			supported, reason := DetectKernelSupport()
			if supported {
				t.Fatalf("expected DetectKernelSupport to be false on %s", nonLinuxOS)
			}
			if !strings.Contains(reason, "linux") {
				t.Fatalf("expected reason to mention linux, got %q", reason)
			}
		})
	}
}

// TestDetectKernelSupport_ModuleNotLoaded verifies detection when sysfs path is missing.
func TestDetectKernelSupport_ModuleNotLoaded(t *testing.T) {
	origGOOS := osGOOS
	origPath := sysModuleAmneziaWGPath
	defer func() {
		osGOOS = origGOOS
		sysModuleAmneziaWGPath = origPath
	}()

	osGOOS = "linux"
	sysModuleAmneziaWGPath = filepath.Join(t.TempDir(), "nonexistent_module_dir")

	if loaded := IsKernelModuleLoaded(); loaded {
		t.Fatalf("expected IsKernelModuleLoaded to be false when sysfs path does not exist")
	}

	supported, reason := DetectKernelSupport()
	if supported {
		t.Fatalf("expected DetectKernelSupport to be false when module is not loaded")
	}
	if !strings.Contains(reason, "not loaded") {
		t.Fatalf("expected reason to mention not loaded, got %q", reason)
	}
}

// TestDetectKernelSupport_PermissionError verifies netlink permission failure handling.
func TestDetectKernelSupport_PermissionError(t *testing.T) {
	origGOOS := osGOOS
	origPath := sysModuleAmneziaWGPath
	origProbe := probeNetlinkFunc
	defer func() {
		osGOOS = origGOOS
		sysModuleAmneziaWGPath = origPath
		probeNetlinkFunc = origProbe
	}()

	tempDir := t.TempDir()
	osGOOS = "linux"
	sysModuleAmneziaWGPath = tempDir

	probeNetlinkFunc = func() (bool, error) {
		return false, errors.New("permission denied (CAP_NET_ADMIN missing)")
	}

	if loaded := IsKernelModuleLoaded(); !loaded {
		t.Fatalf("expected IsKernelModuleLoaded to be true when sysfs directory exists")
	}

	supported, reason := DetectKernelSupport()
	if supported {
		t.Fatalf("expected DetectKernelSupport to be false on permission error")
	}
	if !strings.Contains(reason, "permission") && !strings.Contains(reason, "netlink") {
		t.Fatalf("expected reason to mention permission/netlink, got %q", reason)
	}
}

// TestDetectKernelSupport_Success verifies detection when module is loaded and probe passes.
func TestDetectKernelSupport_Success(t *testing.T) {
	origGOOS := osGOOS
	origPath := sysModuleAmneziaWGPath
	origProbe := probeNetlinkFunc
	defer func() {
		osGOOS = origGOOS
		sysModuleAmneziaWGPath = origPath
		probeNetlinkFunc = origProbe
	}()

	tempDir := t.TempDir()
	osGOOS = "linux"
	sysModuleAmneziaWGPath = tempDir

	probeNetlinkFunc = func() (bool, error) {
		return true, nil
	}

	if loaded := IsKernelModuleLoaded(); !loaded {
		t.Fatalf("expected IsKernelModuleLoaded to be true when sysfs directory exists")
	}

	supported, reason := DetectKernelSupport()
	if !supported {
		t.Fatalf("expected DetectKernelSupport to be true, got reason: %s", reason)
	}
}

// TestDetectKernelSupport_RealEnvironment verifies no panics in current environment.
func TestDetectKernelSupport_RealEnvironment(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("DetectKernelSupport panicked: %v", r)
		}
	}()

	loaded := IsKernelModuleLoaded()
	supported, reason := DetectKernelSupport()

	if runtime.GOOS != "linux" && (loaded || supported) {
		t.Fatalf("expected loaded=false, supported=false on %s", runtime.GOOS)
	}

	if supported && !loaded {
		t.Fatalf("supported cannot be true when loaded is false")
	}

	if supported && reason == "" {
		t.Fatalf("reason should be informative even when supported")
	}
}
