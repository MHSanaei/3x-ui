// Phase 3.5: restoring each opted-in peer's distinct public IPv6 source
// identity for peer-initiated outbound connections. The retired
// kernel-module architecture used NDP-proxying (ip -6 neigh add proxy) to
// hand inbound traffic off to a real awg<N> kernel interface — this path has
// no such interface at all (the tunnel lives entirely inside an in-process
// gVisor netstack), so there is nothing for NDP-proxying to forward into.
// Scoped to what this path actually needs — a peer's own outbound
// connections carrying a distinct source address, not unsolicited inbound
// connections toward the peer (that's the separate, not-yet-built Phase
// 3.6 port-forwarding) — a host-owned address alias is sufficient and
// simpler: once the kernel genuinely owns the address, Xray's freedom
// outbound can bind an egress socket to it, and return traffic lands on a
// normal, locally-owned address with no forwarding or NDP-proxy involved.
package amneziawgnet

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// v6Alias is one host-owned IPv6 address alias this package manages, always
// applied as a /128 regardless of whatever prefix width the peer's own
// AllowedIPs entry happens to use.
type v6Alias struct {
	Addr  string
	Iface string
}

// effectiveIPv6ExternalInterface returns IPv6ExternalInterface if the admin
// set one, falling back to ExternalInterface — matches the frontend's own
// ipv6ExternalInterfaceHint copy ("Leave empty to reuse External
// Interface") and the retired kernel-module PostUp's identical fallback.
func effectiveIPv6ExternalInterface(inst amneziawg.Instance) string {
	if inst.IPv6ExternalInterface != "" {
		return inst.IPv6ExternalInterface
	}
	return inst.ExternalInterface
}

// V6AliasesActive reports whether inst is fully configured for per-peer IPv6
// identity. The Xray-side v6 egress injector must use this exact gate too —
// see xray.go's injectAmneziawgV6Egress — so the two halves can't diverge.
func V6AliasesActive(inst amneziawg.Instance) bool {
	return inst.IPv6Enabled && effectiveIPv6ExternalInterface(inst) != ""
}

// desiredV6Aliases returns the aliases inst wants right now, keyed by peer
// email. Empty whenever inst isn't fully configured for this feature
// (IPv6Enabled false, or no usable interface either way) — deliberately
// what makes "IPv6 toggled off" fall out of diffV6Aliases for free, rather
// than a separate branch anywhere else.
func desiredV6Aliases(inst amneziawg.Instance) map[string]v6Alias {
	out := map[string]v6Alias{}
	if !V6AliasesActive(inst) {
		return out
	}
	iface := effectiveIPv6ExternalInterface(inst)
	for _, p := range inst.Peers {
		if p.Email == "" {
			continue
		}
		if addr := amneziawg.FirstIPv6(p.AllowedIPs); addr != "" {
			out[p.Email] = v6Alias{Addr: addr, Iface: iface}
		}
	}
	return out
}

// diffV6Aliases returns the ip -6 addr add/del calls needed to move the
// host from oldInst's alias set to newInst's. Pass amneziawg.Instance{} as
// oldInst for "nothing was aliased before" (a brand new instance) and as
// newInst for "tear down entirely" (Remove/StopAll/Reconcile's stop-loop).
// A peer whose alias is unchanged appears in neither slice — the common
// case on every steady-state reconcile tick, so a healthy system issues no
// exec calls at all most of the time.
func diffV6Aliases(oldInst, newInst amneziawg.Instance) (add, remove []v6Alias) {
	oldSet, newSet := desiredV6Aliases(oldInst), desiredV6Aliases(newInst)
	for email, oldAlias := range oldSet {
		if newAlias, ok := newSet[email]; ok && newAlias == oldAlias {
			continue
		}
		remove = append(remove, oldAlias)
	}
	for email, newAlias := range newSet {
		if oldAlias, ok := oldSet[email]; ok && oldAlias == newAlias {
			continue
		}
		add = append(add, newAlias)
	}
	return add, remove
}

// runIP is the seam tests swap to assert exact invocations without a real
// ip binary — this package has no internal/database dependency, so
// everything except this var's real invocation builds and unit-tests fine
// even on a non-Linux dev machine; the real command is verified manually
// against a Linux VPS, matching this project's established verification
// pattern for other OS-effecting AmneziaWG changes.
var runIP = func(ctx context.Context, args ...string) (stderr string, err error) {
	cmd := exec.CommandContext(ctx, "ip", args...)
	var buf bytes.Buffer
	cmd.Stderr = &buf
	err = cmd.Run()
	return buf.String(), err
}

const ipCommandTimeout = 3 * time.Second

// applyV6Aliases runs every add before any remove, so a peer whose address
// changed is never briefly unaliased (briefly having both old and new
// aliased at once is harmless). Never surfaces an error — an alias failing
// only narrows that one peer's own outbound-source-identity feature, never
// a reason to fail the tunnel or its SOCKS5 relay.
func applyV6Aliases(add, remove []v6Alias) {
	for _, a := range add {
		addV6Alias(a)
	}
	for _, a := range remove {
		removeV6Alias(a)
	}
}

func addV6Alias(a v6Alias) {
	ctx, cancel := context.WithTimeout(context.Background(), ipCommandTimeout)
	defer cancel()
	// nodad: this address is a specific peer's own admin-assigned identity,
	// nothing else on the link should ever claim it, so the ~1s Duplicate
	// Address Detection window before the kernel would otherwise mark it
	// usable is pure latency with no real collision to detect.
	stderr, err := runIP(ctx, "-6", "addr", "add", a.Addr+"/128", "dev", a.Iface, "nodad")
	if err == nil {
		logger.Infof("amneziawgnet: aliased IPv6 address %s onto %s", a.Addr, a.Iface)
		return
	}
	if strings.Contains(stderr, "File exists") {
		// Already the desired end state -- most commonly hit once, harmlessly,
		// right after an ungraceful panel restart (the OS-level alias from
		// before the crash outlives the process; the in-memory managed map
		// doesn't).
		return
	}
	logger.Warningf("amneziawgnet: alias IPv6 address %s onto %s: %v (%s)", a.Addr, a.Iface, err, strings.TrimSpace(stderr))
}

func removeV6Alias(a v6Alias) {
	ctx, cancel := context.WithTimeout(context.Background(), ipCommandTimeout)
	defer cancel()
	stderr, err := runIP(ctx, "-6", "addr", "del", a.Addr+"/128", "dev", a.Iface)
	if err == nil {
		logger.Infof("amneziawgnet: removed IPv6 alias %s from %s", a.Addr, a.Iface)
		return
	}
	if strings.Contains(stderr, "Cannot assign requested address") || strings.Contains(stderr, "Cannot find device") {
		// Already gone (the address itself, or the whole interface) -- for a
		// delete, the desired end state ("not aliased here") already holds.
		return
	}
	logger.Warningf("amneziawgnet: remove IPv6 alias %s from %s: %v (%s)", a.Addr, a.Iface, err, strings.TrimSpace(stderr))
}
