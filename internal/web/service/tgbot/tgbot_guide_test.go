package tgbot

import (
	"strings"
	"testing"
)

// The picker and the level gate are maintained apart, so a platform added to
// one and not the other renders a button that the gate silently drops.
func TestGuidePlatformsArePickableAndAllowed(t *testing.T) {
	tg := &Tgbot{}
	data := callbackData(tg.guideKeyboard())

	for _, platform := range guidePlatforms {
		callback := guideCallbackPrefix + platform.tag
		if !contains(data, callback) {
			t.Fatalf("picker is missing %q: %v", platform.tag, data)
		}
		if !isClientSelfCallback(callback) {
			t.Fatalf("%q is offered to a customer but the level gate rejects it", callback)
		}
		if platform.messageKey == "" {
			t.Fatalf("platform %q has no instructions to show", platform.tag)
		}
	}
	if !contains(data, "client_help") {
		t.Fatalf("guide picker has no way back to the help hub: %v", data)
	}
}

// The tag arrives as callback data, so an unknown one must resolve to nothing
// rather than reaching the localizer as an arbitrary key.
func TestParseGuideCallback(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
		ok   bool
	}{
		{name: "known platform", data: "guide_ios", want: "tgbot.messages.guideIos", ok: true},
		{name: "another platform", data: "guide_android", want: "tgbot.messages.guideAndroid", ok: true},
		{name: "unknown platform", data: "guide_symbian", ok: false},
		{name: "no platform", data: "guide_", ok: false},
		{name: "bare prefix", data: "guide", ok: false},
		{name: "traversal", data: "guide_../../etc/passwd", ok: false},
		{name: "key injection", data: "guide_tgbot.messages.adminPanel", ok: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseGuideCallback(tc.data)
			if ok != tc.ok {
				t.Fatalf("parseGuideCallback(%q) ok = %v, want %v", tc.data, ok, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("parseGuideCallback(%q) = %q, want %q", tc.data, got, tc.want)
			}
		})
	}
}

// Instructions only, per the decision that Telegram cannot make a custom-scheme
// import link tappable: a dead button is worse than a written step.
func TestGuideOffersNoLinkButtons(t *testing.T) {
	tg := &Tgbot{}
	for _, row := range tg.guideKeyboard().InlineKeyboard {
		for _, button := range row {
			if button.URL != "" {
				t.Fatalf("guide renders a link button to %q", button.URL)
			}
			if strings.Contains(button.CallbackData, "://") {
				t.Fatalf("guide smuggles a URL into callback data: %q", button.CallbackData)
			}
		}
	}
}
