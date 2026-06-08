package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/util/link"
)

func TestAssignStableTags(t *testing.T) {
	t.Run("reuses the tag mapped to a known identity", func(t *testing.T) {
		parsed := []link.Outbound{{"tag": "JP-Tokyo"}}
		prev := map[string]string{"id-abc": "sub1-keepme"}
		got := assignStableTags(parsed, []string{"id-abc"}, prev, nil, 1, "")
		if got[0] != "sub1-keepme" {
			t.Fatalf("got %q, want sub1-keepme", got[0])
		}
		if parsed[0]["tag"] != "sub1-keepme" {
			t.Fatalf("tag was not written back into the outbound: %v", parsed[0]["tag"])
		}
	})

	t.Run("falls back to the previous tag at the same position", func(t *testing.T) {
		parsed := []link.Outbound{{"tag": "JP-Tokyo"}}
		got := assignStableTags(parsed, []string{"id-new"}, map[string]string{}, map[int]string{0: "sub1-oldpos"}, 1, "")
		if got[0] != "sub1-oldpos" {
			t.Fatalf("got %q, want sub1-oldpos", got[0])
		}
	})

	t.Run("allocates a fresh tag with the default sub<id>- prefix", func(t *testing.T) {
		parsed := []link.Outbound{{"tag": "Tokyo"}}
		got := assignStableTags(parsed, []string{"id-x"}, nil, nil, 7, "")
		want := link.SuggestTag("sub7-", "Tokyo", 0)
		if got[0] != want {
			t.Fatalf("got %q, want %q", got[0], want)
		}
	})

	t.Run("uses a custom prefix for fresh tags", func(t *testing.T) {
		parsed := []link.Outbound{{"tag": "Tokyo"}}
		got := assignStableTags(parsed, []string{"id-x"}, nil, nil, 1, "hk-")
		want := link.SuggestTag("hk-", "Tokyo", 0)
		if got[0] != want {
			t.Fatalf("got %q, want %q", got[0], want)
		}
	})

	t.Run("disambiguates colliding tags with a -N suffix", func(t *testing.T) {
		parsed := []link.Outbound{{"tag": "Same"}, {"tag": "Same"}}
		got := assignStableTags(parsed, []string{"id1", "id2"}, nil, nil, 1, "p-")
		base := link.SuggestTag("p-", "Same", 0)
		if got[0] != base {
			t.Fatalf("got[0] = %q, want %q", got[0], base)
		}
		if got[1] != base+"-1" {
			t.Fatalf("got[1] = %q, want %q", got[1], base+"-1")
		}
	})
}

// TestSanitizePublicHTTPURLRejectsPrivateAndBadSchemes covers the SSRF guard used
// when fetching subscription URLs. All rejected cases use literal IPs or bad
// schemes so the test never performs real DNS resolution.
func TestSanitizePublicHTTPURLRejectsPrivateAndBadSchemes(t *testing.T) {
	rejected := []string{
		"http://127.0.0.1/sub",                    // loopback
		"http://10.0.0.1/x",                       // private
		"http://192.168.1.1",                      // private
		"http://169.254.169.254/latest/meta-data", // link-local (cloud metadata)
		"http://[::1]:8080/sub",                   // IPv6 loopback
		"http://0.0.0.0",                          // unspecified
		"ftp://example.com/x",                     // unsupported scheme
		"file:///etc/passwd",                      // unsupported scheme
	}
	for _, raw := range rejected {
		if _, err := SanitizePublicHTTPURL(raw, false); err == nil {
			t.Errorf("expected %q to be rejected, got nil error", raw)
		}
	}

	t.Run("allows a public literal IP without DNS", func(t *testing.T) {
		got, err := SanitizePublicHTTPURL("http://8.8.8.8/sub", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "http://8.8.8.8/sub" {
			t.Fatalf("got %q, want http://8.8.8.8/sub", got)
		}
	})
}
