package tgbot

import "testing"

// The blob is written by us but read back from a database an operator can edit,
// so a damaged value must degrade to "nothing recorded" rather than fail a pass.
func TestParseClientBlob(t *testing.T) {
	tests := []struct {
		name string
		blob string
		want map[string]int64
	}{
		{name: "empty string", blob: "", want: map[string]int64{}},
		{name: "empty object", blob: "{}", want: map[string]int64{}},
		{name: "one client", blob: `{"a@x":17}`, want: map[string]int64{"a@x": 17}},
		{name: "several clients", blob: `{"a@x":17,"b@x":-3}`, want: map[string]int64{"a@x": 17, "b@x": -3}},
		{name: "malformed json", blob: `{"a@x":`, want: map[string]int64{}},
		{name: "not an object", blob: `["a@x"]`, want: map[string]int64{}},
		{name: "wrong value type", blob: `{"a@x":"soon"}`, want: map[string]int64{}},
		{name: "empty key is dropped", blob: `{"":17}`, want: map[string]int64{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseClientBlob(tc.blob)
			if len(got) != len(tc.want) {
				t.Fatalf("parseClientBlob(%q) = %v, want %v", tc.blob, got, tc.want)
			}
			for key, value := range tc.want {
				if got[key] != value {
					t.Fatalf("parseClientBlob(%q)[%q] = %d, want %d", tc.blob, key, got[key], value)
				}
			}
		})
	}
}

func TestClientStoreRoundTrip(t *testing.T) {
	initLangDB(t)
	bot := new(Tgbot)

	if _, ok := renewOptOut.get(bot, "a@x"); ok {
		t.Fatal("an untouched store reported a value")
	}

	if err := renewOptOut.put(bot, "a@x", 1700); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, ok := renewOptOut.get(bot, "a@x")
	if !ok || got != 1700 {
		t.Fatalf("get = (%d, %v), want (1700, true)", got, ok)
	}

	if err := renewOptOut.put(bot, "b@x", 1800); err != nil {
		t.Fatalf("put: %v", err)
	}
	if got, ok := renewOptOut.get(bot, "a@x"); !ok || got != 1700 {
		t.Fatalf("second put overwrote the first: got (%d, %v)", got, ok)
	}

	if err := renewOptOut.drop(bot, "a@x"); err != nil {
		t.Fatalf("drop: %v", err)
	}
	if _, ok := renewOptOut.get(bot, "a@x"); ok {
		t.Fatal("dropped key is still readable")
	}
	if _, ok := renewOptOut.get(bot, "b@x"); !ok {
		t.Fatal("drop removed an unrelated key")
	}
}

// Each store must address its own settings row; sharing one would make a mute
// look like a quota warning.
func TestClientStoresAreIndependent(t *testing.T) {
	initLangDB(t)
	bot := new(Tgbot)

	if err := renewOptOut.put(bot, "a@x", 1); err != nil {
		t.Fatalf("put: %v", err)
	}
	for name, store := range map[string]*clientStore{
		"quotaWarned": quotaWarned,
		"selfResetAt": selfResetAt,
		"renewReqAt":  renewReqAt,
	} {
		if _, ok := store.get(bot, "a@x"); ok {
			t.Fatalf("%s shares a row with renewOptOut", name)
		}
	}
}
