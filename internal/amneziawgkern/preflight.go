package amneziawgkern

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
)

// installCommand is what an admin runs to satisfy every host-level blocker at once.
func installCommand() string {
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return "sudo /usr/local/x-ui/x-ui install-awg-kernel"
	}
	return "sudo " + exe + " install-awg-kernel"
}

// BlockerCode identifies one reason kernel mode cannot run.
type BlockerCode string

const (
	BlockerNotRoot       BlockerCode = "root"
	BlockerToolsMissing  BlockerCode = "tools"
	BlockerModuleMissing BlockerCode = "module"
	BlockerNftMissing    BlockerCode = "nft"
	BlockerIPv6          BlockerCode = "ipv6"
	BlockerPortForward   BlockerCode = "portForwarding"
	BlockerRouteXray     BlockerCode = "routeThroughXray"
	BlockerListenAddress BlockerCode = "listenAddress"
)

// Blocker is one unmet requirement.
type Blocker struct {
	Code   BlockerCode `json:"code" example:"tools"`
	Detail string      `json:"detail,omitempty" example:"awg"`
}

// Readiness is the host's ability to run any kernel-mode inbound at all.
type Readiness struct {
	Ready          bool      `json:"ready" example:"false"`
	Blockers       []Blocker `json:"blockers" example:"[{\"code\":\"tools\",\"detail\":\"awg\"}]"`
	InstallCommand string    `json:"installCommand" example:"sudo /usr/local/x-ui/x-ui install-awg-kernel"`
}

// readinessTTL keeps the form's polling from re-running modprobe on every
// open. Short enough that installing the module shows up almost immediately.
const readinessTTL = 10 * time.Second

var (
	readinessMu     sync.Mutex
	readinessCached Readiness
	readinessAt     time.Time
)

// HostReadiness reports host requirements independently of a particular inbound.
// The UI can check it before an inbound exists.
func HostReadiness() Readiness {
	readinessMu.Lock()
	defer readinessMu.Unlock()

	if !readinessAt.IsZero() && time.Since(readinessAt) < readinessTTL {
		return readinessCached
	}
	readinessCached = probeHost()
	readinessAt = time.Now()
	return readinessCached
}

// resetReadinessCache exists for tests; production code has no reason to force
// a re-probe.
func resetReadinessCache() {
	readinessMu.Lock()
	defer readinessMu.Unlock()
	readinessAt = time.Time{}
	readinessCached = Readiness{}
}

func probeHost() Readiness {
	r := Readiness{Blockers: []Blocker{}, InstallCommand: installCommand()}

	if os.Geteuid() != 0 {
		r.Blockers = append(r.Blockers, Blocker{Code: BlockerNotRoot})
	}
	for _, bin := range []string{"awg", "ip"} {
		if _, err := exec.LookPath(bin); err != nil {
			r.Blockers = append(r.Blockers, Blocker{Code: BlockerToolsMissing, Detail: bin})
		}
	}
	if !moduleAvailable() {
		r.Blockers = append(r.Blockers, Blocker{Code: BlockerModuleMissing})
	}
	// NAT is the only egress kernel mode implements today, so nft is required unconditionally.
	if _, err := exec.LookPath("nft"); err != nil {
		r.Blockers = append(r.Blockers, Blocker{Code: BlockerNftMissing})
	}

	r.Ready = len(r.Blockers) == 0
	return r
}

// Preflight reports host and inbound blockers before kernel mode is enabled.
// This protects API saves when the frontend readiness check is stale.
func Preflight(inst amneziawg.Instance, routeThroughXray bool) error {
	blockers := append([]Blocker(nil), HostReadiness().Blockers...)
	blockers = append(blockers, instanceFeatureBlockers(inst, routeThroughXray)...)
	if len(blockers) == 0 {
		return nil
	}
	return fmt.Errorf("kernel mode unavailable: %s", strings.Join(describe(blockers), "; "))
}

// ValidateInstanceFeatures rejects kernel features this backend cannot serve.
func ValidateInstanceFeatures(inst amneziawg.Instance, routeThroughXray bool) error {
	blockers := instanceFeatureBlockers(inst, routeThroughXray)
	if len(blockers) == 0 {
		return nil
	}
	return fmt.Errorf("kernel mode unavailable: %s", strings.Join(describe(blockers), "; "))
}

func instanceFeatureBlockers(inst amneziawg.Instance, routeThroughXray bool) []Blocker {
	var blockers []Blocker
	if instanceWantsIPv6(inst) {
		blockers = append(blockers, Blocker{Code: BlockerIPv6})
	}
	if instanceWantsPortForwarding(inst) {
		blockers = append(blockers, Blocker{Code: BlockerPortForward})
	}
	if routeThroughXray {
		blockers = append(blockers, Blocker{Code: BlockerRouteXray})
	}
	if listenNeedsPinnedBind(inst.Listen) {
		blockers = append(blockers, Blocker{Code: BlockerListenAddress, Detail: inst.Listen})
	}
	return blockers
}

// describe renders blockers for the error a direct API caller sees. The panel
// UI never reaches this path -- it translates the codes itself.
func describe(blockers []Blocker) []string {
	out := make([]string, 0, len(blockers))
	for _, b := range blockers {
		switch b.Code {
		case BlockerNotRoot:
			out = append(out, "the panel is not running as root, so it cannot create a network interface")
		case BlockerToolsMissing:
			out = append(out, fmt.Sprintf("%s not found in PATH (install amneziawg-tools and iproute2)", b.Detail))
		case BlockerModuleMissing:
			out = append(out, "the amneziawg kernel module is not installed (run "+installCommand()+")")
		case BlockerNftMissing:
			out = append(out, "nft not found in PATH (install nftables); it is needed for the tunnel's NAT rules")
		case BlockerIPv6:
			out = append(out, "per-peer IPv6 egress is not supported in kernel mode yet; disable IPv6 on this inbound first")
		case BlockerPortForward:
			out = append(out, "per-client port forwarding is not supported in kernel mode yet; clear the forwarded ports first")
		case BlockerRouteXray:
			out = append(out, "routeThroughXray has no kernel-mode implementation; it would leave the tunnel with no egress path -- turn it off first")
		case BlockerListenAddress:
			out = append(out, fmt.Sprintf("kernel mode cannot bind to the configured listen address %q; clear it or use a wildcard", b.Detail))
		default:
			out = append(out, string(b.Code))
		}
	}
	return out
}

// moduleAvailable reports a loaded or installed module; the dry run keeps a
// readiness GET from loading kernel code, Ensure loads it when it is needed.
func moduleAvailable() bool {
	if _, err := os.Stat("/sys/module/amneziawg"); err == nil {
		return true
	}
	if _, err := exec.LookPath("modprobe"); err != nil {
		return false
	}
	_, err := run("", "modprobe", "--dry-run", "--quiet", "amneziawg")
	return err == nil
}

func instanceWantsIPv6(inst amneziawg.Instance) bool {
	if inst.IPv6Enabled {
		return true
	}
	for _, a := range inst.Address {
		if strings.Contains(a, ":") {
			return true
		}
	}
	for _, p := range inst.Peers {
		for _, a := range p.AllowedIPs {
			if strings.Contains(a, ":") {
				return true
			}
		}
	}
	return false
}

func instanceWantsPortForwarding(inst amneziawg.Instance) bool {
	for _, p := range inst.Peers {
		if strings.TrimSpace(p.ForwardedPorts) != "" {
			return true
		}
	}
	return false
}
