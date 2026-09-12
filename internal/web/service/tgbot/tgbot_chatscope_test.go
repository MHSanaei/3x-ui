package tgbot

import (
	"testing"

	"github.com/mymmrac/telego"
)

// Conversation state is keyed by chat and authorization by sender, so anything
// but a private chat lets one person answer a prompt meant for another.
func TestIsPrivateChat(t *testing.T) {
	tests := []struct {
		chatType string
		want     bool
	}{
		{telego.ChatTypePrivate, true},
		{telego.ChatTypeGroup, false},
		{telego.ChatTypeSupergroup, false},
		{telego.ChatTypeChannel, false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.chatType, func(t *testing.T) {
			if got := isPrivateChat(telego.Chat{Type: tc.chatType}); got != tc.want {
				t.Fatalf("isPrivateChat(%q) = %v, want %v", tc.chatType, got, tc.want)
			}
		})
	}
}
