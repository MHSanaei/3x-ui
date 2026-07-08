package service

import "testing"

func TestValidateFinalMaskRealityCombo(t *testing.T) {
	tests := []struct {
		name           string
		streamSettings string
		wantErr        bool
	}{
		{
			name:           "empty streamSettings",
			streamSettings: "",
			wantErr:        false,
		},
		{
			name:           "reality without finalmask",
			streamSettings: `{"security":"reality","realitySettings":{}}`,
			wantErr:        false,
		},
		{
			name:           "reality with empty finalmask",
			streamSettings: `{"security":"reality","finalmask":{"tcp":[],"udp":[]}}`,
			wantErr:        false,
		},
		{
			name:           "reality with tcp fragment finalmask",
			streamSettings: `{"security":"reality","finalmask":{"tcp":[{"type":"fragment","settings":{"packets":"tlshello"}}]}}`,
			wantErr:        true,
		},
		{
			name:           "reality with udp finalmask",
			streamSettings: `{"security":"reality","finalmask":{"udp":[{"type":"salamander"}]}}`,
			wantErr:        true,
		},
		{
			name:           "non-reality security with finalmask",
			streamSettings: `{"security":"tls","finalmask":{"tcp":[{"type":"fragment","settings":{"packets":"tlshello"}}]}}`,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFinalMaskRealityCombo(tt.streamSettings)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFinalMaskRealityCombo(%q) error = %v, wantErr %v", tt.streamSettings, err, tt.wantErr)
			}
		})
	}
}
