package tgbot

import "testing"

// A stranger must reach nothing but the greeting: any command that leaks
// through tells them the bot does more than say hello.
func TestCommandAllowedByLevel(t *testing.T) {
	tests := []struct {
		level   userLevel
		command string
		want    bool
	}{
		{levelStranger, "start", true},
		{levelStranger, "help", false},
		{levelStranger, "usage", false},
		{levelStranger, "pm", false},
		{levelStranger, "cancel", false},
		{levelStranger, "clients", false},
		{levelStranger, "server", false},
		{levelStranger, "broadcast", false},

		{levelClient, "start", true},
		{levelClient, "help", true},
		{levelClient, "usage", true},
		{levelClient, "pm", true},
		{levelClient, "cancel", true},
		{levelClient, "clients", false},
		{levelClient, "server", false},
		{levelClient, "broadcast", false},
		{levelClient, "sethelp", false},
		{levelClient, "whois", false},

		{levelAdmin, "start", true},
		{levelAdmin, "clients", true},
		{levelAdmin, "server", true},
		{levelAdmin, "broadcast", true},
	}

	for _, tc := range tests {
		if got := commandAllowed(tc.level, tc.command); got != tc.want {
			t.Errorf("commandAllowed(level %d, %q) = %v, want %v", tc.level, tc.command, got, tc.want)
		}
	}
}

// Every admin-only command reachable from the router must be denied to the two
// lower levels, so adding one without listing it cannot silently expose it.
func TestAdminCommandsAreNeverAllowedBelowAdmin(t *testing.T) {
	adminOnly := []string{
		"sethelp", "broadcast", "send", "whois", "clients", "server",
		"inbound", "restart", "clearall",
	}
	for _, command := range adminOnly {
		for _, level := range []userLevel{levelStranger, levelClient} {
			if commandAllowed(level, command) {
				t.Errorf("%q must not be allowed at level %d", command, level)
			}
		}
	}
}

// The old wording described the manual ChatID flow that invite links replaced; an
// admin with nothing bound needs pointing at the admin panel, not at themselves.
func TestNoBoundClientMsgIsLevelAware(t *testing.T) {
	tg := &Tgbot{}
	admin := tg.noBoundClientMsg(levelAdmin)
	client := tg.noBoundClientMsg(levelClient)

	if admin == client {
		t.Fatal("an admin and a customer must get different advice")
	}
	if admin != "tgbot.messages.noBoundClientAdmin" {
		t.Fatalf("admin message key = %q", admin)
	}
	if client != "tgbot.messages.noBoundClient" {
		t.Fatalf("client message key = %q", client)
	}
	if got := tg.noBoundClientMsg(levelStranger); got != client {
		t.Fatalf("stranger should get the customer wording, got %q", got)
	}
}
