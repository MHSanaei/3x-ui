package tgbot

import (
	"strings"
	"testing"
)

// The picker and the customer gate are maintained apart, so a platform added to
// one and not the other renders a button that the gate silently drops.
func TestGuidePlatformsArePickableAndAllowed(t *testing.T) {
	tg := &Tgbot{}
	var data []string
	for _, row := range tg.guideRows() {
		for _, button := range row {
			data = append(data, button.CallbackData)
		}
	}
	for _, callback := range data {
		if !isClientSelfCallback(callback) {
			t.Errorf("the guide offers %q, which the customer gate refuses", callback)
		}
	}
	for _, platform := range guidePlatforms {
		if !strings.Contains(strings.Join(data, " "), guideCallbackPrefix+platform.tag) {
			t.Errorf("the picker is missing %q: %v", platform.tag, data)
		}
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
			if ok != tc.ok || got != tc.want {
				t.Fatalf("parseGuideCallback(%q) = (%q, %v), want (%q, %v)", tc.data, got, ok, tc.want, tc.ok)
			}
		})
	}
}

// A customer reaches the guide from home and reads the chosen platform's steps
// on the same screen, whether they hold one config or several.
func TestCustomerOpensTheSetupGuideFromHome(t *testing.T) {
	localizeWithRealBundle(t)
	tb, _ := newScreenTgbot(t, false)
	initReportDB(t)
	setTestAdmins(t)
	seedClientForTgUser(t, 4242, "solo@x")
	seedClientForTgUser(t, 5353, "first@x")
	seedClientForTgUser(t, 5353, "second@x")

	tb.screenClientHome(4242, 4242)
	if markup := screenMarkup(t, tb, 4242); !strings.Contains(markup, `"`+cbClientGuide+`"`) || !strings.Contains(markup, `"client_one_link"`) {
		t.Errorf("the one-config home lacks Get one or the guide: %s", markup)
	}
	tb.screenClientHome(5353, 5353)
	if markup := screenMarkup(t, tb, 5353); !strings.Contains(markup, `"`+cbClientGuide+`"`) {
		t.Errorf("the several-config home lacks the guide: %s", markup)
	}

	tapClientLinks(t, tb, 4242, "guide_android")
	sc, _ := tb.screens().get(4242)
	if !strings.Contains(sc.body, "v2rayNG") {
		t.Errorf("the Android steps are not on the screen: %q", sc.body)
	}
}
