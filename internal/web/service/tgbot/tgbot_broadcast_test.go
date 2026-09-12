package tgbot

import (
	"testing"
)

func withAdminIds(t *testing.T, ids ...int64) {
	t.Helper()
	previous := adminIds
	adminIds = ids
	t.Cleanup(func() { adminIds = previous })
}

// A customer owning several subscriptions must receive one broadcast, not one
// per subscription, and admins must not be sent their own announcement.
func TestBroadcastRecipients(t *testing.T) {
	initInviteDB(t)
	withAdminIds(t, 900)

	seedClient(t, "dup-a@x", "subbroad000000001", 500)
	seedClient(t, "dup-b@x", "subbroad000000002", 500)
	seedClient(t, "other@x", "subbroad000000003", 600)
	seedClient(t, "admin@x", "subbroad000000004", 900)
	seedClient(t, "unbound@x", "subbroad000000005", 0)

	tg := &Tgbot{}
	got := tg.broadcastRecipients()

	want := []int64{500, 600}
	if len(got) != len(want) {
		t.Fatalf("recipients = %v, want %v", got, want)
	}
	for i, id := range want {
		if got[i] != id {
			t.Fatalf("recipients = %v, want %v", got, want)
		}
	}
}

func TestBroadcastRecipientsWithoutBoundClients(t *testing.T) {
	initInviteDB(t)
	withAdminIds(t, 900)
	seedClient(t, "unbound@x", "subbroad000000006", 0)
	seedClient(t, "admin@x", "subbroad000000007", 900)

	tg := &Tgbot{}
	if got := tg.broadcastRecipients(); len(got) != 0 {
		t.Fatalf("recipients = %v, want none", got)
	}
}

// The pending draft is consumed by the send so a second confirmation tap
// cannot replay the same announcement to every customer.
func TestTakePendingBroadcastConsumesDraft(t *testing.T) {
	setPendingBroadcast(4242, "scheduled maintenance tonight")

	text, ok := takePendingBroadcast(4242)
	if !ok || text != "scheduled maintenance tonight" {
		t.Fatalf("first take = (%q, %v), want the stored draft", text, ok)
	}
	if text, ok := takePendingBroadcast(4242); ok || text != "" {
		t.Fatalf("second take = (%q, %v), want empty", text, ok)
	}
}

func TestPendingBroadcastIsPerAdmin(t *testing.T) {
	setPendingBroadcast(1, "first")
	setPendingBroadcast(2, "second")
	t.Cleanup(func() {
		takePendingBroadcast(1)
		takePendingBroadcast(2)
	})

	if text, _ := takePendingBroadcast(1); text != "first" {
		t.Fatalf("admin 1 draft = %q, want %q", text, "first")
	}
	if text, _ := takePendingBroadcast(2); text != "second" {
		t.Fatalf("admin 2 draft = %q, want %q", text, "second")
	}
}
