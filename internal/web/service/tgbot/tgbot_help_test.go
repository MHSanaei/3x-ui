package tgbot

import (
	"testing"
)

// Without a localizer the bot helper echoes the key back, which makes it an
// unambiguous marker for "the built-in text was served".
const builtinHelpKey = "tgbot.commands.help"

func TestHelpTextFallsBackToBuiltIn(t *testing.T) {
	initInviteDB(t)
	tg := &Tgbot{}

	tests := []struct {
		name   string
		stored string
		want   string
	}{
		{"never customised", "", builtinHelpKey},
		{"cleared to blank", "   ", builtinHelpKey},
		{"customised", "Contact support at 09:00-18:00", "Contact support at 09:00-18:00"},
		{"markup preserved", "<b>Rules</b>\r\nBe nice", "<b>Rules</b>\r\nBe nice"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tg.settingService.SetTgBotHelpText(tc.stored); err != nil {
				t.Fatalf("SetTgBotHelpText: %v", err)
			}
			if got := tg.helpText(); got != tc.want {
				t.Fatalf("helpText() = %q, want %q", got, tc.want)
			}
		})
	}
}

// The key must be registered as a default, otherwise reading it before the
// operator ever writes one returns an error instead of an empty string.
func TestHelpTextOnUntouchedDatabase(t *testing.T) {
	initInviteDB(t)
	tg := &Tgbot{}

	stored, err := tg.settingService.GetTgBotHelpText()
	if err != nil {
		t.Fatalf("GetTgBotHelpText: %v", err)
	}
	if stored != "" {
		t.Fatalf("stored help text = %q, want empty", stored)
	}
	if got := tg.helpText(); got != builtinHelpKey {
		t.Fatalf("helpText() = %q, want %q", got, builtinHelpKey)
	}
}
