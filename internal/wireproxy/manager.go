// Package wireproxy manages the panel's WARP-via-wireproxy sidecar: Cloudflare
// WARP reached through wireproxy (github.com/pufferffish/wireproxy), a
// userspace WireGuard-to-SOCKS5 bridge, exposed at 127.0.0.1:40000 for Xray
// to use as an outbound. Distinct from the existing WarpService
// (internal/web/service/integration/warp.go), which registers a WARP device
// against Cloudflare's own API and builds a native WireGuard outbound with no
// external process at all -- this package instead runs an actual sidecar,
// the same general shape as internal/tor and internal/psiphon, and adds
// something neither of those give: continuous, self-healing endpoint health
// checking (Cloudflare WARP endpoints periodically go bad; this notices and
// rotates automatically instead of leaving a dead outbound in place).
//
// Unlike Tor/AdGuard/Psiphon, this package does not own a process directly.
// It wraps github.com/kuzzrus/WARP_WireProxy_Manager (warpwp), the admin's
// own already-built, already-maintained installer and manager for exactly
// this setup -- a full tool in its own right (endpoint scanning with its own
// stability metric, a cron-or-systemd-timer scheduler, doctor-style
// diagnostics, a routing guard against a real footgun: a stray `wg-quick up
// warp` turning the whole VPS into a WARP client and breaking its own
// inbound connectivity). Re-deriving any of that in Go would be worse than
// what already exists, so this package is deliberately thin: place the
// vendored scripts, shell out to warpwp's own CLI for every real action, and
// parse `warpwp --status-json` -- already a rich, structured status blob --
// instead of inventing a second source of truth.
//
// warpwp.sh/warp-wireproxy-native.sh are vendored via go:embed at a pinned
// commit (see Install in install.go), not fetched live from "main" --
// REPO_RAW inside the vendored warpwp.sh is itself repointed at that same
// commit so even its own internal update_local_scripts call can't silently
// pull different content than what this panel build shipped and reviewed.
// That said, warpwp's own --install still re-fetches both scripts from that
// pinned URL as its first step (update_local_scripts runs unconditionally),
// so Install is not network-free the way a pure go:embed read would be --
// it is pinned, not offline, the same tradeoff install.go's binary download
// makes for Psiphon.
//
// This carries real system-level blast radius none of Tor/AdGuard/Psiphon
// do: warpwp runs as root, installs OS packages via the host's own package
// manager, and writes systemd units, cron/timer files, and WireGuard/routing
// state. Install and Uninstall say so plainly to the admin in the frontend's
// own confirm copy, not just here.
package wireproxy

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
)

// SocksPort is fixed by the vendored warpwp.sh itself (SOCKS_PORT="40000"),
// not something this package chooses independently -- changing it here alone
// would not change what warpwp actually binds.
const SocksPort = 40000

// serviceName is the systemd unit warp-wireproxy-native.sh installs.
const serviceName = "wireproxy"

// managerBinPath/nativeBinPath are hardcoded absolute paths because the
// vendored scripts hardcode them too (MANAGER_BIN/NATIVE_BIN in warpwp.sh) --
// every internal warpwp codepath that shells out to the native script or
// re-execs itself assumes these exact locations, so this package cannot
// relocate them under GetBinFolderPath() the way Tor/AdGuard/Psiphon do.
// Vars, not consts, purely so tests can point them at a throwaway temp file
// instead of the real /usr/local/bin.
var (
	managerBinPath = "/usr/local/bin/warpwp"
	nativeBinPath  = "/usr/local/bin/warp-wireproxy-native.sh"
)

//go:embed warpwp.sh
var managerScript []byte

//go:embed warp-wireproxy-native.sh
var nativeScript []byte

// IsInstalled reports whether the manager script is in place. Mirrors
// adguard/psiphon's IsInstalled -- the settings UI checks this before
// offering Start/Status actions.
func IsInstalled() bool {
	info, err := os.Stat(managerBinPath)
	return err == nil && info.Mode().IsRegular()
}

// run shells out to the installed warpwp with args, returning stdout. warpwp
// needs root (need_root in the script) -- the panel's own process already
// runs as root, the same precondition every other host-level action here
// (AmneziaWG, frontproxy certs, etc.) already relies on.
func run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "bash", append([]string{managerBinPath}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("warpwp %v: %w: %s", args, err, bytes.TrimSpace(stderr.Bytes()))
	}
	return stdout.Bytes(), nil
}

// Start starts the wireproxy service directly rather than through warpwp --
// warpwp's own CLI has no "just (re)start the service" action distinct from
// a full --install, and Stop needs the systemctl-direct symmetry anyway.
func Start(ctx context.Context) error {
	if !IsInstalled() {
		return fmt.Errorf("wireproxy is not installed")
	}
	if out, err := exec.CommandContext(ctx, "systemctl", "start", serviceName).CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl start %s: %w: %s", serviceName, err, bytes.TrimSpace(out))
	}
	return nil
}

// Stop stops the wireproxy service, leaving everything warpwp installed
// (systemd units, cron/timer, WARP registration) in place so Start can
// resume quickly -- the same "pause, don't tear down" distinction
// Uninstall's --remove draws versus a full --purge.
func Stop(ctx context.Context) error {
	if out, err := exec.CommandContext(ctx, "systemctl", "stop", serviceName).CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl stop %s: %w: %s", serviceName, err, bytes.TrimSpace(out))
	}
	return nil
}

// RepairMode selects how hard Repair looks for a working Cloudflare
// endpoint, mirroring warpwp's own --check/--quick-scan/--deep-scan.
type RepairMode string

const (
	RepairCheck     RepairMode = "check"
	RepairQuickScan RepairMode = "quick-scan"
	RepairDeepScan  RepairMode = "deep-scan"
)

// Repair re-runs warpwp's own endpoint scan/repair -- the same action its
// cron/timer scheduler already performs periodically, exposed here for an
// admin who does not want to wait for the next tick.
func Repair(ctx context.Context, mode RepairMode) error {
	switch mode {
	case RepairCheck, RepairQuickScan, RepairDeepScan:
	default:
		return fmt.Errorf("unknown repair mode %q", mode)
	}
	_, err := run(ctx, "--"+string(mode))
	return err
}

// FixRouting clears dangerous system-wide WARP routing outside wireproxy's
// own management (see Status.RoutingGuard.Danger and this package's own doc
// comment) without touching wireproxy's own SOCKS5 setup -- the UI exposes
// this specifically so clearing it never requires SSH either.
func FixRouting(ctx context.Context) error {
	_, err := run(ctx, "--fix-routing")
	return err
}
