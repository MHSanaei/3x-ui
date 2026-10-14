package tgbot

import "testing"

// The verbs that name a client are the security boundary, so a new per-client
// callback added later without a prefix here would silently never be checked.
func TestSplitClientSelfAction(t *testing.T) {
	tests := []struct {
		name string
		data string
		verb string
		arg  string
		ok   bool
	}{
		{name: "one link", data: "client_one_link owner@x", verb: "client_one_link", arg: "owner@x", ok: true},
		{name: "indexed link", data: "link_one owner@x 2", verb: "link_one", arg: "owner@x 2", ok: true},
		{name: "no target", data: "client_one_link "},
		{name: "bare verb", data: "client_one_link"},
		{name: "an admin callback", data: "client_cancel owner@x"},
		{name: "empty", data: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			verb, arg, ok := splitClientSelfAction(tc.data)
			if ok != tc.ok {
				t.Fatalf("splitClientSelfAction(%q) ok = %v, want %v", tc.data, ok, tc.ok)
			}
			if !ok {
				return
			}
			if verb != tc.verb || arg != tc.arg {
				t.Fatalf("= (%q, %q), want (%q, %q)", verb, arg, tc.verb, tc.arg)
			}
		})
	}
}

// link_one carries an index after the email, and that index is attacker-chosen,
// so only the email may reach the ownership check.
func TestClientSelfTargetDropsTheIndex(t *testing.T) {
	tests := []struct {
		verb string
		arg  string
		want string
	}{
		{verb: "link_one", arg: "owner@x 3", want: "owner@x"},
		{verb: "link_one", arg: "  owner@x  0  ", want: "owner@x"},
		{verb: "client_one_link", arg: "owner@x", want: "owner@x"},
		{verb: "client_one_link", arg: " owner@x ", want: "owner@x"},
		{verb: "client_one_link", arg: "  ", want: ""},
	}

	for _, tc := range tests {
		if got := clientSelfTarget(tc.verb, tc.arg); got != tc.want {
			t.Errorf("clientSelfTarget(%q, %q) = %q, want %q", tc.verb, tc.arg, got, tc.want)
		}
	}
}

// A negative or unparsable index must not reach the link list, where it would
// be a read past the start of a slice.
func TestSplitClientLinkIndex(t *testing.T) {
	tests := []struct {
		name  string
		arg   string
		email string
		index int
		ok    bool
	}{
		{name: "plain", arg: "owner@x 2", email: "owner@x", index: 2, ok: true},
		{name: "zero is valid", arg: "owner@x 0", email: "owner@x", index: 0, ok: true},
		{name: "surrounding space", arg: " owner@x  7 ", email: "owner@x", index: 7, ok: true},
		{name: "negative", arg: "owner@x -1"},
		{name: "not a number", arg: "owner@x two"},
		{name: "no index", arg: "owner@x"},
		{name: "no email", arg: " 3"},
		{name: "empty", arg: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			email, index, ok := splitClientLinkIndex(tc.arg)
			if ok != tc.ok {
				t.Fatalf("splitClientLinkIndex(%q) ok = %v, want %v", tc.arg, ok, tc.ok)
			}
			if !ok {
				return
			}
			if email != tc.email || index != tc.index {
				t.Fatalf("= (%q, %d), want (%q, %d)", email, index, tc.email, tc.index)
			}
		})
	}
}

// A stranger's tap must be refused, so an empty or nonsensical id never matches
// a client even when the database happens to hold one.
func TestOwnsClientRefusesUnusableInput(t *testing.T) {
	tg := &Tgbot{}
	for _, tc := range []struct {
		name  string
		tgID  int64
		email string
	}{
		{name: "no email", tgID: 4242},
		{name: "no telegram id", tgID: 0, email: "owner@x"},
		{name: "negative telegram id", tgID: -1, email: "owner@x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tg.ownsClient(tc.tgID, tc.email) {
				t.Fatalf("ownsClient(%d, %q) accepted an unusable request", tc.tgID, tc.email)
			}
		})
	}
}

// Only an admin sees the panel-side advice; everyone else is told to ask for an
// invite link, since an admin tapping a client button has the panel to go to.
func TestNoBoundClientMsgDependsOnAdmin(t *testing.T) {
	tg := &Tgbot{}
	if tg.noBoundClientMsg(true) == tg.noBoundClientMsg(false) {
		t.Fatal("admin and customer were given the same advice")
	}
	if tg.noBoundClientMsg(false) == "" {
		t.Fatal("customer advice resolved to an empty string")
	}
}
