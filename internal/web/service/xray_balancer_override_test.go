package service

import (
	"errors"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// The core takes any string as an override and closes every connection routed
// to a tag it lacks, so each refused case here is traffic the panel would drop.
func TestResolveOverrideTarget(t *testing.T) {
	cfg := &xray.Config{
		RouterConfig: []byte(`{
			"rules": [
				{"type": "field", "inboundTag": ["_bl_b2"], "balancerTag": "b2"},
				{"type": "field", "port": "7777", "balancerTag": "b3"},
				{"type": "field", "inboundTag": ["_bl_b4"], "balancerTag": "b4"}
			],
			"balancers": [
				{"tag": "b1", "selector": ["proxy-"]},
				{"tag": "b2", "selector": ["proxy-"]},
				{"tag": "b3", "selector": ["proxy-"]},
				{"tag": "b4", "selector": ["proxy-"]}
			]
		}`),
		OutboundConfigs: []byte(`[
			{"tag": "direct", "protocol": "freedom"},
			{"tag": "proxy-a", "protocol": "freedom"},
			{"tag": "_bl_b2", "protocol": "loopback", "settings": {"inboundTag": "_bl_b2"}}
		]`),
	}
	cases := []struct {
		name, target, want string
		wantErr            error
	}{
		{"outbound is used as is", "proxy-a", "proxy-a", nil},
		{"balancer resolves to the loopback outbound feeding it", "b2", "_bl_b2", nil},
		{"balancer without a loopback is refused", "b3", "", errOverrideTargetUnknown},
		{"loopback rule whose outbound is gone is refused", "b4", "", errOverrideTargetUnknown},
		{"unknown tag is refused", "no-such-outbound", "", errOverrideTargetUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveOverrideTarget(cfg, tc.target)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("resolveOverrideTarget(%q) error = %v, want %v", tc.target, err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("resolveOverrideTarget(%q) = %q, want %q", tc.target, got, tc.want)
			}
		})
	}
}
