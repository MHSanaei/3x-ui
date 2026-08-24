package frontproxy

import "testing"

func testConfig() Config {
	return Config{
		PanelBasePath: "/nAMUGqBqnQ6crf3zvE/",
		PanelPort:     52973,
		SubPath:       "/pojht0vsfvseghbdnr/",
		SubPort:       35985,
		SubEnabled:    true,
	}
}

func TestResolveTargetRoutesSecretPaths(t *testing.T) {
	c := testConfig()
	cases := []struct {
		path string
		want Route
	}{
		{"/nAMUGqBqnQ6crf3zvE/", RoutePanel},
		{"/nAMUGqBqnQ6crf3zvE", RoutePanel},
		{"/nAMUGqBqnQ6crf3zvE/panel/inbounds", RoutePanel},
		{"/nAMUGqBqnQ6crf3zvE/ws", RoutePanel},
		{"/pojht0vsfvseghbdnr/", RouteSub},
		{"/pojht0vsfvseghbdnr/abc123", RouteSub},
		{"/", RouteDecoy},
		{"/index.html", RouteDecoy},
		{"/wp-login.php", RouteDecoy},
	}
	for _, tc := range cases {
		if got := c.resolveTarget(tc.path); got != tc.want {
			t.Errorf("resolveTarget(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// A path that merely starts with the same letters is a different path, and
// must not leak the panel to an outsider probing near-miss prefixes.
func TestResolveTargetRejectsPrefixLookalikes(t *testing.T) {
	c := testConfig()
	for _, path := range []string{"/nAMUGqBqnQ6crf3zvEX", "/nAMUGqBqnQ6crf3zvE-admin", "/pojht0vsfvseghbdnrX"} {
		if got := c.resolveTarget(path); got != RouteDecoy {
			t.Errorf("resolveTarget(%q) = %v, want RouteDecoy", path, got)
		}
	}
}

// Go leaves ".." in r.URL.Path, so dispatch sees traversing paths verbatim.
// They must never resolve to the panel or the subscription without the real
// prefix actually leading the path.
func TestResolveTargetRejectsTraversalIntoSecrets(t *testing.T) {
	c := testConfig()
	for _, path := range []string{
		"/x/../nAMUGqBqnQ6crf3zvE/panel",
		"/../nAMUGqBqnQ6crf3zvE/",
		"/x/../pojht0vsfvseghbdnr/abc",
		"/..%2fnAMUGqBqnQ6crf3zvE/",
	} {
		if got := c.resolveTarget(path); got != RouteDecoy {
			t.Errorf("resolveTarget(%q) = %v, want RouteDecoy", path, got)
		}
	}
}

// With the subscription server switched off its path is not special, so it
// falls through to the decoy rather than proxying to a dead port.
func TestResolveTargetIgnoresSubPathWhenDisabled(t *testing.T) {
	c := testConfig()
	c.SubEnabled = false
	if got := c.resolveTarget("/pojht0vsfvseghbdnr/abc"); got != RouteDecoy {
		t.Errorf("got %v, want RouteDecoy when sub is disabled", got)
	}
}

// A root base path cannot be distinguished from the decoy, so it must never
// match -- otherwise every request would be swallowed by the panel route.
func TestResolveTargetRootBasePathNeverMatches(t *testing.T) {
	for _, base := range []string{"/", "", "//"} {
		c := Config{PanelBasePath: base, PanelPort: 2053}
		for _, path := range []string{"/", "/anything", "/panel/"} {
			if got := c.resolveTarget(path); got != RouteDecoy {
				t.Errorf("base %q: resolveTarget(%q) = %v, want RouteDecoy", base, path, got)
			}
		}
	}
}

// The stored setting may or may not carry surrounding slashes; both spellings
// have to resolve identically or the door breaks on a cosmetic settings edit.
func TestResolveTargetToleratesSlashSpelling(t *testing.T) {
	for _, base := range []string{"/secret/", "secret", "/secret", "secret/"} {
		c := Config{PanelBasePath: base, PanelPort: 2053}
		if got := c.resolveTarget("/secret/panel"); got != RoutePanel {
			t.Errorf("base %q: got %v, want RoutePanel", base, got)
		}
		if got := c.resolveTarget("/other"); got != RouteDecoy {
			t.Errorf("base %q: got %v, want RouteDecoy", base, got)
		}
	}
}
