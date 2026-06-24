package sub

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func initSubDB(t *testing.T) {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	// Close the handle before t.TempDir cleanup so Windows doesn't refuse to
	// remove the still-open sqlite file.
	t.Cleanup(func() { _ = database.CloseDB() })
}

// The subscription page's Copy URL must be built from the same host the
// subscriber reached the page on (after PrepareForRequest normalizes away a
// loopback/bind address) — never the raw listen IP. A subscriber that hit a
// loopback bind should see "localhost", not "127.0.0.1".
func TestBuildURLs_NormalizesListenIP(t *testing.T) {
	initSubDB(t)
	s := &SubService{}
	s.PrepareForRequest("127.0.0.1")

	subURL, _, _ := s.BuildURLs("/sub/", "/json/", "/clash/", "ABC")

	if strings.Contains(subURL, "127.0.0.1") {
		t.Fatalf("listen IP leaked into Copy URL: %q", subURL)
	}
	if !strings.Contains(subURL, "localhost") {
		t.Fatalf("Copy URL = %q, want a localhost host", subURL)
	}
	if !strings.HasSuffix(subURL, "/sub/ABC") {
		t.Fatalf("Copy URL = %q, want it to end with /sub/ABC", subURL)
	}
}

// A subscriber arriving on a real domain gets that exact domain in the Copy
// URL, with the configured sub port — matching the Client Information page.
func TestBuildURLs_UsesSubscriberDomain(t *testing.T) {
	initSubDB(t)
	s := &SubService{}
	s.PrepareForRequest("sub.example.com")

	subURL, jsonURL, clashURL := s.BuildURLs("/sub/", "/json/", "/clash/", "ABC")

	if subURL != "http://sub.example.com:2096/sub/ABC" {
		t.Fatalf("subURL = %q", subURL)
	}
	if jsonURL != "http://sub.example.com:2096/json/ABC" {
		t.Fatalf("jsonURL = %q", jsonURL)
	}
	if clashURL != "http://sub.example.com:2096/clash/ABC" {
		t.Fatalf("clashURL = %q", clashURL)
	}
}

// A local wildcard inbound (no node, no custom share address, blank/0.0.0.0
// listen) must not advertise the raw request host when it carries a client IP
// that leaked in behind NAT/proxy. The admin's configured panel host wins for
// this last-resort fallback; without a configured host the request host stands.
func TestResolveInboundAddress_PrefersConfiguredHostOverClientIP(t *testing.T) {
	initSubDB(t)
	local := &model.Inbound{Listen: "", ShareAddrStrategy: "node"}

	s := &SubService{}
	s.PrepareForRequest("192.168.1.50") // a client LAN IP that reached the panel
	if got := s.resolveInboundAddress(local); got != "192.168.1.50" {
		t.Fatalf("with no configured host the request host stands, got %q", got)
	}

	if err := database.GetDB().Create(&model.Setting{Key: "subDomain", Value: "panel.example.com"}).Error; err != nil {
		t.Fatalf("set subDomain: %v", err)
	}
	s2 := &SubService{}
	s2.PrepareForRequest("192.168.1.50")
	if got := s2.resolveInboundAddress(local); got != "panel.example.com" {
		t.Fatalf("configured host must win over the leaked client IP, got %q", got)
	}
}

func TestBuildURLs_EmptySubId(t *testing.T) {
	initSubDB(t)
	s := &SubService{}
	s.PrepareForRequest("sub.example.com")
	a, b, c := s.BuildURLs("/sub/", "/json/", "/clash/", "")
	if a != "" || b != "" || c != "" {
		t.Fatalf("empty subId must yield empty URLs, got %q %q %q", a, b, c)
	}
}

func TestForRequestDoesNotMutateSharedService(t *testing.T) {
	initSubDB(t)
	base := &SubService{}

	first := base.ForRequest("first.example.com")
	second := base.ForRequest("second.example.com")

	if base.address != "" || base.nodesByID != nil {
		t.Fatalf("ForRequest mutated the shared service: address=%q nodes=%v", base.address, base.nodesByID)
	}

	firstURL, _, _ := first.BuildURLs("/sub/", "/json/", "/clash/", "ABC")
	secondURL, _, _ := second.BuildURLs("/sub/", "/json/", "/clash/", "ABC")
	if !strings.Contains(firstURL, "first.example.com") {
		t.Fatalf("first request URL = %q, want first.example.com", firstURL)
	}
	if !strings.Contains(secondURL, "second.example.com") {
		t.Fatalf("second request URL = %q, want second.example.com", secondURL)
	}
}

// A subscriber arriving via a reverse proxy (subURI configured with full
// HTTPS URL) must see the same scheme+host in the JSON and Clash Copy
// URLs as in the main subURL — not the raw sub-server port 2096.
func TestBuildURLs_DerivesJsonFromConfiguredSubURI(t *testing.T) {
	initSubDB(t)
	s := &SubService{}
	s.PrepareForRequest("sub.example.com")

	// Simulate the admin having set subURI (reverse-proxy setup).
	database.GetDB().Exec(
		"INSERT INTO settings (key, value) VALUES (?, ?)",
		"subURI", "https://example.com/sub-xxx/")

	subURL, jsonURL, clashURL := s.BuildURLs("/sub-xxx/", "/json/", "/clash/", "ABC")

	if subURL != "https://example.com/sub-xxx/ABC" {
		t.Fatalf("subURL = %q", subURL)
	}
	if jsonURL != "https://example.com/json/ABC" {
		t.Fatalf("jsonURL = %q (should derive scheme+host from subURI), want %q", jsonURL, "https://example.com/json/ABC")
	}
	if clashURL != "https://example.com/clash/ABC" {
		t.Fatalf("clashURL = %q (should derive scheme+host from subURI), want %q", clashURL, "https://example.com/clash/ABC")
	}
}

// A malformed subURI (no scheme/host) must not leak a broken base into the
// JSON/Clash URLs; BuildURLs should fall back to the request-derived base.
func TestBuildURLs_MalformedSubURIFallsBackToRequestBase(t *testing.T) {
	initSubDB(t)
	s := &SubService{}
	s.PrepareForRequest("sub.example.com")

	// A value with no scheme can't yield a usable scheme+host.
	database.GetDB().Exec(
		"INSERT INTO settings (key, value) VALUES (?, ?)",
		"subURI", "example.com/sub-xxx/")

	_, jsonURL, clashURL := s.BuildURLs("/sub-xxx/", "/json/", "/clash/", "ABC")

	if jsonURL != "http://sub.example.com:2096/json/ABC" {
		t.Fatalf("jsonURL = %q, want fallback to request base %q", jsonURL, "http://sub.example.com:2096/json/ABC")
	}
	if clashURL != "http://sub.example.com:2096/clash/ABC" {
		t.Fatalf("clashURL = %q, want fallback to request base %q", clashURL, "http://sub.example.com:2096/clash/ABC")
	}
}
