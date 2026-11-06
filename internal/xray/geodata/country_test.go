package geodata

import (
	"errors"
	"net/netip"
	"os"
	"testing"
)

func TestCountryIndexLookup(t *testing.T) {
	dir := t.TempDir()
	writeIPDB(t, dir, "geoip.dat",
		geoip("DE", "10.0.0.0/8", "20.0.0.0/24", "20.0.1.0/24"),
		geoip("fr", "10.1.0.0/16"),
		geoip("ES", "10.1.2.0/24", "2001:db8:1::/48"),
		geoip("US", "2001:db8::/32", "255.255.255.0/24"),
		// Service categories overlap real allocations, and are not places.
		geoip("PRIVATE", "192.168.0.0/16", "10.9.0.0/16"),
		geoip("CLOUDFLARE", "1.1.1.0/24"),
	)
	idx, err := LoadCountryIndex(dir, "geoip.dat")
	if err != nil {
		t.Fatalf("LoadCountryIndex: %v", err)
	}
	cases := []struct {
		ip   string
		want string
	}{
		{"10.0.0.1", "DE"},
		{"10.1.0.1", "FR"},
		{"10.1.2.3", "ES"},
		{"10.1.3.1", "FR"}, // back in the parent after the nested /24 ends
		{"10.2.0.1", "DE"}, // back in the grandparent after the nested /16 ends
		{"10.9.0.1", "DE"}, // a service category must not punch a hole in a country
		{"10.255.255.255", "DE"},
		{"9.255.255.255", ""},
		{"11.0.0.0", ""},
		{"20.0.0.200", "DE"},
		{"20.0.1.200", "DE"},
		{"20.0.2.0", ""},
		{"192.168.1.1", ""},
		{"1.1.1.1", ""},
		{"255.255.255.255", "US"}, // a range ending at the top of IPv4 must not overflow
		{"2001:db8::1", "US"},
		{"2001:db8:1::1", "ES"},
		{"2001:db8:2::1", "US"},
		{"2001:db9::1", ""},
		{"::ffff:10.0.0.1", "DE"}, // the same client, written as a v4-mapped address
	}
	for _, tc := range cases {
		if got := idx.Country(netip.MustParseAddr(tc.ip)); got != tc.want {
			t.Errorf("Country(%s) = %q, want %q", tc.ip, got, tc.want)
		}
	}
}

// Neighbouring ranges of one country are the common case, so the table should not pay for them.
func TestCountryIndexMergesAdjacentRanges(t *testing.T) {
	dir := t.TempDir()
	writeIPDB(t, dir, "geoip.dat", geoip("DE", "20.0.0.0/24", "20.0.1.0/24", "20.0.2.0/23"))
	idx, err := LoadCountryIndex(dir, "geoip.dat")
	if err != nil {
		t.Fatalf("LoadCountryIndex: %v", err)
	}
	if got := len(idx.starts); got != 2 {
		t.Errorf("table holds %d breakpoints, want 2: where DE begins and where it ends", got)
	}
}

func TestCountryIndexCoversTheWholeAddressSpace(t *testing.T) {
	dir := t.TempDir()
	writeIPDB(t, dir, "geoip.dat", geoip("AQ", "::/0"), geoip("FR", "ffff:ffff::/32"))
	idx, err := LoadCountryIndex(dir, "geoip.dat")
	if err != nil {
		t.Fatalf("LoadCountryIndex: %v", err)
	}
	for ip, want := range map[string]string{
		"::":           "AQ",
		"ffff:fffe::1": "AQ",
		"ffff:ffff::1": "FR",
		"ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff": "FR",
	} {
		if got := idx.Country(netip.MustParseAddr(ip)); got != want {
			t.Errorf("Country(%s) = %q, want %q", ip, got, want)
		}
	}
}

func TestCountryIndexRejectsBadInput(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadCountryIndex(dir, "geoip.dat"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing file error = %v, want os.ErrNotExist", err)
	}
	writeFile(t, dir, "broken.dat", []byte{0xff, 0xff, 0xff})
	if _, err := LoadCountryIndex(dir, "broken.dat"); err == nil {
		t.Error("truncated database loaded without an error")
	}
	writeIPDB(t, dir, "services.dat", geoip("CLOUDFLARE", "1.1.1.0/24"), geoip("A1", "2.2.2.0/24"))
	idx, err := LoadCountryIndex(dir, "services.dat")
	if err != nil {
		t.Fatalf("LoadCountryIndex: %v", err)
	}
	if got := idx.Country(netip.MustParseAddr("1.1.1.1")); got != "" {
		t.Errorf("Country(service range) = %q, want none", got)
	}
	if got := idx.Country(netip.MustParseAddr("2.2.2.2")); got != "" {
		t.Errorf("Country(code with a digit) = %q, want none", got)
	}
}
