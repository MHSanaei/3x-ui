package amneziawg

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// portSpec is a single port (start == end) or an inclusive range start..end.
type portSpec struct {
	start int
	end   int
}

// parseForwardedPorts splits a user-supplied string ("80, 443; 8000-8100")
// into validated port specs. Tokens are separated by comma or semicolon;
// whitespace is ignored. Invalid tokens are silently dropped — the input is
// a free-form text field and validation is best-effort by design.
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

// ForwardedPortsInclude reports whether port is covered by any spec in a raw
// ForwardedPorts string (a single port or an inclusive range). Used for
// save-time validation that a client isn't about to hijack the panel's own
// port or another inbound's port -- see
// internal/web/service/inbound_amneziawg.go's port-conflict checks.
//
// Per-client port-forwarding is implemented by internal/amneziawgnet's
// listener supervisor (PortForwardSet), which dials directly into the
// embedded gVisor netstack toward the peer's tunnel-internal address --
// the retired kernel-module architecture used PostUp/PostDown iptables DNAT
// rules instead, which had no equivalent path once that architecture was
// cut over; ExpandForwardedPorts below is what the supervisor uses to turn
// a raw spec into the concrete ports it listens on.
func ForwardedPortsInclude(forwardedPorts string, port int) bool {
	for _, spec := range parseForwardedPorts(forwardedPorts) {
		if port >= spec.start && port <= spec.end {
			return true
		}
	}
	return false
}

// MaxForwardedPorts caps how many unique ports a single client's
// ForwardedPorts spec can expand to. internal/amneziawgnet's listener
// supervisor opens up to two real sockets (TCP+UDP) per port, so this bounds
// worst-case file descriptor usage to a fixed, sane amount regardless of how
// large a stored spec claims to be -- a legacy or hand-edited "1-65535"
// costs exactly the same as "1-100" once expansion stops at the cap.
const MaxForwardedPorts = 100

// ExpandForwardedPorts parses forwardedPorts the same way
// ForwardedPortsInclude does and returns every unique port it covers, in
// ascending order, capped at MaxForwardedPorts. Expansion stops the instant
// the cap is reached rather than expanding fully and truncating afterward,
// so this is safe to call unconditionally against arbitrary -- including
// pre-existing, pre-cap -- stored data.
func ExpandForwardedPorts(forwardedPorts string) []int {
	return expandForwardedPorts(forwardedPorts, MaxForwardedPorts)
}

// ExceedsForwardedPortsCap reports whether forwardedPorts expands to more
// than MaxForwardedPorts unique ports. Expanding to MaxForwardedPorts+1 (one
// past the cap) rather than reusing ExpandForwardedPorts's own
// cap-truncated result is what actually distinguishes "exactly at the cap"
// from "over it" -- ExpandForwardedPorts stops emitting the instant it hits
// MaxForwardedPorts, so its result's length can never exceed the cap and a
// len(...) >= MaxForwardedPorts check would wrongly reject a spec that
// lands exactly on the boundary.
func ExceedsForwardedPortsCap(forwardedPorts string) bool {
	return len(expandForwardedPorts(forwardedPorts, MaxForwardedPorts+1)) > MaxForwardedPorts
}

func expandForwardedPorts(forwardedPorts string, limit int) []int {
	seen := make(map[int]struct{}, limit)
	ports := make([]int, 0, limit)
outer:
	for _, spec := range parseForwardedPorts(forwardedPorts) {
		for p := spec.start; p <= spec.end; p++ {
			if len(ports) >= limit {
				break outer
			}
			if _, dup := seen[p]; dup {
				continue
			}
			seen[p] = struct{}{}
			ports = append(ports, p)
		}
	}
	sort.Ints(ports)
	return ports
}
