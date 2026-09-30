package tgbot

import "testing"

// An unknown tag must be refused rather than localized: the localizer resolves
// a missing key to "", which would send the customer a blank message.
func TestParseGuideCallback(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
		ok   bool
	}{
		{name: "ios", data: "guide_ios", want: "tgbot.messages.guideIos", ok: true},
		{name: "android", data: "guide_android", want: "tgbot.messages.guideAndroid", ok: true},
		{name: "windows", data: "guide_windows", want: "tgbot.messages.guideWindows", ok: true},
		{name: "macos", data: "guide_macos", want: "tgbot.messages.guideMacos", ok: true},
		{name: "linux", data: "guide_linux", want: "tgbot.messages.guideLinux", ok: true},
		{name: "unknown platform", data: "guide_solaris"},
		{name: "empty tag", data: "guide_"},
		{name: "another callback", data: "guide"},
		{name: "another prefix", data: "set_hour 8"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseGuideCallback(tc.data)
			if ok != tc.ok {
				t.Fatalf("parseGuideCallback(%q) ok = %v, want %v", tc.data, ok, tc.ok)
			}
			if ok && got != tc.want {
				t.Fatalf("key = %q, want %q", got, tc.want)
			}
		})
	}
}

// Every platform in the picker must have a message behind it, or the button
// resolves to a blank message in that language.
func TestEveryGuidePlatformHasItsMessage(t *testing.T) {
	for _, platform := range guidePlatforms {
		if platform.tag == "" || platform.labelKey == "" || platform.messageKey == "" {
			t.Fatalf("incomplete guide entry: %+v", platform)
		}
		if _, ok := parseGuideCallback(guideCallbackPrefix + platform.tag); !ok {
			t.Errorf("guide_%s does not resolve", platform.tag)
		}
	}
}
