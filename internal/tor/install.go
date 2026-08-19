package tor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// installTimeout bounds a single package-manager invocation. Generous
// because a fresh VPS's first index refresh can genuinely take a while on a
// slow mirror, but still finite so a wedged package manager can't hang the
// request forever.
const installTimeout = 180 * time.Second

// packageManager describes how to drive one package manager non-interactively.
// updateArgs is nil for managers that need no separate index-refresh step.
type packageManager struct {
	bin         string
	updateArgs  []string
	installArgs []string
	removeArgs  []string
}

var packageManagers = []packageManager{
	{bin: "apt-get", updateArgs: []string{"update", "-qq"}, installArgs: []string{"install", "-y", "tor"}, removeArgs: []string{"remove", "-y", "tor"}},
	{bin: "dnf", installArgs: []string{"install", "-y", "tor"}, removeArgs: []string{"remove", "-y", "tor"}},
	{bin: "yum", installArgs: []string{"install", "-y", "tor"}, removeArgs: []string{"remove", "-y", "tor"}},
	{bin: "pacman", installArgs: []string{"-Sy", "--noconfirm", "tor"}, removeArgs: []string{"-R", "--noconfirm", "tor"}},
	{bin: "apk", updateArgs: []string{"update"}, installArgs: []string{"add", "tor"}, removeArgs: []string{"del", "tor"}},
	{bin: "zypper", installArgs: []string{"install", "-y", "tor"}, removeArgs: []string{"remove", "-y", "tor"}},
}

// detectPackageManager returns the first supported package manager found on
// PATH, checked in the fixed order above, or ok=false if none is.
func detectPackageManager() (packageManager, bool) {
	for _, pm := range packageManagers {
		if _, err := exec.LookPath(pm.bin); err == nil {
			return pm, true
		}
	}
	return packageManager{}, false
}

// run executes one package-manager subcommand non-interactively.
// DEBIAN_FRONTEND is harmless to set for managers that ignore it.
func (pm packageManager) run(args []string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), installTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, pm.bin, args...)
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func noPackageManagerErr(verb string) error {
	return fmt.Errorf("no supported package manager found (tried apt-get, dnf, yum, pacman, apk, zypper) -- %s tor manually", verb)
}

// Install installs the tor package via whichever supported package manager
// is available. A no-op when tor is already on PATH.
func Install() error {
	if IsAvailable() {
		return nil
	}
	pm, ok := detectPackageManager()
	if !ok {
		return noPackageManagerErr("install")
	}
	if pm.updateArgs != nil {
		if out, err := pm.run(pm.updateArgs); err != nil {
			// A stale/unreachable index shouldn't block the install attempt
			// itself -- the install command below will fail on its own if
			// the package genuinely can't be found.
			logger.Warningf("tor: %s %v failed, continuing to install anyway: %v: %s", pm.bin, pm.updateArgs, err, strings.TrimSpace(out))
		}
	}
	out, err := pm.run(pm.installArgs)
	if err != nil {
		return fmt.Errorf("%s %v: %w: %s", pm.bin, pm.installArgs, err, strings.TrimSpace(out))
	}
	if !IsAvailable() {
		return fmt.Errorf("%s reported success but no tor binary is on PATH afterwards", pm.bin)
	}
	return nil
}

// Uninstall stops the managed daemon (if running) and removes the tor
// package via whichever supported package manager is available. A no-op
// when tor is already absent from PATH.
func Uninstall() error {
	GetManager().StopAll()
	if !IsAvailable() {
		return nil
	}
	pm, ok := detectPackageManager()
	if !ok {
		return noPackageManagerErr("remove")
	}
	out, err := pm.run(pm.removeArgs)
	if err != nil {
		return fmt.Errorf("%s %v: %w: %s", pm.bin, pm.removeArgs, err, strings.TrimSpace(out))
	}
	return nil
}
