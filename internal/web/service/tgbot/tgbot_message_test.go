package tgbot

import (
	"testing"

	"github.com/mymmrac/telego"
)

// Only clients already bound may reach an admin: an unbound chat must resolve to no
// client, or anyone who finds the bot could page the operator.
func TestClientEmailsForOnlyResolvesBoundAccounts(t *testing.T) {
	initInviteDB(t)
	seedClient(t, "bound-a@x", "sub00000000000a", 4242)
	seedClient(t, "bound-b@x", "sub00000000000b", 4242)
	seedClient(t, "unbound@x", "sub00000000000c", 0)

	tg := &Tgbot{}

	t.Run("bound user resolves every client they own", func(t *testing.T) {
		got := tg.clientEmailsFor(4242)
		if len(got) != 2 {
			t.Fatalf("emails = %v, want both clients owned by 4242", got)
		}
	})

	t.Run("unknown user resolves nothing", func(t *testing.T) {
		if got := tg.clientEmailsFor(9999); len(got) != 0 {
			t.Fatalf("emails = %v, want none for an unbound chat", got)
		}
	})

	t.Run("zero tg id resolves nothing", func(t *testing.T) {
		if got := tg.clientEmailsFor(0); len(got) != 0 {
			t.Fatalf("emails = %v, want none; 0 marks an unclaimed client", got)
		}
	})
}

// A reply is addressed by a chat id carried in the state string, so a malformed or
// zero target must be refused rather than sent to the wrong chat, or to chat 0.
func TestParseReplyTarget(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  int64
		ok    bool
	}{
		{"valid target", "awaiting_reply:12345", 12345, true},
		{"negative group id", "awaiting_reply:-100200300", -100200300, true},
		{"zero is refused", "awaiting_reply:0", 0, false},
		{"non-numeric is refused", "awaiting_reply:abc", 0, false},
		{"empty target is refused", "awaiting_reply:", 0, false},
		{"unrelated state is refused", "awaiting_email", 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseReplyTarget(tc.state)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("target = %d, want %d", got, tc.want)
			}
		})
	}
}

// The state hook must only claim the states it owns, otherwise it would
// swallow the add-client wizard's states and break client creation.
func TestHandleConversationStateIgnoresForeignStates(t *testing.T) {
	tg := &Tgbot{}
	for _, state := range []string{"awaiting_email", "awaiting_comment", "awaiting_tg_id", ""} {
		msg := newTestMessage(1, 1)
		if tg.handleConversationState(msg, state) {
			t.Fatalf("state %q was consumed; it belongs to the add-client wizard", state)
		}
	}
}

func newTestMessage(chatID, fromID int64) *telego.Message {
	return &telego.Message{
		Chat: telego.Chat{ID: chatID},
		From: &telego.User{ID: fromID},
	}
}

// /cancel has to name the flow it ended: the sender's question is whether the
// message went out, and a bare "cancelled" does not answer it.
func TestCancelledKey(t *testing.T) {
	tests := []struct {
		name    string
		pending string
		want    string
	}{
		{"pending message to admin", statePmText, "tgbot.messages.pmCancelled"},
		{"pending admin reply", stateReplyPrefix + "12345", "tgbot.messages.replyCancelled"},
		{"another flow", stateBroadcast, "tgbot.messages.cancelled"},
		{"nothing pending", "", "tgbot.messages.cancelNothing"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cancelledKey(tc.pending); got != tc.want {
				t.Fatalf("cancelledKey(%q) = %q, want %q", tc.pending, got, tc.want)
			}
		})
	}
}

// take is what lets /cancel report accurately: a read that left the state
// behind would send the next typed line to the admin the user just cancelled.
func TestTakeClearsAndReturnsState(t *testing.T) {
	userStateMgr.reset()
	t.Cleanup(userStateMgr.reset)

	userStateMgr.set(77, statePmText)
	if state, ok := userStateMgr.take(77); !ok || state != statePmText {
		t.Fatalf("take = (%q, %v), want the pending state", state, ok)
	}
	if state, ok := userStateMgr.take(77); ok || state != "" {
		t.Fatalf("take after take = (%q, %v), want the entry gone", state, ok)
	}
}
