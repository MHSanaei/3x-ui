package service

import (
	"encoding/json"
	"testing"
)

// TestCanonicalizeStreamNetworkKey covers xray-core v26.7.11's "method" alias
// for streamSettings "network": a config keyed on "method" (from an imported
// or API-authored inbound) must be folded back to the panel-canonical
// "network" key that every downstream reader — link generation, port-conflict
// detection, flow eligibility — depends on.
func TestCanonicalizeStreamNetworkKey(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantNetwork string
		wantMethod  bool
	}{
		{
			name:        "method alias becomes network",
			in:          `{"method": "ws", "security": "tls"}`,
			wantNetwork: "ws",
		},
		{
			name:        "method wins when both present",
			in:          `{"method": "grpc", "network": "tcp"}`,
			wantNetwork: "grpc",
		},
		{
			name:        "plain network untouched",
			in:          `{"network": "tcp"}`,
			wantNetwork: "tcp",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := canonicalizeStreamNetworkKey(tc.in)
			var stream map[string]any
			if err := json.Unmarshal([]byte(got), &stream); err != nil {
				t.Fatalf("result is not valid JSON: %v", err)
			}
			if stream["network"] != tc.wantNetwork {
				t.Fatalf("network = %v, want %q", stream["network"], tc.wantNetwork)
			}
			if _, hasMethod := stream["method"]; hasMethod {
				t.Fatalf("method key must be removed, got %s", got)
			}
		})
	}
}

func TestCanonicalizeStreamNetworkKey_EmptyPassthrough(t *testing.T) {
	if got := canonicalizeStreamNetworkKey(""); got != "" {
		t.Fatalf("empty stream must round-trip, got %q", got)
	}
}
