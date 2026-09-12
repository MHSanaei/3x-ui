package tgbot

import (
	"strconv"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Writes a raw value the typed setter would refuse, which is what an operator
// editing the database by hand can leave behind.
func seedRawSetting(t *testing.T, key, value string) {
	t.Helper()
	db := database.GetDB()
	if err := db.Where("key = ?", key).Delete(&model.Setting{}).Error; err != nil {
		t.Fatalf("clear %s: %v", key, err)
	}
	if err := db.Create(&model.Setting{Key: key, Value: value}).Error; err != nil {
		t.Fatalf("seed %s: %v", key, err)
	}
}

// The hour arrives as callback data. Anything off a clock must be refused
// before it is stored, or the pass gates on an hour that never comes.
func TestParseHourCallback(t *testing.T) {
	tests := []struct {
		name string
		data string
		want int
		ok   bool
	}{
		{name: "midnight", data: "set_hour 0", want: 0, ok: true},
		{name: "the default", data: "set_hour 8", want: 8, ok: true},
		{name: "last hour", data: "set_hour 23", want: 23, ok: true},
		{name: "past the clock", data: "set_hour 24", ok: false},
		{name: "negative", data: "set_hour -1", ok: false},
		{name: "empty", data: "set_hour ", ok: false},
		{name: "not a number", data: "set_hour noon", ok: false},
		{name: "float", data: "set_hour 8.5", ok: false},
		{name: "huge", data: "set_hour 99999999999999999999", ok: false},
		{name: "different verb", data: "set_lang 8", ok: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseHourCallback(tc.data)
			if ok != tc.ok {
				t.Fatalf("parseHourCallback(%q) ok = %v, want %v", tc.data, ok, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("parseHourCallback(%q) = %d, want %d", tc.data, got, tc.want)
			}
		})
	}
}

// A damaged settings row must fall back to the documented default rather than
// to midnight, which would wake customers in the middle of the night.
func TestDailyHourFallsBackToEight(t *testing.T) {
	initLangDB(t)
	tg := new(Tgbot)

	if got := tg.dailyHour(); got != defaultDailyHour {
		t.Fatalf("stock panel hour = %d, want %d", got, defaultDailyHour)
	}

	for _, bad := range []string{"25", "-3", "", "noon"} {
		t.Run("stored "+strconv.Quote(bad), func(t *testing.T) {
			seedRawSetting(t, "tgBotDailyHour", bad)
			if got := tg.dailyHour(); got != defaultDailyHour {
				t.Fatalf("hour with %q stored = %d, want %d", bad, got, defaultDailyHour)
			}
		})
	}

	if err := tg.settingService.SetTgBotDailyHour(21); err != nil {
		t.Fatalf("SetTgBotDailyHour: %v", err)
	}
	if got := tg.dailyHour(); got != 21 {
		t.Fatalf("hour = %d, want 21", got)
	}
}

// A whole day must be reachable, and the picker must show which hour is live
// or an admin cannot tell whether their choice took.
func TestHourKeyboardCoversTheDay(t *testing.T) {
	tg := &Tgbot{}
	data := callbackData(tg.hourKeyboard(8))

	for hour := range 24 {
		want := "set_hour " + strconv.Itoa(hour)
		if !contains(data, want) {
			t.Fatalf("picker is missing %q", want)
		}
	}

	var marked []string
	for _, label := range buttonLabels(tg.hourKeyboard(8)) {
		if strings.HasPrefix(label, languageMark) {
			marked = append(marked, label)
		}
	}
	if len(marked) != 1 || !strings.Contains(marked[0], "8:00") {
		t.Fatalf("want exactly 8:00 marked, got %v", marked)
	}
}
