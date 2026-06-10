package service

import "testing"

// BuildSubURIBase is the single source of truth for the scheme://host[:port]
// prefix shown both on the panel's Client Information page and on the
// subscription page. The cases pin scheme selection (sub TLS cert/key),
// Sub Domain preference, standard-port omission, and IPv6 bracketing.
func TestBuildSubURIBase(t *testing.T) {
	setupConflictDB(t)
	s := &SettingService{}

	set := func(subDomain, port, cert, key string) {
		if err := s.saveSetting("subDomain", subDomain); err != nil {
			t.Fatalf("set subDomain: %v", err)
		}
		if err := s.saveSetting("subPort", port); err != nil {
			t.Fatalf("set subPort: %v", err)
		}
		if err := s.saveSetting("subCertFile", cert); err != nil {
			t.Fatalf("set subCertFile: %v", err)
		}
		if err := s.saveSetting("subKeyFile", key); err != nil {
			t.Fatalf("set subKeyFile: %v", err)
		}
	}

	cases := []struct {
		name            string
		subDomain, port string
		cert, key       string
		host            string
		want            string
	}{
		{"no domain, plain, non-standard port", "", "2096", "", "", "panel.example.com", "http://panel.example.com:2096"},
		{"host carries a port — stripped, sub port applied", "", "2096", "", "", "panel.example.com:9999", "http://panel.example.com:2096"},
		{"sub domain preferred over host", "sub.cdn.com", "2096", "", "", "panel.example.com", "http://sub.cdn.com:2096"},
		{"tls + 443 omits the port", "sub.cdn.com", "443", "/c.crt", "/k.key", "panel.example.com", "https://sub.cdn.com"},
		{"plain + 80 omits the port", "", "80", "", "", "x.com", "http://x.com"},
		{"tls on a non-standard port keeps it", "", "2096", "/c.crt", "/k.key", "x.com", "https://x.com:2096"},
		{"ipv6 host is bracketed", "", "2096", "", "", "::1", "http://[::1]:2096"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			set(c.subDomain, c.port, c.cert, c.key)
			if got := s.BuildSubURIBase(c.host); got != c.want {
				t.Fatalf("BuildSubURIBase(%q) = %q, want %q", c.host, got, c.want)
			}
		})
	}
}
