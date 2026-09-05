package wireproxy

import (
	"context"
	"fmt"
	"os"
)

// Install places the pinned warpwp/warp-wireproxy-native.sh at the absolute
// paths warpwp itself expects, then runs the real provisioning: OS packages,
// a WARP registration, wireproxy as a systemd service, and a default cron
// scheduler (warpwp's own --install default; see internal/wireproxy's own
// package doc for what this actually touches on the host). Already-installed
// is not a no-op the way Tor/AdGuard/Psiphon's Install is -- warpwp --install
// is itself idempotent (update/repair-in-place) and admins may want to
// re-run it deliberately, so this always re-places the pinned scripts and
// re-invokes --install rather than gating on IsInstalled first.
func Install(ctx context.Context) error {
	if err := os.WriteFile(managerBinPath, managerScript, 0o755); err != nil {
		return fmt.Errorf("placing %s: %w", managerBinPath, err)
	}
	if err := os.WriteFile(nativeBinPath, nativeScript, 0o755); err != nil {
		return fmt.Errorf("placing %s: %w", nativeBinPath, err)
	}
	_, err := run(ctx, "--install")
	return err
}

// Uninstall runs warpwp's own safe removal (--remove): stops and disables
// the wireproxy service, removes the cron/timer scheduler and this project's
// own files under /etc/wireguard, but leaves warpwp itself in place (its own
// design -- an admin can re-Install without re-fetching it) and does not
// touch unrelated WARP tooling. Deliberately not --purge, which also chases
// down wgcf/warp-cli/fscarmen traces -- a much larger blast radius this
// package has no way to scope down to just what it installed.
func Uninstall(ctx context.Context) error {
	_, err := run(ctx, "--remove")
	return err
}
