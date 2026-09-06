package tgbot

import (
	"slices"
	"strings"
)

// Per-client callbacks a customer may send for their OWN client — the single source of
// truth for both the level gate and the ownership check, so neither can forget one.
var clientSelfPrefixes = []string{
	"client_sub_links ",
	"client_individual_links ",
	"client_qr_links ",
	"client_one_link ",
	"link_one ",
	"qr_sub ",
	"qr_subjson ",
	"qr_pick ",
	"qr_one ",
	"renew_req ",
	"renew_mute ",
	"client_reset_self ",
	"client_reset_self_c ",
}

func clientSelfAction(data string) (verb string, arg string, ok bool) {
	for _, prefix := range clientSelfPrefixes {
		if rest, found := strings.CutPrefix(data, prefix); found {
			return strings.TrimSuffix(prefix, " "), rest, true
		}
	}
	return "", "", false
}

// The target is always the first field; qr_one and link_one carry
// "<email> <index>", so the index must not reach the ownership check.
func clientSelfTarget(verb, arg string) string {
	if verb == "qr_one" || verb == "link_one" {
		email, _, _ := strings.Cut(strings.TrimSpace(arg), " ")
		return email
	}
	return strings.TrimSpace(arg)
}

// Callback data is attacker-controlled: a user's own client can post arbitrary bytes
// against any message we sent, so the target is re-checked against their own clients.
func (t *Tgbot) ownsClient(tgUserID int64, email string) bool {
	if email == "" || tgUserID <= 0 {
		return false
	}
	return slices.Contains(t.clientEmailsFor(tgUserID), email)
}
