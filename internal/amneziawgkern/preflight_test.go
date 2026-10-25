package amneziawgkern

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestInstallCommandNamesThePanelBinary: /usr/bin/x-ui is the management shell
// script, whose case list has no install-awg-kernel -- it just opens the menu.
func TestInstallCommandNamesThePanelBinary(t *testing.T) {
	resetReadinessCache()
	got := HostReadiness().InstallCommand

	exe, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable unavailable: %v", err)
	}
	if !strings.Contains(got, exe) {
		t.Errorf("InstallCommand = %q, want it to name the running binary %q", got, exe)
	}
	if !strings.HasSuffix(got, " install-awg-kernel") {
		t.Errorf("InstallCommand = %q, want it to end with the subcommand", got)
	}
}

// TestHostReadinessIsCached keeps the form's polling from re-running modprobe
// on every open.
func TestHostReadinessIsCached(t *testing.T) {
	resetReadinessCache()

	HostReadiness()
	readinessMu.Lock()
	first := readinessAt
	readinessMu.Unlock()
	if first.IsZero() {
		t.Fatal("first call must record a probe time")
	}

	HostReadiness()
	readinessMu.Lock()
	second := readinessAt
	readinessMu.Unlock()
	if !second.Equal(first) {
		t.Error("a call inside the TTL must reuse the cached result, not re-probe")
	}

	// Age the entry past the TTL and confirm it re-probes.
	readinessMu.Lock()
	readinessAt = time.Now().Add(-2 * readinessTTL)
	readinessMu.Unlock()
	HostReadiness()
	readinessMu.Lock()
	third := readinessAt
	readinessMu.Unlock()
	if third.Equal(first) {
		t.Error("an expired entry must be re-probed")
	}
}

// TestPreflightAddsInstanceBlockersToHostOnes: the two kinds are collected in
// one pass so the admin sees everything to fix at once.
func TestPreflightAddsInstanceBlockers(t *testing.T) {
	resetReadinessCache()

	v6 := testInstance()
	v6.IPv6Enabled = true
	err := Preflight(v6, false)
	if err == nil || !strings.Contains(err.Error(), "IPv6") {
		t.Errorf("Preflight must refuse IPv6 inbounds, got %v", err)
	}

	pf := testInstance()
	pf.Peers[0].ForwardedPorts = "80, 443"
	if err := Preflight(pf, false); err == nil || !strings.Contains(err.Error(), "port forwarding") {
		t.Errorf("Preflight must refuse port forwarding, got %v", err)
	}

	v6addr := testInstance()
	v6addr.Peers[0].AllowedIPs = append(v6addr.Peers[0].AllowedIPs, "fd00::2/128")
	if err := Preflight(v6addr, false); err == nil || !strings.Contains(err.Error(), "IPv6") {
		t.Errorf("Preflight must refuse peers with IPv6 AllowedIPs, got %v", err)
	}

	pinned := testInstance()
	pinned.Listen = "203.0.113.10"
	if err := Preflight(pinned, false); err == nil || !strings.Contains(err.Error(), "cannot bind to the configured listen address") {
		t.Errorf("Preflight must refuse a pinned listen address the kernel backend cannot honor, got %v", err)
	}
}

// TestPreflightDoesNotInventInstanceBlockers: a plain instance must only ever
// be refused for host reasons, never for a feature it is not using.
func TestPreflightDoesNotInventInstanceBlockers(t *testing.T) {
	resetReadinessCache()

	if err := Preflight(testInstance(), false); err != nil {
		if strings.Contains(err.Error(), "IPv6") || strings.Contains(err.Error(), "port forwarding") {
			t.Errorf("plain instance must not trip feature gates, got %v", err)
		}
	}
}

// TestPreflightRefusesRouteThroughXray: Render drops the whole NAT block for it
// and nothing replaces it, so the tunnel would come up with no egress path.
func TestPreflightRefusesRouteThroughXray(t *testing.T) {
	resetReadinessCache()

	err := Preflight(testInstance(), true)
	if err == nil || !strings.Contains(err.Error(), "routeThroughXray") {
		t.Errorf("Preflight must refuse routeThroughXray in kernel mode, got %v", err)
	}
	if err := Preflight(testInstance(), false); err != nil && strings.Contains(err.Error(), "routeThroughXray") {
		t.Errorf("routeThroughXray must not be reported when it is off, got %v", err)
	}
}
