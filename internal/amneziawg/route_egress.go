package amneziawg

import (
	"fmt"
	"hash/fnv"
)

// EgressBasePort is the first loopback port of an AmneziaWG inbound's opt-in
// Xray TPROXY bridge (Instance.RouteThroughXray, off by default).
const EgressBasePort = 63100

// EgressPortForInbound derives one inbound's bridge port from its id, so this
// package's PostUp generator and the Xray-config generator never have to agree
// on a runtime-negotiated value. ok is false once the port leaves the valid
// range: an impossible Port makes Xray reject the whole generated config.
func EgressPortForInbound(inboundID int) (port int, ok bool) {
	port = EgressBasePort + inboundID
	return port, inboundID > 0 && port <= 65535
}

// EgressFwmark and EgressTable let TPROXY deliver a peer's packets to a local
// socket despite a non-local destination. Host-wide and shared by every
// instance; change them here if they collide with something else on the host.
const (
	EgressFwmark = 0x2377
	EgressTable  = 87
)

// routeEgressComment tags one peer's TPROXY rule so PostDown removes exactly
// what PostUp added. Hashed, not the raw email: that is admin-supplied free
// text ending up in a shell-executed line.
func routeEgressComment(email string) string {
	if email == "" {
		return "awg-route"
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(email))
	return fmt.Sprintf("awg-route-%08x", h.Sum32())
}

// routeEgressLines returns the PostUp ("-A") or PostDown ("-D") mangle rules
// redirecting one peer's whole traffic into its instance's Xray bridge; which
// outbound it then takes is left entirely to the admin's own Routing rules.
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
