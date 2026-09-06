package tgbot

import (
	"strings"
	"testing"
)

// Callback data is attacker-controlled, so any button naming a client must resolve
// through clientSelfAction; a targeted callback added without that gate is an IDOR.
func TestTargetedClientCallbacksAreOwnershipChecked(t *testing.T) {
	initLangDB(t)
	tg := &Tgbot{}
	const email = "amy@example.com"

	rendered := map[string][]string{
		"renew keyboard":  callbackData(tg.renewKeyboard(email)),
		"client keyboard": callbackData(tg.clientKeyboard(levelClient)),
		"settings":        callbackData(tg.settingsKeyboard()),
		"languages":       callbackData(tg.languageKeyboard("")),
	}

	for source, data := range rendered {
		for _, entry := range data {
			verb, arg, matched := clientSelfAction(entry)
			switch {
			case matched:
				if got := clientSelfTarget(verb, arg); got != email {
					t.Fatalf("%s: %q resolves to target %q, want %q", source, entry, got, email)
				}
			case strings.Contains(entry, email):
				t.Fatalf("%s: %q names a client but is not ownership-checked", source, entry)
			}
			if !isClientSelfCallback(entry) {
				t.Fatalf("%s: %q is rendered for a customer but the level gate rejects it", source, entry)
			}
		}
	}
}

// The two lists are the level gate and the ownership check. A prefix in one and
// not the other is admitted by one and forgotten by the other.
func TestClientSelfPrefixesPassTheLevelGate(t *testing.T) {
	for _, prefix := range clientSelfPrefixes {
		data := prefix + "amy@example.com"
		if !isClientSelfCallback(data) {
			t.Fatalf("prefix %q is ownership-checked but the level gate rejects it", prefix)
		}
		verb, arg, ok := clientSelfAction(data)
		if !ok {
			t.Fatalf("prefix %q does not match its own action parser", prefix)
		}
		if clientSelfTarget(verb, arg) == "" {
			t.Fatalf("prefix %q resolves to an empty target, so ownership cannot be checked", prefix)
		}
	}
}

// ownsClient is the whole check. An empty or zero caller must never pass, or a
// callback with a stripped sender would read as owning everything.
func TestOwnsClientRejectsMissingIdentity(t *testing.T) {
	initLangDB(t)
	tg := new(Tgbot)

	tests := []struct {
		name  string
		tgID  int64
		email string
	}{
		{name: "no telegram id", tgID: 0, email: "amy@example.com"},
		{name: "negative telegram id", tgID: -1, email: "amy@example.com"},
		{name: "no email", tgID: 42, email: ""},
		{name: "unbound client", tgID: 42, email: "amy@example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tg.ownsClient(tc.tgID, tc.email) {
				t.Fatalf("ownsClient(%d, %q) = true, want false", tc.tgID, tc.email)
			}
		})
	}
}

// Default-deny stops a non-admin reaching an admin callback: anything listed here
// must stay outside the customer allowlist, or a client could run bulk actions.
func TestAdminCallbacksAreNotCustomerReachable(t *testing.T) {
	adminOnly := []string{
		"admin_panel", "admin_clients", "admin_reports", "admin_messaging", "admin_settings",
		"admin_features", "settings_hour", "settings_bindings", "bulk_menu", "roster_search",
		"client_roster", "notify_settings", "broadcast", "set_help", "get_backup",
		"remind_now", "remind_send",
		"set_hour 8", "set_bindmax 5", "feature_toggle feature_self_reset", "notify_toggle notify_quota",
		"roster_filter unbound", "bulk_preview extend", "bulk_apply disable",
		"reset_exp amy@example.com", "client_delete_c amy@example.com",
		"toggle_enable_c amy@example.com", "pm_reply 777",
	}

	for _, data := range adminOnly {
		t.Run(data, func(t *testing.T) {
			if isClientSelfCallback(data) {
				t.Fatalf("%q is reachable by a non-admin", data)
			}
		})
	}
}

// The hour and binding-limit pickers are admin-only, so their parsers must not be
// reachable through the customer allowlist despite matching before the outer switch.
func TestHourCallbackIsNotACustomerAction(t *testing.T) {
	for _, data := range []string{
		"set_hour 0", "set_hour 8", "set_hour 23",
		"set_bindmax 0", "set_bindmax 1", "set_bindmax 50",
	} {
		if _, _, ok := clientSelfAction(data); ok {
			t.Fatalf("%q parses as a customer action", data)
		}
		if isClientSelfCallback(data) {
			t.Fatalf("%q passes the customer level gate", data)
		}
	}
}
