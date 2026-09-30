package tgbot

import (
	"strconv"
	"strings"
)

// runClientSelfAction turns a per-client verb into work, shared by the callback
// router and the single-config auto-selection, so a verb cannot mean one thing
// on the tap and another when it was chosen for them. The verbs main already
// serves are not routed here.
func (t *Tgbot) runClientSelfAction(chatId int64, verb, arg string) {
	switch verb {
	case "client_one_link":
		t.oneLinkPicker(chatId, arg)
	case "link_one":
		if target, index, ok := splitClientLinkIndex(arg); ok {
			t.sendOneLink(chatId, target, index)
		}
	}
}

// splitClientLinkIndex pulls "<email> <index>" out of a link_one callback. The
// index is attacker-controlled, so it never reaches the ownership check.
func splitClientLinkIndex(arg string) (string, int, bool) {
	email, raw, found := strings.Cut(strings.TrimSpace(arg), " ")
	if !found || email == "" {
		return "", 0, false
	}
	index, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || index < 0 {
		return "", 0, false
	}
	return email, index, true
}

// The customer verbs this part adds, so the router can route them and the
// ownership check can cover them from one list. The three callbacks main
// already serves (client_sub_links, client_individual_links, client_qr_links)
// are deliberately absent: they keep main's own routing and its
// clientOwnedByTgUser check, so this part does not change their behavior.
var clientSelfActionPrefixes = []string{
	"client_one_link ",
	"link_one ",
}

func splitClientSelfAction(data string) (verb, arg string, ok bool) {
	for _, prefix := range clientSelfActionPrefixes {
		if rest, found := strings.CutPrefix(data, prefix); found && rest != "" {
			return strings.TrimSuffix(prefix, " "), rest, true
		}
	}
	return "", "", false
}

// The target is the email, so link_one's trailing index is dropped before the
// ownership check compares it against the caller's own clients.
func clientSelfTarget(verb, arg string) string {
	if verb == "link_one" {
		email, _, _ := strings.Cut(strings.TrimSpace(arg), " ")
		return email
	}
	return strings.TrimSpace(arg)
}

// Callback data is attacker-controlled: a customer can post arbitrary bytes
// against any message the bot sent them, so the target is re-checked here.
func (t *Tgbot) ownsClient(tgUserID int64, email string) bool {
	if email == "" || tgUserID <= 0 {
		return false
	}
	for _, own := range t.clientEmailsFor(tgUserID) {
		if own == email {
			return true
		}
	}
	return false
}

// Only an admin reaches this with nothing bound; everyone else resolves to a
// stranger, so the advice points at the invite link rather than the panel.
func (t *Tgbot) noBoundClientMsg(isAdmin bool) string {
	if isAdmin {
		return t.I18nBot("tgbot.messages.noBoundClientAdmin")
	}
	return t.I18nBot("tgbot.messages.noBoundClient")
}
