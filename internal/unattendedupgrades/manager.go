// Package unattendedupgrades manages the host's own unattended-upgrades
// package (Debian/Ubuntu's standard, already-hardened automatic security
// update tool) rather than having the panel drive apt directly.
//
// The original ask was a dashboard button running the equivalent of
// `apt update && apt upgrade -y && apt autoclean -y && apt clean -y &&
// apt autoremove -y`, no SSH needed. Pushed back with concrete failure
// modes before writing any code: a bare `apt upgrade -y` with no
// DEBIAN_FRONTEND=noninteractive/--force-confdef/--force-confold can hang
// forever on a debconf prompt with nothing to answer it -- exactly the
// scenario a "no SSH needed" button exists to avoid needing SSH for; a
// kernel/systemd upgrade that breaks the panel's own process mid-upgrade
// could leave dpkg half-configured with no working tool left to finish the
// job; auto-rebooting after a kernel update risks a VPS that never comes
// back with no recourse. The chosen answer: lean on unattended-upgrades,
// which already solves all three (never prompts, applies packages one at a
// time so a single failure doesn't take dpkg down with it, and defaults to
// no reboot) instead of re-deriving that hardening in Go.
//
// Deliberately thin, the same shape as internal/wireproxy: ensure the OS
// package is installed, own one small apt.conf.d drop-in for the settings
// this panel actually exposes (mode, auto-reboot), and shell out to the
// real `unattended-upgrade` binary for "run now" -- never reimplementing
// what dpkg/apt already do correctly.
package unattendedupgrades

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

// Mode selects which apt origins unattended-upgrades is allowed to touch.
type Mode string

const (
	// ModeSecurityOnly relies on the distro package's own stock
	// Origins-Pattern in 50unattended-upgrades (already security-only on
	// both Debian and Ubuntu's shipped template) -- this package's own
	// drop-in adds nothing in this mode.
	ModeSecurityOnly Mode = "security"
	// ModeFull additionally allows the regular (non-security) update
	// origin, via this package's own drop-in. An explicit admin choice,
	// never the default.
	ModeFull Mode = "full"
)

const packageName = "unattended-upgrades"

// autoUpgradesPath/dropInPath are vars, not consts, purely so tests can
// point them at a throwaway temp file instead of the real /etc/apt/apt.conf.d
// (the same reason internal/wireproxy's managerBinPath/nativeBinPath are
// vars rather than consts).
var (
	// autoUpgradesPath is the standard toggle file the unattended-upgrades
	// package itself ships a template for. Owned outright here -- fully
	// rewritten, never hand-patched -- so enabling/disabling never depends
	// on guessing what a previous install (or a cloud image's own
	// defaults) left behind.
	autoUpgradesPath = "/etc/apt/apt.conf.d/20auto-upgrades"

	// dropInPath is this panel's own override, named so an admin
	// inspecting /etc/apt/apt.conf.d/ can tell at a glance which file the
	// panel owns, and sorted after the stock 50unattended-upgrades so its
	// scalar setting (Automatic-Reboot) wins over the distro default
	// deterministically.
	dropInPath = "/etc/apt/apt.conf.d/52-3x-ui-unattended-upgrades"
)

// isInstalled is a var so tests can substitute a fake check instead of
// shelling out to the real dpkg-query -- GetStatus's own installed/not-installed
// branch is worth covering without a dpkg on the test host.
var isInstalled = func() bool {
	out, err := exec.Command("dpkg-query", "-W", "-f=${Status}", packageName).Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "install ok installed")
}

// IsInstalled reports whether the unattended-upgrades package is actually
// installed -- checks dpkg's own recorded status string, not just that the
// dpkg-query invocation itself succeeded (which it also does for a
// "deinstall ok config-files" package, i.e. removed but not purged).
func IsInstalled() bool { return isInstalled() }

// noninteractiveEnv is the exact environment this feature exists to
// guarantee is always set -- see this package's own doc comment for why a
// bare `apt-get install -y` alone is not enough.
func noninteractiveEnv() []string {
	return append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
}

// aptGet runs one apt-get subcommand with the noninteractive/force-confdef
// environment this whole feature is built around, bounded by ctx (an
// unreachable mirror hanging apt-get update is exactly the class of hang
// this feature exists to avoid), returning combined output on failure for
// the caller to surface.
func aptGet(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "apt-get", args...)
	cmd.Env = noninteractiveEnv()
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apt-get %v: %w: %s", args, err, bytes.TrimSpace(out.Bytes()))
	}
	return nil
}

// Install installs the unattended-upgrades package if it is not already
// present. A no-op otherwise -- Configure is the action that changes
// settings, so re-running Install on an already-installed system does not
// need to do anything further.
func Install(ctx context.Context) error {
	if IsInstalled() {
		return nil
	}
	if err := aptGet(ctx, "update"); err != nil {
		return err
	}
	return aptGet(ctx, "install", "-y",
		"-o", "Dpkg::Options::=--force-confdef",
		"-o", "Dpkg::Options::=--force-confold",
		packageName,
	)
}

// dropInHeader documents ownership directly in the file apt itself parses,
// not just in this package's comments -- an admin reading
// /etc/apt/apt.conf.d/ by hand should not need to guess where this file
// came from.
const dropInHeader = "// Managed by the 3x-ui panel -- edits here are overwritten the next\n" +
	"// time the panel's unattended-upgrades settings are saved.\n"

// Configure writes this panel's own apt.conf.d drop-in (mode + auto-reboot)
// and enables the periodic timer via autoUpgradesPath. Always rewrites both
// files wholesale -- there is no partial state to merge, since this package
// owns both outright and neither has any content this panel did not put
// there itself.
func Configure(mode Mode, autoReboot bool) error {
	if mode != ModeSecurityOnly && mode != ModeFull {
		return fmt.Errorf("unknown unattended-upgrades mode %q", mode)
	}

	var b strings.Builder
	b.WriteString(dropInHeader)
	rebootValue := "false"
	if autoReboot {
		rebootValue = "true"
	}
	fmt.Fprintf(&b, "Unattended-Upgrade::Automatic-Reboot \"%s\";\n", rebootValue)
	if mode == ModeFull {
		// ${distro_id}/${distro_codename} are apt.conf's own template
		// variables, resolved by unattended-upgrade itself at run time --
		// this file never needs to know which distro it is running on.
		b.WriteString("Unattended-Upgrade::Origins-Pattern {\n")
		b.WriteString("    \"origin=${distro_id},codename=${distro_codename}-updates\";\n")
		b.WriteString("    \"origin=${distro_id},codename=${distro_codename}-proposed-updates\";\n")
		b.WriteString("};\n")
	}
	if err := os.WriteFile(dropInPath, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", dropInPath, err)
	}

	if err := writeAutoUpgrades(true); err != nil {
		return err
	}
	return nil
}

// Disable turns off the periodic timer (APT::Periodic::Unattended-Upgrade
// "0") and removes this panel's own drop-in, reverting to whatever the
// distro's stock 50unattended-upgrades alone would do. Deliberately does
// NOT apt-get remove the package: unattended-upgrades often ships
// pre-installed on cloud images, and removing an OS security tool as a side
// effect of turning off the panel's own management of it would be a much
// bigger surprise than just disabling the timer.
func Disable() error {
	if err := writeAutoUpgrades(false); err != nil {
		return err
	}
	if err := os.Remove(dropInPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", dropInPath, err)
	}
	return nil
}

func writeAutoUpgrades(enabled bool) error {
	value := "0"
	if enabled {
		value = "1"
	}
	content := fmt.Sprintf(
		"APT::Periodic::Update-Package-Lists \"%s\";\nAPT::Periodic::Unattended-Upgrade \"%s\";\n",
		value, value,
	)
	if err := os.WriteFile(autoUpgradesPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", autoUpgradesPath, err)
	}
	return nil
}

// Status is this package's own view of the host's configuration -- always
// re-read from the actual apt.conf.d files on disk, never a cached DB flag,
// so it can never drift from what apt itself will actually do.
type Status struct {
	Installed  bool `json:"installed"`
	Enabled    bool `json:"enabled"`
	Mode       Mode `json:"mode"`
	AutoReboot bool `json:"autoReboot"`
}

var periodicRe = regexp.MustCompile(`APT::Periodic::Unattended-Upgrade\s+"(\d+)"`)

// GetStatus reads the two files this package owns (or the distro's stock
// files, if the panel has never configured this yet) and reports what apt
// will actually do. Never errors -- a missing/unreadable file just reads as
// "not configured this way," matching IsInstalled's own not-installed case.
func GetStatus() Status {
	status := Status{Installed: IsInstalled(), Mode: ModeSecurityOnly}
	if !status.Installed {
		return status
	}
	if data, err := os.ReadFile(autoUpgradesPath); err == nil {
		if m := periodicRe.FindStringSubmatch(string(data)); len(m) == 2 {
			status.Enabled = m[1] != "0"
		}
	}
	if data, err := os.ReadFile(dropInPath); err == nil {
		content := string(data)
		status.AutoReboot = strings.Contains(content, `Automatic-Reboot "true"`)
		if strings.Contains(content, "Origins-Pattern") {
			status.Mode = ModeFull
		}
	}
	return status
}

// logDir is this package's own subdirectory of bin/, matching the
// "sidecar owns a subdirectory of bin/" convention Tor/AdGuard/Psiphon use
// -- for RunNow's own status/log tracking, not unattended-upgrades' own
// /var/log/unattended-upgrades/ (which this package leaves untouched).
func logDir() string { return config.GetBinFolderPath() + "/unattended-upgrades" }

// LogPath is where the most recent RunNow's combined output is kept.
func LogPath() string { return logDir() + "/last-run.log" }

// StatusPath is where RunNow's own terminal outcome is recorded, read back
// by GetRunStatus.
func StatusPath() string { return logDir() + "/last-run-status.json" }
