package tor

import "testing"

func TestDetectPackageManager(t *testing.T) {
	pm, ok := detectPackageManager()
	if !ok {
		return // no package manager on this machine (expected on the Windows dev box) -- nothing further to assert
	}
	if pm.bin == "" {
		t.Fatal("detectPackageManager: ok=true but bin is empty")
	}
	if len(pm.installArgs) == 0 || len(pm.removeArgs) == 0 {
		t.Fatalf("detectPackageManager(%q): installArgs/removeArgs must not be empty", pm.bin)
	}
}

func TestInstallWithNoPackageManager(t *testing.T) {
	if _, ok := detectPackageManager(); ok {
		t.Skip("a real package manager is on PATH -- this case only reproduces where none is")
	}
	if IsAvailable() {
		t.Skip("tor is already on PATH, Install() would no-op before ever reaching package-manager detection")
	}
	err := Install()
	if err == nil {
		t.Fatal("Install() with no package manager on PATH: want error, got nil")
	}
}

func TestUninstallIsANoOpWhenAlreadyAbsent(t *testing.T) {
	if IsAvailable() {
		t.Skip("tor is on PATH -- this case only reproduces where it's already absent")
	}
	if err := Uninstall(); err != nil {
		t.Fatalf("Uninstall() when tor was never installed: want nil, got %v", err)
	}
}
