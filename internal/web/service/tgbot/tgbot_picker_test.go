package tgbot

import "testing"

// Asking a customer with one config which config they mean is a question with
// a single possible answer, so the picker answers it instead.
func TestPlanPicker(t *testing.T) {
	tests := []struct {
		name   string
		emails []string
		auto   string
		offer  int
	}{
		{name: "nothing bound", emails: nil, auto: "", offer: 0},
		{name: "one config resolves itself", emails: []string{"amy@x"}, auto: "amy@x", offer: 0},
		{name: "two configs are offered", emails: []string{"amy@x", "bob@x"}, auto: "", offer: 2},
		{name: "many configs are offered", emails: []string{"a", "b", "c", "d", "e", "f", "g"}, auto: "", offer: 7},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan := planPicker(tc.emails)
			if plan.auto != tc.auto {
				t.Fatalf("planPicker(%v).auto = %q, want %q", tc.emails, plan.auto, tc.auto)
			}
			if len(plan.offer) != tc.offer {
				t.Fatalf("planPicker(%v).offer has %d entries, want %d", tc.emails, len(plan.offer), tc.offer)
			}
		})
	}
}

// The picker packs into two columns once a single column would scroll.
func TestPickerColumns(t *testing.T) {
	tests := []struct {
		count int
		want  int
	}{
		{count: 0, want: 1},
		{count: 1, want: 1},
		{count: 5, want: 1},
		{count: 6, want: 2},
		{count: 30, want: 2},
	}

	for _, tc := range tests {
		if got := pickerColumns(tc.count); got != tc.want {
			t.Fatalf("pickerColumns(%d) = %d, want %d", tc.count, got, tc.want)
		}
	}
}

// Every button a picker renders must carry the verb that ownership is checked
// against, or the choice lands somewhere the caller was never authorised for.
func TestPickerKeyboardCarriesTheVerb(t *testing.T) {
	tg := &Tgbot{}
	data := callbackData(tg.pickerKeyboard("client_qr_links", []string{"amy@x", "bob@x"}))

	for _, want := range []string{"client_qr_links amy@x", "client_qr_links bob@x"} {
		if !contains(data, want) {
			t.Fatalf("picker is missing %q: %v", want, data)
		}
	}
	for _, entry := range data {
		if _, _, ok := clientSelfAction(entry); !ok {
			t.Fatalf("picker renders %q, which is not ownership-checked", entry)
		}
	}
}
