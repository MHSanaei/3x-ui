package sub

import "regexp"

// happUserAgentRegex matches the Happ client's own User-Agent header.
var happUserAgentRegex = regexp.MustCompile(`(?i)\bhapp\b`)

// IsHappClient reports whether userAgent identifies as the Happ client.
// Ported from upstream MHSanaei/3x-ui#6434, which found a real gap this
// panel had too: Routing-Enable/Hide-Settings were only ever set to their
// "on" value and omitted entirely when off, so a client that had them
// enabled and then got them turned back off in the panel kept whatever
// state it had already cached -- Happ's own spec calls for an explicit "0"
// to clear that. Gated on IsHappClient rather than sent unconditionally: no
// other subscription client (v2rayN, Clash, ...) expects these headers, so
// there is no reason to add them to every response.
func IsHappClient(userAgent string) bool {
	return happUserAgentRegex.MatchString(userAgent)
}
