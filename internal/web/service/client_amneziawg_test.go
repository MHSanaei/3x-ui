package service

import (
	"fmt"
	"testing"
)

// AmneziaWG's own kernel interface Address is exactly the configured
// subnet (unlike WireGuard's Xray-native inbound), so allocation for it must
// never widen past that subnet -- an address from outside it would be
// silently unroutable. See PR #6105 Finding 12.
func TestAllocateWireguardAddress_AmneziaWGNeverWidens(t *testing.T) {
	used := make([]string, 0, 254)
	for i := 2; i <= 255; i++ {
		used = append(used, fmt.Sprintf("10.8.1.%d/32", i))
	}
	if _, err := allocateWireguardAddress(used, "10.8.1.0/24", false); err == nil {
		t.Fatal("a full AmneziaWG /24 must fail loudly instead of allocating an address outside the interface's own subnet")
	}
}

func TestAllocateWireguardAddress_AmneziaWGFillsItsOwnSubnetNormally(t *testing.T) {
	got, err := allocateWireguardAddress([]string{"10.8.1.2/32"}, "10.8.1.0/24", false)
	if err != nil {
		t.Fatalf("allocateWireguardAddress: %v", err)
	}
	if got != "10.8.1.3/32" {
		t.Fatalf("address = %q, want 10.8.1.3/32", got)
	}
}
