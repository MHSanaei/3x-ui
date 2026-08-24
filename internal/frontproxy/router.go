// Package frontproxy is the panel's own reverse proxy: the single
// local listener Xray's REALITY fallback target points at, which routes by
// URL path to the panel, to the subscription server, or to a decoy site.
//
// This replaces what admins otherwise hand-build with an external nginx. The
// REALITY fallback hands over raw, still-encrypted bytes -- Xray cannot
// decrypt them -- so this listener terminates real TLS itself, exactly like
// that nginx would.
package frontproxy

import "strings"

// Route names where a reverse-proxy request should be sent.
type Route int

const (
	// RouteDecoy is the fallback: anything not matching a secret path.
	RouteDecoy Route = iota
	// RoutePanel is the admin panel, reached under its own base path.
	RoutePanel
	// RouteSub is the subscription server, reached under its own path.
	RouteSub
)

// Config is the routing half of the reverse proxy, resolved from settings.
// Ports are loopback targets on this same host.
type Config struct {
	PanelBasePath string
	PanelPort     int
	SubPath       string
	SubPort       int
	SubEnabled    bool
	// UpstreamTLS is set when the panel and subscription listeners serve TLS
	// themselves, which they do whenever certificate files are configured.
	UpstreamTLS bool
}

// resolveTarget picks the destination for one request path. Subscription is
// checked first so a sub path nested under the panel's base still reaches it.
func (c Config) resolveTarget(path string) Route {
	if c.SubEnabled && matchesPrefix(path, c.SubPath) {
		return RouteSub
	}
	if matchesPrefix(path, c.PanelBasePath) {
		return RoutePanel
	}
	return RouteDecoy
}

// matchesPrefix reports whether path is at or under base. A root or empty
// base never matches: it would swallow every request and hide the decoy.
func matchesPrefix(path, base string) bool {
	trimmed := strings.Trim(base, "/")
	if trimmed == "" {
		return false
	}
	exact := "/" + trimmed
	return path == exact || strings.HasPrefix(path, exact+"/")
}
