package config

import (
	"os"
	"testing"
)

func TestGetPortOverride(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		set        bool
		wantPort   int
		configured bool
		wantErr    bool
	}{
		{name: "unset"},
		{name: "empty", value: "", set: true},
		{name: "whitespace", value: "   ", set: true},
		{name: "minimum", value: "1", set: true, wantPort: 1, configured: true},
		{name: "default panel port", value: "2053", set: true, wantPort: 2053, configured: true},
		{name: "surrounding whitespace", value: " 8080 ", set: true, wantPort: 8080, configured: true},
		{name: "maximum", value: "65535", set: true, wantPort: 65535, configured: true},
		{name: "zero", value: "0", set: true, configured: true, wantErr: true},
		{name: "above maximum", value: "65536", set: true, configured: true, wantErr: true},
		{name: "negative", value: "-1", set: true, configured: true, wantErr: true},
		{name: "non-numeric", value: "abc", set: true, configured: true, wantErr: true},
		{name: "decimal", value: "8080.0", set: true, configured: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.set {
				t.Setenv("XUI_PORT", tt.value)
			} else {
				original, existed := os.LookupEnv("XUI_PORT")
				if err := os.Unsetenv("XUI_PORT"); err != nil {
					t.Fatalf("unset XUI_PORT: %v", err)
				}
				t.Cleanup(func() {
					if existed {
						_ = os.Setenv("XUI_PORT", original)
					} else {
						_ = os.Unsetenv("XUI_PORT")
					}
				})
			}

			port, configured, err := GetPortOverride()
			if port != tt.wantPort {
				t.Errorf("port = %d, want %d", port, tt.wantPort)
			}
			if configured != tt.configured {
				t.Errorf("configured = %t, want %t", configured, tt.configured)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}
