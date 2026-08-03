package amneziawg

import (
	"fmt"
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
// The field itself is currently inert: per-client port-forwarding was
// implemented via PostUp/PostDown iptables DNAT rules under the retired
// kernel-module architecture (internal/amneziawg's old Manager), which had
// no equivalent under the embedded amneziawg-go path
// (internal/amneziawgnet) as of the hard cutover -- see the migration
// plan's Phase 3.6 for the panel-side relay design that will restore it.
// The field and this validation are kept so existing values aren't lost and
// re-validated identically once that phase lands, not because anything
// currently acts on them.
func ForwardedPortsInclude(forwardedPorts string, port int) bool {
	for _, spec := range parseForwardedPorts(forwardedPorts) {
		if port >= spec.start && port <= spec.end {
			return true
		}
	}
	return false
}
