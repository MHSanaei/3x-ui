package amneziawg

import (
	"fmt"
	"hash/fnv"
)

// EgressPort is the loopback port of the single Xray dokodemo-door bridge
// every RouteThroughXray peer's TPROXY'd traffic lands on, shared across
// every AmneziaWG instance. defaultPostUpDown's TPROXY rules target it, and
// internal/web/service's injectAmneziawgEgress listens on it. A single
// shared bridge — rather than one per peer — keeps this a plain constant
// instead of state two independent reconcile loops would otherwise have to
// agree on at runtime; per-peer distinction happens downstream, in Xray's
// own router, matched by each peer's TPROXY-preserved tunnel source IP (see
// EgressTag).
const EgressPort = 63100

// EgressTag is the tag of that shared bridge inbound in the generated Xray
// config. Routing rules that distinguish peers match against it as their
// inboundTag.
const EgressTag = "amneziawg-egress"

// EgressFwmark and EgressTable are the fwmark and policy-routing table
// TPROXY needs to deliver a routed peer's packets to a local socket even
// though their destination is never one of this host's own addresses.
// Chosen to be distinctive; if either happens to collide with something else
// already using fwmarks/routing tables on the host, change the values here —
// nothing outside this package and its own PostUp/PostDown output depends on
// the actual numbers.
const (
	EgressFwmark = 0x2377
	EgressTable  = 87
)

// routeEgressComment returns a short, shell-safe iptables comment tag for one
// peer's TPROXY rule, so PostDown removes exactly what PostUp added
// regardless of ordering. Derived from a hash of the peer's email for the
// same reason portForwardComment is: email is admin/API-supplied free text
// that ends up embedded in a shell-executed PostUp/PostDown line, and a hash
// can never carry a shell metacharacter through.
func routeEgressComment(email string) string {
	if email == "" {
		return "awg-route"
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(email))
	return fmt.Sprintf("awg-route-%08x", h.Sum32())
}

// routeEgressLines returns the PostUp ("-A") or PostDown ("-D") mangle-table
// TPROXY lines that redirect one peer's traffic — matched by its tunnel
// source IP, arriving on tunIface — into the shared Xray bridge. Both TCP and
// UDP are covered since RouteThroughXray means "this peer's traffic", not a
// specific protocol or port. Returns nil when clientIP is empty.
func routeEgressLines(action, tunIface, clientIP, email string) []string {
	clientIP = stripCIDRMask(clientIP)
	if clientIP == "" {
		return nil
	}
	comment := routeEgressComment(email)
	lines := make([]string, 0, 2)
	for _, proto := range []string{"tcp", "udp"} {
		lines = append(lines, fmt.Sprintf(
			"iptables -t mangle %s PREROUTING -i %s -s %s -p %s -m comment --comment %s -j TPROXY --on-port %d --on-ip 127.0.0.1 --tproxy-mark %#x/%#x",
			action, tunIface, clientIP, proto, comment, EgressPort, EgressFwmark, EgressFwmark,
		))
	}
	return lines
}
