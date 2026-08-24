package service

import (
	"net"
	"testing"
)

func TestRejectAllBlockedIPsNeedsOnlyOneUsableAddress(t *testing.T) {
	teredo := net.IPAddr{IP: net.ParseIP("2001::1")}
	public4 := net.IPAddr{IP: net.ParseIP("142.250.74.36")}
	private4 := net.IPAddr{IP: net.ParseIP("10.0.0.1")}

	cases := []struct {
		name    string
		ips     []net.IPAddr
		wantErr string
	}{
		{"poisoned AAAA next to healthy A", []net.IPAddr{teredo, public4}, ""},
		{"all blocked", []net.IPAddr{teredo, private4}, "host h.example resolves to blocked private/internal address 2001::1"},
		{"no addresses", nil, "host h.example has no IP addresses"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := rejectAllBlockedIPs("h.example", tc.ips)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("rejectAllBlockedIPs() = %v, want nil", err)
				}
				return
			}
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("rejectAllBlockedIPs() = %v, want %q", err, tc.wantErr)
			}
		})
	}
}
