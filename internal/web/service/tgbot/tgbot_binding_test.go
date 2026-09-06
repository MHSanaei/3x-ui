package tgbot

import (
	"strconv"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// The limit arrives as attacker-controlled callback data, so anything outside
// the range the menu offers must be refused rather than stored as a ceiling.
func TestParseBindLimitCallback(t *testing.T) {
	tests := []struct {
		name string
		data string
		want int
		ok   bool
	}{
		{name: "a listed choice", data: "set_bindmax 5", want: 5, ok: true},
		{name: "one restores the old rule", data: "set_bindmax 1", want: 1, ok: true},
		{name: "zero is unlimited", data: "set_bindmax 0", want: 0, ok: true},
		{name: "the ceiling itself", data: "set_bindmax 50", want: 50, ok: true},
		{name: "surrounding space", data: "set_bindmax  7 ", want: 7, ok: true},
		{name: "above the ceiling", data: "set_bindmax 51"},
		{name: "negative", data: "set_bindmax -1"},
		{name: "not a number", data: "set_bindmax lots"},
		{name: "empty", data: "set_bindmax "},
		{name: "another callback", data: "set_hour 8"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			limit, ok := parseBindLimitCallback(tc.data)
			if ok != tc.ok {
				t.Fatalf("parseBindLimitCallback(%q) ok = %v, want %v", tc.data, ok, tc.ok)
			}
			if ok && limit != tc.want {
				t.Fatalf("limit = %d, want %d", limit, tc.want)
			}
		})
	}
}

// A damaged or hand-edited row must fall back to the bounded default. Falling
// back to zero would read as unlimited and quietly remove the ceiling.
func TestMaxBindingsFallsBackToTheDefault(t *testing.T) {
	initInviteDB(t)
	tg := &Tgbot{}

	if err := tg.settingService.SetTgBotMaxBindings(maxBindingsCeiling + 1); err != nil {
		t.Fatalf("SetTgBotMaxBindings: %v", err)
	}
	if got := tg.maxBindings(); got != defaultMaxBindings {
		t.Fatalf("maxBindings = %d, want the default %d", got, defaultMaxBindings)
	}

	if err := tg.settingService.SetTgBotMaxBindings(3); err != nil {
		t.Fatalf("SetTgBotMaxBindings: %v", err)
	}
	if got := tg.maxBindings(); got != 3 {
		t.Fatalf("maxBindings = %d, want 3", got)
	}
}

// One subscription can span several inbounds. Counting its records instead of
// the subscription would refuse a household at a third of its stated limit.
func TestCountSubscriptions(t *testing.T) {
	tests := []struct {
		name    string
		records []*model.ClientRecord
		want    int
	}{
		{name: "nothing held", want: 0},
		{name: "one client", records: []*model.ClientRecord{{SubID: "a"}}, want: 1},
		{
			name:    "one subscription across inbounds",
			records: []*model.ClientRecord{{SubID: "a"}, {SubID: "a"}, {SubID: "a"}},
			want:    1,
		},
		{
			name:    "two subscriptions",
			records: []*model.ClientRecord{{SubID: "a"}, {SubID: "a"}, {SubID: "b"}},
			want:    2,
		},
		{
			name:    "a client with no subId cannot be shared, so it counts alone",
			records: []*model.ClientRecord{{SubID: ""}, {SubID: ""}, {SubID: "a"}},
			want:    3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := countSubscriptions(tc.records); got != tc.want {
				t.Fatalf("countSubscriptions = %d, want %d", got, tc.want)
			}
		})
	}
}

// Zero is stored as "no ceiling", so showing it as the number zero would read
// to an admin as "nobody may bind anything".
func TestBindLimitLabelShowsUnlimitedAsASymbol(t *testing.T) {
	if got := bindLimitLabel(0); got != "∞" {
		t.Fatalf("bindLimitLabel(0) = %q, want the unlimited symbol", got)
	}
	if got := bindLimitLabel(5); got != "5" {
		t.Fatalf("bindLimitLabel(5) = %q, want \"5\"", got)
	}
}

// The picker must say which ceiling is in force and must not be the dead end
// the Server menu once was.
func TestBindLimitKeyboardMarksCurrentAndOffersAWayBack(t *testing.T) {
	initLangDB(t)
	tg := new(Tgbot)

	markup := tg.bindLimitKeyboard(6)
	marked := 0
	for _, row := range markup.InlineKeyboard {
		for _, button := range row {
			if strings.HasPrefix(button.Text, languageMark) {
				marked++
				if button.Text != languageMark+"6" {
					t.Fatalf("marked %q, want the current limit", button.Text)
				}
			}
		}
	}
	if marked != 1 {
		t.Fatalf("%d buttons marked, want exactly the current limit", marked)
	}

	data := callbackData(markup)
	if !contains(data, "admin_settings") {
		t.Fatalf("the limit picker has no way back: %v", data)
	}
	for _, choice := range bindLimitChoices {
		if _, ok := parseBindLimitCallback(bindLimitCallbackPrefix + strconv.Itoa(choice)); !ok {
			t.Fatalf("choice %d renders a callback its own parser refuses", choice)
		}
	}
}
