package amneziawg

import (
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
)

// portSpec is a single port (start == end) or an inclusive range start..end.
type portSpec struct {
	start int
	end   int
}

func (p portSpec) isRange() bool { return p.end > p.start }

// dportArg returns the iptables --dport argument: "N" or "N:M".
func (p portSpec) dportArg() string {
	if p.isRange() {
		return fmt.Sprintf("%d:%d", p.start, p.end)
	}
	return strconv.Itoa(p.start)
}

// dnatTarget returns the DNAT target: "ip:N" or "ip:N-M".
func (p portSpec) dnatTarget(clientIP string) string {
	if p.isRange() {
		return fmt.Sprintf("%s:%d-%d", clientIP, p.start, p.end)
	}
	return fmt.Sprintf("%s:%d", clientIP, p.start)
}

// parseForwardedPorts splits a free-form "80, 443; 8000-8100" into specs,
// silently dropping invalid tokens. Every returned bound is an integer in
// [1, 65535], so callers may embed them in a shell-executed line unescaped.
func parseForwardedPorts(input string) []portSpec {
	if input == "" {
		return nil
	}
	input = strings.ReplaceAll(input, ";", ",")
	tokens := strings.Split(input, ",")

	var specs []portSpec
	seen := make(map[string]struct{}, len(tokens))
	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		spec, ok := parsePortToken(tok)
		if !ok {
			continue
		}
		key := fmt.Sprintf("%d-%d", spec.start, spec.end)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		specs = append(specs, spec)
	}
	return specs
}

func parsePortToken(tok string) (portSpec, bool) {
	if idx := strings.IndexByte(tok, '-'); idx >= 0 {
		start, ok1 := parsePortNumber(strings.TrimSpace(tok[:idx]))
		end, ok2 := parsePortNumber(strings.TrimSpace(tok[idx+1:]))
		if !ok1 || !ok2 || start > end {
			return portSpec{}, false
		}
		return portSpec{start: start, end: end}, true
	}
	p, ok := parsePortNumber(tok)
	if !ok {
		return portSpec{}, false
	}
	return portSpec{start: p, end: p}, true
}

func parsePortNumber(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 65535 {
		return 0, false
	}
	return n, true
}

// ForwardedPortsInclude reports whether port is covered by a raw
// ForwardedPorts string. portForwardLines has no -d restriction, so save-time
// callers use this to reject a spec that would hijack a port already in use.
func ForwardedPortsInclude(forwardedPorts string, port int) bool {
	for _, spec := range parseForwardedPorts(forwardedPorts) {
		if port >= spec.start && port <= spec.end {
			return true
		}
	}
	return false
}

// portForwardComment tags one peer's forwarded-port rules so PostDown removes
// exactly what PostUp added. Hashed for the same reason routeEgressComment is.
func portForwardComment(email string) string {
	if email == "" {
		return "awg-fwd"
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(email))
	return fmt.Sprintf("awg-fwd-%08x", h.Sum32())
}

// portForwardLines returns the PostUp ("-A") or PostDown ("-D") DNAT + FORWARD
// rules for one peer's forwarded ports, tcp and udp alike (games and P2P need
// UDP). Returns nil when the spec has no valid entry or clientIP is empty.
func portForwardLines(action, extIface, tunIface, clientIP, email, forwardedPorts string) []string {
	specs := parseForwardedPorts(forwardedPorts)
	if len(specs) == 0 {
		return nil
	}
	clientIP = stripCIDRMask(clientIP)
	if clientIP == "" {
		return nil
	}
	comment := portForwardComment(email)

	lines := make([]string, 0, len(specs)*4)
	for _, spec := range specs {
		dport := spec.dportArg()
		target := spec.dnatTarget(clientIP)
		for _, proto := range []string{"tcp", "udp"} {
			nat := fmt.Sprintf("iptables -t nat %s PREROUTING -p %s", action, proto)
			if extIface != "" {
				nat += fmt.Sprintf(" -i %s", extIface)
			}
			nat += fmt.Sprintf(" --dport %s -m comment --comment %s -j DNAT --to-destination %s", dport, comment, target)
			lines = append(lines, nat)

			fwd := fmt.Sprintf("iptables %s FORWARD -d %s -p %s -o %s --dport %s -m comment --comment %s -j ACCEPT",
				action, clientIP, proto, tunIface, dport, comment)
			lines = append(lines, fwd)
		}
	}
	return lines
}

// stripCIDRMask removes a "/N" suffix if present.
func stripCIDRMask(addr string) string {
	if idx := strings.IndexByte(addr, '/'); idx >= 0 {
		return addr[:idx]
	}
	return addr
}
