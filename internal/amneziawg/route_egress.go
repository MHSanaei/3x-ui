package amneziawg

import (
	"fmt"
	"hash/fnv"
)

// EgressBasePort is the first loopback port used for an AmneziaWG inbound's
// own Xray TPROXY bridge. The bridge is opt-in per inbound, gated on
// Instance.RouteThroughXray (off by default): only when it's on does
// defaultPostUpDown's TPROXY rules redirect a peer's traffic there, and only
// then does internal/web/service's injectAmneziawgEgress create the matching
// dokodemo-door inbound, tagged with the AmneziaWG inbound's own real tag so
// it's already selectable in the panel's stock Routing page (the same
// mechanism that already makes an mtproto inbound's own bridge routable
// there — see injectMtprotoEgress). A plain AmneziaWG tunnel with routing
// left off never depends on Xray being up at all. Whether — and where —
// routed traffic actually goes anywhere beyond Xray's default routing is
// entirely up to whatever rules the admin adds on that page; this package
// and injectAmneziawgEgress never generate a routing rule themselves.
//
// EgressPortForInbound derives each inbound's own port deterministically
// from its id, so the two independent reconcile loops (this package's
// PostUp generator and the Xray-config generator, in a different package)
// never have to agree on a runtime-negotiated value.
const EgressBasePort = 63100

// EgressPortForInbound returns the loopback port of one AmneziaWG inbound's
// own Xray TPROXY bridge.
func EgressPortForInbound(inboundID int) int {
	return EgressBasePort + inboundID
}

// EgressFwmark and EgressTable are the fwmark and policy-routing table
// TPROXY needs to deliver a peer's packets to a local socket even though
// their destination is never one of this host's own addresses. Shared by
// every AmneziaWG instance's bridge — only the port differs per instance.
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
// source IP, arriving on tunIface — into that instance's own Xray bridge on
// port. Both TCP and UDP are covered since every peer's whole traffic is
// meant to reach the bridge, not just a specific protocol or port; which
// outbound (if any) it then takes is entirely up to the admin's own Routing
// rules. Returns nil when clientIP is empty.
func routeEgressLines(action, tunIface, clientIP, email string, port int) []string {
	clientIP = stripCIDRMask(clientIP)
	if clientIP == "" {
		return nil
	}
	comment := routeEgressComment(email)
	lines := make([]string, 0, 2)
	for _, proto := range []string{"tcp", "udp"} {
		lines = append(lines, fmt.Sprintf(
			"iptables -t mangle %s PREROUTING -i %s -s %s -p %s -m comment --comment %s -j TPROXY --on-port %d --on-ip 127.0.0.1 --tproxy-mark %#x/%#x",
			action, tunIface, clientIP, proto, comment, port, EgressFwmark, EgressFwmark,
		))
	}
	return lines
}
