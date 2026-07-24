package database

import (
	"encoding/json"
	"testing"
)

func TestStripRealityFinalmaskTcpFromStream(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantChanged bool
		wantAbsent  []string
		wantPresent []string
	}{
		{
			name:        "reality with tcp masks is stripped",
			raw:         `{"network":"tcp","security":"reality","realitySettings":{"privateKey":"k"},"finalmask":{"tcp":[{"type":"sudoku"}]}}`,
			wantChanged: true,
			wantAbsent:  []string{"finalmask"},
			wantPresent: []string{"realitySettings"},
		},
		{
			name:        "reality with tcp and udp masks keeps udp",
			raw:         `{"security":"reality","finalmask":{"tcp":[{"type":"sudoku"}],"udp":[{"type":"salt"}]}}`,
			wantChanged: true,
			wantAbsent:  []string{"tcp"},
			wantPresent: []string{"udp"},
		},
		{
			name:        "tls with tcp masks is untouched",
			raw:         `{"security":"tls","finalmask":{"tcp":[{"type":"sudoku"}]}}`,
			wantChanged: false,
		},
		{
			name:        "reality without finalmask is untouched",
			raw:         `{"security":"reality","realitySettings":{}}`,
			wantChanged: false,
		},
		{
			name:        "empty stream is untouched",
			raw:         "",
			wantChanged: false,
		},
		{
			name:        "invalid json is untouched",
			raw:         "{not json",
			wantChanged: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			updated, changed := stripRealityFinalmaskTcpFromStream(tc.raw)
			if changed != tc.wantChanged {
				t.Fatalf("changed = %v, want %v (updated: %s)", changed, tc.wantChanged, updated)
			}
			if !tc.wantChanged {
				if updated != tc.raw {
					t.Fatalf("stream mutated without change flag: %s", updated)
				}
				return
			}
			var stream map[string]any
			if err := json.Unmarshal([]byte(updated), &stream); err != nil {
				t.Fatalf("updated stream is not valid json: %v", err)
			}
			flat, _ := json.Marshal(stream)
			for _, key := range tc.wantAbsent {
				if containsJSONKey(stream, key) {
					t.Fatalf("key %q should be gone: %s", key, flat)
				}
			}
			for _, key := range tc.wantPresent {
				if !containsJSONKey(stream, key) {
					t.Fatalf("key %q should survive: %s", key, flat)
				}
			}
		})
	}
}

func containsJSONKey(m map[string]any, key string) bool {
	if _, ok := m[key]; ok {
		return true
	}
	for _, v := range m {
		if nested, ok := v.(map[string]any); ok && containsJSONKey(nested, key) {
			return true
		}
	}
	return false
}
