package tgbot

// userLevel decides what the bot admits to existing at all. A stranger must not
// be able to infer that a client panel or an admin panel is there to be found.
type userLevel int

const (
	levelStranger userLevel = iota
	levelClient
	levelAdmin
)

// An admin outranks their own client bindings, so an operator who also holds a
// config still reaches the admin panel.
func (t *Tgbot) levelOf(tgUserID int64) userLevel {
	if checkAdmin(tgUserID) {
		return levelAdmin
	}
	if len(t.clientEmailsFor(tgUserID)) > 0 {
		return levelClient
	}
	return levelStranger
}

// Commands are allowlisted rather than denied one by one: a command added later
// stays invisible to strangers until it is deliberately listed here.
var commandsByLevel = map[userLevel]map[string]bool{
	levelStranger: {
		"start": true,
	},
	levelClient: {
		"start": true, "help": true, "status": true, "id": true,
		"usage": true, "pm": true, "cancel": true,
	},
}

func commandAllowed(level userLevel, command string) bool {
	if level == levelAdmin {
		return true
	}
	allowed, ok := commandsByLevel[level]
	return ok && allowed[command]
}

// Only an admin reaches a client button with nothing bound — everyone else resolves to
// levelStranger — so the advice points at the admin panel, not at asking an admin.
func (t *Tgbot) noBoundClientMsg(level userLevel) string {
	if level == levelAdmin {
		return t.I18nBot("tgbot.messages.noBoundClientAdmin")
	}
	return t.I18nBot("tgbot.messages.noBoundClient")
}
