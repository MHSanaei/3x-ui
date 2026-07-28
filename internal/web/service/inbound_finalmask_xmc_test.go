package service

import (
	"encoding/json"
	"testing"

	"github.com/xtls/xray-core/infra/conf"
)

const completeXmcProfile = `{"username":"Notch","uuid":"069a79f4-44e9-4726-a5be-fca90e38aaf5","texturesValue":"dmFsdWU=","texturesSignature":"c2ln"}`

func TestValidateFinalMaskXmcProfiles(t *testing.T) {
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
			name:           "no finalmask",
			streamSettings: `{"network":"tcp","security":"none"}`,
			wantErr:        false,
		},
		{
			name:           "non-xmc mask is untouched",
			streamSettings: `{"finalmask":{"tcp":[{"type":"fragment","settings":{"packets":"tlshello"}}]}}`,
			wantErr:        false,
		},
		{
			name:           "xmc with a complete profile",
			streamSettings: `{"finalmask":{"tcp":[{"type":"xmc","settings":{"hostname":"mc.example.com","password":"pw","profiles":[` + completeXmcProfile + `]}}]}}`,
			wantErr:        false,
		},
		{
			name:           "legacy usernames shape without profiles",
			streamSettings: `{"finalmask":{"tcp":[{"type":"xmc","settings":{"hostname":"mc.example.com","password":"pw","usernames":["Dream"]}}]}}`,
			wantErr:        true,
		},
		{
			name:           "xmc with an empty profiles array",
			streamSettings: `{"finalmask":{"tcp":[{"type":"xmc","settings":{"password":"pw","profiles":[]}}]}}`,
			wantErr:        true,
		},
		{
			name:           "profile missing the textures signature",
			streamSettings: `{"finalmask":{"tcp":[{"type":"xmc","settings":{"password":"pw","profiles":[{"username":"Notch","uuid":"069a79f4-44e9-4726-a5be-fca90e38aaf5","texturesValue":"dmFsdWU=","texturesSignature":""}]}}]}}`,
			wantErr:        true,
		},
		{
			name:           "profile with an unparseable uuid",
			streamSettings: `{"finalmask":{"tcp":[{"type":"xmc","settings":{"password":"pw","profiles":[{"username":"Notch","uuid":"not-a-uuid","texturesValue":"dmFsdWU=","texturesSignature":"c2ln"}]}}]}}`,
			wantErr:        true,
		},
		{
			name:           "profile with an out-of-range username",
			streamSettings: `{"finalmask":{"tcp":[{"type":"xmc","settings":{"password":"pw","profiles":[{"username":"ab","uuid":"069a79f4-44e9-4726-a5be-fca90e38aaf5","texturesValue":"dmFsdWU=","texturesSignature":"c2ln"}]}}]}}`,
			wantErr:        true,
		},
		{
			name:           "one complete and one incomplete profile",
			streamSettings: `{"finalmask":{"tcp":[{"type":"xmc","settings":{"password":"pw","profiles":[` + completeXmcProfile + `,{"username":"Herobrine","uuid":"","texturesValue":"","texturesSignature":""}]}}]}}`,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFinalMaskXmcProfiles(tt.streamSettings)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFinalMaskXmcProfiles(%q) error = %v, wantErr %v", tt.streamSettings, err, tt.wantErr)
			}
		})
	}
}

// TestXmcMaskProfilesCompleteMatchesCoreValidation pins the panel's predicate
// to xray-core's own loader instead of a restatement of it: every profile the
// panel accepts must build, and every one it rejects must fail to build. A
// future core release that tightens or relaxes the rules fails here rather
// than silently producing configs the core refuses to start on.
func TestXmcMaskProfilesCompleteMatchesCoreValidation(t *testing.T) {
	profiles := []struct {
		name string
		raw  string
	}{
		{name: "complete", raw: completeXmcProfile},
		{name: "undashed uuid", raw: `{"username":"Notch","uuid":"069a79f444e94726a5befca90e38aaf5","texturesValue":"dmFsdWU=","texturesSignature":"c2ln"}`},
		{name: "username at the 16 char limit", raw: `{"username":"Abcdefghijklmnop","uuid":"069a79f4-44e9-4726-a5be-fca90e38aaf5","texturesValue":"dmFsdWU=","texturesSignature":"c2ln"}`},
		{name: "username over the limit", raw: `{"username":"Abcdefghijklmnopq","uuid":"069a79f4-44e9-4726-a5be-fca90e38aaf5","texturesValue":"dmFsdWU=","texturesSignature":"c2ln"}`},
		{name: "username with a hyphen", raw: `{"username":"No-tch","uuid":"069a79f4-44e9-4726-a5be-fca90e38aaf5","texturesValue":"dmFsdWU=","texturesSignature":"c2ln"}`},
		{name: "empty uuid", raw: `{"username":"Notch","uuid":"","texturesValue":"dmFsdWU=","texturesSignature":"c2ln"}`},
		{name: "missing textures value", raw: `{"username":"Notch","uuid":"069a79f4-44e9-4726-a5be-fca90e38aaf5","texturesValue":"","texturesSignature":"c2ln"}`},
	}

	for _, tt := range profiles {
		t.Run(tt.name, func(t *testing.T) {
			var coreProfile conf.XMCProfile
			if err := json.Unmarshal([]byte(tt.raw), &coreProfile); err != nil {
				t.Fatalf("unmarshal into conf.XMCProfile: %v", err)
			}
			_, coreErr := coreProfile.Build()

			mask := map[string]any{"type": "xmc"}
			var settings map[string]any
			if err := json.Unmarshal([]byte(`{"profiles":[`+tt.raw+`]}`), &settings); err != nil {
				t.Fatalf("unmarshal settings: %v", err)
			}
			mask["settings"] = settings

			panelAccepts := xmcMaskProfilesComplete(mask)
			coreAccepts := coreErr == nil
			if panelAccepts != coreAccepts {
				t.Errorf("xmcMaskProfilesComplete = %v, but conf.XMCProfile.Build() accepts = %v (err %v)", panelAccepts, coreAccepts, coreErr)
			}
		})
	}
}

// TestXmcEmptyProfilesRejectedByCore covers the rule that lives on XMC rather
// than XMCProfile: v26.7.28 dropped the "default to Dream" fallback, so a mask
// with no profiles at all is now a build failure.
func TestXmcEmptyProfilesRejectedByCore(t *testing.T) {
	var core conf.XMC
	if err := json.Unmarshal([]byte(`{"hostname":"mc.example.com","password":"pw","profiles":[]}`), &core); err != nil {
		t.Fatalf("unmarshal into conf.XMC: %v", err)
	}
	if _, err := core.Build(); err == nil {
		t.Fatal("conf.XMC.Build() accepted an empty profiles list; the panel's strip/validate pair is no longer needed")
	}
}

func TestStripIncompleteXmcMasks(t *testing.T) {
	tests := []struct {
		name        string
		stream      string
		wantDropped int
		wantStream  string
	}{
		{
			name:        "legacy usernames mask is dropped and finalmask removed",
			stream:      `{"network":"tcp","finalmask":{"tcp":[{"type":"xmc","settings":{"usernames":["Dream"],"password":"pw"}}]}}`,
			wantDropped: 1,
			wantStream:  `{"network":"tcp"}`,
		},
		{
			name:        "complete mask is kept",
			stream:      `{"finalmask":{"tcp":[{"type":"xmc","settings":{"password":"pw","profiles":[` + completeXmcProfile + `]}}]}}`,
			wantDropped: 0,
			wantStream:  `{"finalmask":{"tcp":[{"type":"xmc","settings":{"password":"pw","profiles":[` + completeXmcProfile + `]}}]}}`,
		},
		{
			name:        "sibling masks survive the drop",
			stream:      `{"finalmask":{"tcp":[{"type":"xmc","settings":{"usernames":["Dream"]}},{"type":"fragment","settings":{"packets":"tlshello"}}]}}`,
			wantDropped: 1,
			wantStream:  `{"finalmask":{"tcp":[{"type":"fragment","settings":{"packets":"tlshello"}}]}}`,
		},
		{
			name:        "udp masks are preserved when tcp empties out",
			stream:      `{"finalmask":{"tcp":[{"type":"xmc","settings":{}}],"udp":[{"type":"salamander"}]}}`,
			wantDropped: 1,
			wantStream:  `{"finalmask":{"udp":[{"type":"salamander"}]}}`,
		},
		{
			name:        "stream without finalmask is untouched",
			stream:      `{"network":"tcp","security":"tls"}`,
			wantDropped: 0,
			wantStream:  `{"network":"tcp","security":"tls"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stream map[string]any
			if err := json.Unmarshal([]byte(tt.stream), &stream); err != nil {
				t.Fatalf("unmarshal stream: %v", err)
			}

			dropped := stripIncompleteXmcMasks(stream)
			if dropped != tt.wantDropped {
				t.Errorf("stripIncompleteXmcMasks dropped = %d, want %d", dropped, tt.wantDropped)
			}

			var want map[string]any
			if err := json.Unmarshal([]byte(tt.wantStream), &want); err != nil {
				t.Fatalf("unmarshal wantStream: %v", err)
			}
			got, err := json.Marshal(stream)
			if err != nil {
				t.Fatalf("marshal stream: %v", err)
			}
			wantJSON, err := json.Marshal(want)
			if err != nil {
				t.Fatalf("marshal want: %v", err)
			}
			if string(got) != string(wantJSON) {
				t.Errorf("stream after strip = %s, want %s", got, wantJSON)
			}
		})
	}
}
