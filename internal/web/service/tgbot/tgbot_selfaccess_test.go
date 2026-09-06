package tgbot

import (
	"testing"
)

// The IDOR this guards: a customer who guesses another's client name must not be able
// to fetch that client's subscription URL, which would hand over their whole config.
func TestOwnsClientRefusesAnotherCustomersClient(t *testing.T) {
	initInviteDB(t)
	seedClient(t, "mine@x", "subowner000000001", 1001)
	seedClient(t, "theirs@x", "subother000000002", 2002)
	seedClient(t, "unbound@x", "subnobody00000003", 0)

	tg := &Tgbot{}

	if !tg.ownsClient(1001, "mine@x") {
		t.Fatal("a customer must reach their own client")
	}
	if tg.ownsClient(1001, "theirs@x") {
		t.Fatal("IDOR: a customer reached another customer's client")
	}
	if tg.ownsClient(1001, "unbound@x") {
		t.Fatal("IDOR: a customer reached a client bound to nobody")
	}
	if tg.ownsClient(2002, "mine@x") {
		t.Fatal("IDOR: ownership is not symmetric")
	}
	if tg.ownsClient(9999, "mine@x") {
		t.Fatal("IDOR: an account holding nothing reached a client")
	}
	if tg.ownsClient(1001, "") {
		t.Fatal("an empty target must never be treated as owned")
	}
	if tg.ownsClient(0, "mine@x") {
		t.Fatal("an absent caller must never be treated as an owner")
	}
}

// The guard checks whatever this returns, so an extraction bug would check the
// wrong string. qr_one is the one action whose argument is not just an email.
func TestClientSelfTargetExtraction(t *testing.T) {
	tests := []struct {
		verb string
		arg  string
		want string
	}{
		{"client_sub_links", "alice@x", "alice@x"},
		{"client_individual_links", "  alice@x  ", "alice@x"},
		{"qr_pick", "alice@x", "alice@x"},
		{"qr_one", "alice@x 3", "alice@x"},
		{"qr_one", "  alice@x 12  ", "alice@x"},
		{"qr_one", "alice@x", "alice@x"},
		{"client_one_link", "alice@x", "alice@x"},
		{"link_one", "alice@x 3", "alice@x"},
		{"link_one", "  alice@x 12  ", "alice@x"},
		{"link_one", "alice@x", "alice@x"},
	}
	for _, tc := range tests {
		if got := clientSelfTarget(tc.verb, tc.arg); got != tc.want {
			t.Errorf("clientSelfTarget(%q, %q) = %q, want %q", tc.verb, tc.arg, got, tc.want)
		}
	}
}

// The level gate and the ownership gate read the same list, so an action can
// never be admitted by one and skipped by the other.
func TestClientSelfActionAndGateAgree(t *testing.T) {
	for _, prefix := range clientSelfPrefixes {
		data := prefix + "alice@x"
		if !isClientSelfCallback(data) {
			t.Errorf("%q is dispatchable but the level gate denies it", data)
		}
		if _, _, ok := clientSelfAction(data); !ok {
			t.Errorf("%q passes the level gate but does not parse", data)
		}
	}

	// Admin-only callbacks must match neither.
	for _, data := range []string{
		"admin_panel", "invite_links", "client_edit alice@x", "client_delete alice@x",
		"server_inbound_toggle 1", "tgid_remove alice@x", "reset_cred alice@x",
		"client_get_usage alice@x", "get_backup",
	} {
		if isClientSelfCallback(data) {
			t.Errorf("%q is admin-only but the level gate admits it", data)
		}
		if _, _, ok := clientSelfAction(data); ok {
			t.Errorf("%q is admin-only but parses as a self action", data)
		}
	}
}
