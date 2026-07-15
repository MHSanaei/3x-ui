package tgbot

import (
	"testing"

	"github.com/mymmrac/telego"
)

// A non-admin callback must never reach a privileged handler. The second
// callback switch runs outside the isAdmin guard, so without the default-deny
// check a non-admin who can tap an admin's inline button (e.g. in a group) could
// export the database backup or reset all traffic. Here the privileged handler
// would panic on the nil bot/services of a bare Tgbot; the guard must return
// first, so no panic occurs.
func TestAnswerCallbackDeniesPrivilegedActionToNonAdmin(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a non-admin callback reached a privileged handler: %v", r)
		}
	}()

	tg := &Tgbot{}
	for _, data := range []string{"get_backup", "reset_all_traffics_c", "add_client", "onlines", "inbounds"} {
		q := &telego.CallbackQuery{
			Data:    data,
			From:    telego.User{ID: 999999},
			Message: &telego.Message{Chat: telego.Chat{ID: 1}},
		}
		tg.answerCallback(q, false)
	}
}

func TestIsClientSelfCallback(t *testing.T) {
	allowed := []string{"client_traffic", "client_sub_links", "client_qr_links", "client_sub_links alice@x"}
	for _, d := range allowed {
		if !isClientSelfCallback(d) {
			t.Errorf("%q should be a per-user client callback", d)
		}
	}
	denied := []string{"get_backup", "reset_all_traffics_c", "add_client", "onlines", "get_banlogs", "get_usage"}
	for _, d := range denied {
		if isClientSelfCallback(d) {
			t.Errorf("%q is an admin-only callback and must not be treated as per-user", d)
		}
	}
}
