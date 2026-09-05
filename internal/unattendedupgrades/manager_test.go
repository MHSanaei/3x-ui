package unattendedupgrades

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempAptPaths points autoUpgradesPath/dropInPath at throwaway files for
// the duration of the test, restoring the real /etc/apt/apt.conf.d paths
// afterward, and reports isInstalled() as true (the common case: Configure/
// GetStatus are only meaningful once the package is actually installed).
func withTempAptPaths(t *testing.T) {
	t.Helper()
	dir := t.TempDir()

	realAuto, realDropIn := autoUpgradesPath, dropInPath
	autoUpgradesPath = filepath.Join(dir, "20auto-upgrades")
	dropInPath = filepath.Join(dir, "52-3x-ui-unattended-upgrades")
	t.Cleanup(func() { autoUpgradesPath, dropInPath = realAuto, realDropIn })

	realInstalled := isInstalled
	isInstalled = func() bool { return true }
	t.Cleanup(func() { isInstalled = realInstalled })
}

func TestConfigureRejectsUnknownMode(t *testing.T) {
	withTempAptPaths(t)
	if err := Configure(Mode("bogus"), false); err == nil {
		t.Fatal("Configure with an unknown mode returned nil error, want a rejection")
	}
}

func TestConfigureSecurityOnlyDoesNotAddOriginsPattern(t *testing.T) {
	withTempAptPaths(t)
	if err := Configure(ModeSecurityOnly, false); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	dropIn, err := os.ReadFile(dropInPath)
	if err != nil {
		t.Fatalf("reading drop-in: %v", err)
	}
	if strings.Contains(string(dropIn), "Origins-Pattern") {
		t.Errorf("security-only drop-in declares Origins-Pattern, want it to rely on the stock file entirely:\n%s", dropIn)
	}
	if !strings.Contains(string(dropIn), `Automatic-Reboot "false"`) {
		t.Errorf("drop-in = %q, want Automatic-Reboot \"false\"", dropIn)
	}
}

func TestConfigureFullModeAddsOriginsPattern(t *testing.T) {
	withTempAptPaths(t)
	if err := Configure(ModeFull, true); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	dropIn, err := os.ReadFile(dropInPath)
	if err != nil {
		t.Fatalf("reading drop-in: %v", err)
	}
	content := string(dropIn)
	if !strings.Contains(content, "Origins-Pattern") {
		t.Errorf("full-mode drop-in has no Origins-Pattern block:\n%s", content)
	}
	if !strings.Contains(content, "${distro_id}") || !strings.Contains(content, "${distro_codename}") {
		t.Errorf("full-mode Origins-Pattern doesn't use apt's own distro template variables:\n%s", content)
	}
	if !strings.Contains(content, `Automatic-Reboot "true"`) {
		t.Errorf("drop-in = %q, want Automatic-Reboot \"true\"", content)
	}
}

func TestConfigureEnablesPeriodicTimer(t *testing.T) {
	withTempAptPaths(t)
	if err := Configure(ModeSecurityOnly, false); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	auto, err := os.ReadFile(autoUpgradesPath)
	if err != nil {
		t.Fatalf("reading %s: %v", autoUpgradesPath, err)
	}
	if !periodicRe.MatchString(string(auto)) {
		t.Fatalf("20auto-upgrades = %q, want a parseable APT::Periodic::Unattended-Upgrade line", auto)
	}
	if m := periodicRe.FindStringSubmatch(string(auto)); m[1] != "1" {
		t.Errorf("APT::Periodic::Unattended-Upgrade = %q, want \"1\" after Configure", m[1])
	}
}

func TestDisableTurnsOffPeriodicAndRemovesDropIn(t *testing.T) {
	withTempAptPaths(t)
	if err := Configure(ModeFull, true); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if err := Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}

	auto, err := os.ReadFile(autoUpgradesPath)
	if err != nil {
		t.Fatalf("reading %s: %v", autoUpgradesPath, err)
	}
	if m := periodicRe.FindStringSubmatch(string(auto)); len(m) != 2 || m[1] != "0" {
		t.Errorf("APT::Periodic::Unattended-Upgrade after Disable = %v, want \"0\"", m)
	}
	if _, err := os.Stat(dropInPath); !os.IsNotExist(err) {
		t.Errorf("drop-in still exists after Disable (stat err = %v), want it removed", err)
	}
}

func TestDisableWithNoDropInIsNotAnError(t *testing.T) {
	withTempAptPaths(t)
	// Never configured -- dropInPath doesn't exist yet.
	if err := Disable(); err != nil {
		t.Fatalf("Disable with no prior Configure: %v", err)
	}
}

func TestGetStatusNotInstalled(t *testing.T) {
	withTempAptPaths(t)
	isInstalled = func() bool { return false }

	status := GetStatus()
	if status.Installed {
		t.Error("GetStatus().Installed = true, want false")
	}
	if status.Enabled || status.AutoReboot || status.Mode != ModeSecurityOnly {
		t.Errorf("GetStatus() when not installed = %+v, want the zero-ish default", status)
	}
}

func TestGetStatusRoundTripsConfigure(t *testing.T) {
	withTempAptPaths(t)
	if err := Configure(ModeFull, true); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	status := GetStatus()
	want := Status{Installed: true, Enabled: true, Mode: ModeFull, AutoReboot: true}
	if status != want {
		t.Errorf("GetStatus() = %+v, want %+v", status, want)
	}
}

func TestGetStatusReflectsDisable(t *testing.T) {
	withTempAptPaths(t)
	if err := Configure(ModeFull, true); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if err := Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	status := GetStatus()
	want := Status{Installed: true, Enabled: false, Mode: ModeSecurityOnly, AutoReboot: false}
	if status != want {
		t.Errorf("GetStatus() after Disable = %+v, want %+v", status, want)
	}
}
