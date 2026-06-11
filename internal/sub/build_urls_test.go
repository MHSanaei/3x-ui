package sub

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
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

func TestBuildURLs_EmptySubId(t *testing.T) {
	initSubDB(t)
	s := &SubService{}
	s.PrepareForRequest("sub.example.com")
	a, b, c := s.BuildURLs("/sub/", "/json/", "/clash/", "")
	if a != "" || b != "" || c != "" {
		t.Fatalf("empty subId must yield empty URLs, got %q %q %q", a, b, c)
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
