package amneziawgnet

import (
	"context"
	"errors"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
)

func peerWithIPs(email string, ips ...string) amneziawg.Peer {
	return amneziawg.Peer{Email: email, PublicKey: "pub-" + email, AllowedIPs: ips}
}

func instV6(enabled bool, extIface, v6ExtIface string, peers ...amneziawg.Peer) amneziawg.Instance {
	return amneziawg.Instance{
		Id:                    1,
		IPv6Enabled:           enabled,
		ExternalInterface:     extIface,
		IPv6ExternalInterface: v6ExtIface,
		Peers:                 peers,
	}
}

func TestV6AliasesActive(t *testing.T) {
	cases := []struct {
		name string
		inst amneziawg.Instance
		want bool
	}{
		{"enabled with interface", instV6(true, "eth0", "", peerWithIPs("a@x", "fd86::2/128")), true},
		{"enabled, IPv6ExternalInterface only", instV6(true, "", "eth1", peerWithIPs("a@x", "fd86::2/128")), true},
		{"disabled", instV6(false, "eth0", "", peerWithIPs("a@x", "fd86::2/128")), false},
		{"enabled, no interface either way", instV6(true, "", "", peerWithIPs("a@x", "fd86::2/128")), false},
	}
	for _, c := range cases {
		if got := V6AliasesActive(c.inst); got != c.want {
			t.Errorf("%s: V6AliasesActive = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDesiredV6AliasesDisabledOrNoInterfaceReturnsEmpty(t *testing.T) {
	cases := []struct {
		name string
		inst amneziawg.Instance
	}{
		{"IPv6Enabled false", instV6(false, "", "eth0", peerWithIPs("a@x", "fd86::2/128"))},
		{"no interface either way", instV6(true, "", "", peerWithIPs("a@x", "fd86::2/128"))},
	}
	for _, c := range cases {
		if got := desiredV6Aliases(c.inst); len(got) != 0 {
			t.Errorf("%s: desiredV6Aliases = %v, want empty", c.name, got)
		}
	}
}

func TestDesiredV6AliasesFallsBackToExternalInterface(t *testing.T) {
	inst := instV6(true, "eth0", "", peerWithIPs("a@x", "fd86::2/128"))
	got := desiredV6Aliases(inst)
	if got["a@x"].Iface != "eth0" {
		t.Fatalf("expected fallback to ExternalInterface eth0, got %+v", got)
	}

	inst2 := instV6(true, "eth0", "eth1", peerWithIPs("a@x", "fd86::2/128"))
	got2 := desiredV6Aliases(inst2)
	if got2["a@x"].Iface != "eth1" {
		t.Fatalf("expected IPv6ExternalInterface eth1 to win over ExternalInterface, got %+v", got2)
	}
}

func TestDesiredV6AliasesSkipsPeersWithoutEmailOrV6Address(t *testing.T) {
	inst := instV6(true, "eth0", "",
		peerWithIPs("", "fd86::2/128"),    // no email
		peerWithIPs("b@x", "10.8.1.2/32"), // v4 only, no v6
		peerWithIPs("c@x", "fd86::3/128"), // qualifies
	)
	got := desiredV6Aliases(inst)
	if len(got) != 1 {
		t.Fatalf("desiredV6Aliases = %+v, want exactly one entry (c@x)", got)
	}
	if _, ok := got["c@x"]; !ok {
		t.Fatalf("desiredV6Aliases = %+v, want c@x present", got)
	}
}

func TestDiffV6AliasesNoOpWhenUnchanged(t *testing.T) {
	inst := instV6(true, "eth0", "", peerWithIPs("a@x", "fd86::2/128"))
	add, remove := diffV6Aliases(inst, inst)
	if len(add) != 0 || len(remove) != 0 {
		t.Fatalf("expected no-op for an unchanged instance, got add=%v remove=%v", add, remove)
	}
}

func TestDiffV6AliasesBrandNewInstanceIsAddOnly(t *testing.T) {
	newInst := instV6(true, "eth0", "", peerWithIPs("a@x", "fd86::2/128"), peerWithIPs("b@x", "fd86::3/128"))
	add, remove := diffV6Aliases(amneziawg.Instance{}, newInst)
	if len(remove) != 0 {
		t.Fatalf("expected no removals for a brand new instance, got %v", remove)
	}
	if len(add) != 2 {
		t.Fatalf("expected both peers added, got %v", add)
	}
}

func TestDiffV6AliasesTornDownInstanceIsRemoveOnly(t *testing.T) {
	oldInst := instV6(true, "eth0", "", peerWithIPs("a@x", "fd86::2/128"), peerWithIPs("b@x", "fd86::3/128"))
	add, remove := diffV6Aliases(oldInst, amneziawg.Instance{})
	if len(add) != 0 {
		t.Fatalf("expected no adds when tearing down, got %v", add)
	}
	if len(remove) != 2 {
		t.Fatalf("expected both peers removed, got %v", remove)
	}
}

func TestDiffV6AliasesIPv6EnabledToggledOffRemovesAllAddsNone(t *testing.T) {
	oldInst := instV6(true, "eth0", "", peerWithIPs("a@x", "fd86::2/128"))
	newInst := instV6(false, "eth0", "", peerWithIPs("a@x", "fd86::2/128")) // same peers, feature disabled
	add, remove := diffV6Aliases(oldInst, newInst)
	if len(add) != 0 {
		t.Fatalf("expected no adds when IPv6Enabled is toggled off, got %v", add)
	}
	if len(remove) != 1 {
		t.Fatalf("expected the previously-aliased peer removed, got %v", remove)
	}
}

func TestDiffV6AliasesAddressChangeForSamePeerIsRemoveOldAddNew(t *testing.T) {
	oldInst := instV6(true, "eth0", "", peerWithIPs("a@x", "fd86::2/128"))
	newInst := instV6(true, "eth0", "", peerWithIPs("a@x", "fd86::99/128"))
	add, remove := diffV6Aliases(oldInst, newInst)
	if len(add) != 1 || add[0].Addr != "fd86::99" {
		t.Fatalf("expected new address added, got %v", add)
	}
	if len(remove) != 1 || remove[0].Addr != "fd86::2" {
		t.Fatalf("expected old address removed, got %v", remove)
	}
}

func TestDiffV6AliasesInterfaceChangeReAliasesUnchangedPeers(t *testing.T) {
	oldInst := instV6(true, "eth0", "", peerWithIPs("a@x", "fd86::2/128"))
	newInst := instV6(true, "eth1", "", peerWithIPs("a@x", "fd86::2/128")) // same address, interface moved
	add, remove := diffV6Aliases(oldInst, newInst)
	if len(add) != 1 || add[0].Iface != "eth1" {
		t.Fatalf("expected re-add on the new interface, got %v", add)
	}
	if len(remove) != 1 || remove[0].Iface != "eth0" {
		t.Fatalf("expected removal from the old interface, got %v", remove)
	}
}

func TestDiffV6AliasesPeerRemovedFromInstanceIsRemoveOnly(t *testing.T) {
	oldInst := instV6(true, "eth0", "", peerWithIPs("a@x", "fd86::2/128"), peerWithIPs("b@x", "fd86::3/128"))
	newInst := instV6(true, "eth0", "", peerWithIPs("a@x", "fd86::2/128")) // b@x removed
	add, remove := diffV6Aliases(oldInst, newInst)
	if len(add) != 0 {
		t.Fatalf("expected no adds, got %v", add)
	}
	if len(remove) != 1 || remove[0].Addr != "fd86::3" {
		t.Fatalf("expected only b@x's address removed, got %v", remove)
	}
}

// --- exec-layer tests: swap runIP, never invoke a real ip binary ---

func withFakeRunIP(t *testing.T, fn func(ctx context.Context, args ...string) (string, error)) *[][]string {
	t.Helper()
	var calls [][]string
	orig := runIP
	runIP = func(ctx context.Context, args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		return fn(ctx, args...)
	}
	t.Cleanup(func() { runIP = orig })
	return &calls
}

func TestAddV6AliasPassesExpectedArgs(t *testing.T) {
	calls := withFakeRunIP(t, func(ctx context.Context, args ...string) (string, error) {
		return "", nil
	})
	addV6Alias(v6Alias{Addr: "fd86::2", Iface: "eth0"})
	if len(*calls) != 1 {
		t.Fatalf("expected exactly one runIP call, got %d", len(*calls))
	}
	want := []string{"-6", "addr", "add", "fd86::2/128", "dev", "eth0", "nodad"}
	got := (*calls)[0]
	if len(got) != len(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args = %v, want %v", got, want)
		}
	}
}

func TestAddV6AliasFileExistsIsSwallowed(t *testing.T) {
	withFakeRunIP(t, func(ctx context.Context, args ...string) (string, error) {
		return "RTNETLINK answers: File exists", errors.New("exit status 2")
	})
	// Must not panic and must return normally -- there is nothing else to
	// assert on since addV6Alias has no return value, matching this
	// codebase's existing best-effort exec-call conventions (no test in
	// this repo asserts on logger output for a swallowed vs. warned
	// classification; see internal/web/service/server.go's own untested
	// exec.CommandContext call sites).
	addV6Alias(v6Alias{Addr: "fd86::2", Iface: "eth0"})
}

func TestAddV6AliasOtherFailureDoesNotPanic(t *testing.T) {
	withFakeRunIP(t, func(ctx context.Context, args ...string) (string, error) {
		return "RTNETLINK answers: Cannot find device \"eth9\"", errors.New("exit status 1")
	})
	addV6Alias(v6Alias{Addr: "fd86::2", Iface: "eth9"})
}

func TestRemoveV6AliasPassesExpectedArgs(t *testing.T) {
	calls := withFakeRunIP(t, func(ctx context.Context, args ...string) (string, error) {
		return "", nil
	})
	removeV6Alias(v6Alias{Addr: "fd86::2", Iface: "eth0"})
	want := []string{"-6", "addr", "del", "fd86::2/128", "dev", "eth0"}
	got := (*calls)[0]
	if len(got) != len(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args = %v, want %v", got, want)
		}
	}
}

func TestRemoveV6AliasAddressAlreadyGoneIsSwallowed(t *testing.T) {
	withFakeRunIP(t, func(ctx context.Context, args ...string) (string, error) {
		return "RTNETLINK answers: Cannot assign requested address", errors.New("exit status 2")
	})
	removeV6Alias(v6Alias{Addr: "fd86::2", Iface: "eth0"})
}

func TestRemoveV6AliasDeviceAlreadyGoneIsSwallowed(t *testing.T) {
	withFakeRunIP(t, func(ctx context.Context, args ...string) (string, error) {
		return "Cannot find device \"eth0\"", errors.New("exit status 1")
	})
	removeV6Alias(v6Alias{Addr: "fd86::2", Iface: "eth0"})
}

func TestRemoveV6AliasOtherFailureDoesNotPanic(t *testing.T) {
	withFakeRunIP(t, func(ctx context.Context, args ...string) (string, error) {
		return "some unrelated failure", errors.New("exit status 1")
	})
	removeV6Alias(v6Alias{Addr: "fd86::2", Iface: "eth0"})
}

func TestApplyV6AliasesAddsBeforeRemoves(t *testing.T) {
	var order []string
	calls := withFakeRunIP(t, func(ctx context.Context, args ...string) (string, error) {
		if args[2] == "add" {
			order = append(order, "add")
		} else {
			order = append(order, "del")
		}
		return "", nil
	})
	applyV6Aliases(
		[]v6Alias{{Addr: "fd86::99", Iface: "eth0"}},
		[]v6Alias{{Addr: "fd86::2", Iface: "eth0"}},
	)
	if len(*calls) != 2 {
		t.Fatalf("expected exactly 2 calls, got %d", len(*calls))
	}
	if order[0] != "add" || order[1] != "del" {
		t.Fatalf("expected add before del, got order=%v", order)
	}
}
