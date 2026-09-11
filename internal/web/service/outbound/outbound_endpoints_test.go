package outbound

import (
	"reflect"
	"testing"
)

func TestExtractOutboundEndpointsVLESS(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		want     []string
	}{
		{
			name: "vnext endpoints",
			settings: map[string]any{
				"vnext": []any{
					map[string]any{"address": "first.example.com", "port": float64(443)},
					map[string]any{"address": "second.example.com", "port": float64(8443)},
				},
			},
			want: []string{"first.example.com:443", "second.example.com:8443"},
		},
		{
			name: "flat endpoint",
			settings: map[string]any{
				"address": "legacy.example.com",
				"port":    float64(443),
			},
			want: []string{"legacy.example.com:443"},
		},
		{
			name: "invalid vnext falls back to flat endpoint",
			settings: map[string]any{
				"vnext":   []any{map[string]any{"address": "missing-port.example.com"}},
				"address": "fallback.example.com",
				"port":    float64(2053),
			},
			want: []string{"fallback.example.com:2053"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractOutboundEndpoints(map[string]any{
				"protocol": "vless",
				"settings": tt.settings,
			})
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("extractOutboundEndpoints() = %v, want %v", got, tt.want)
			}
		})
	}
}
