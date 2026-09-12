package tgbot

import (
	"testing"

	"github.com/mymmrac/telego"
)

func TestAnswerCallbackDeniesPrivilegedActionToNonAdmin(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a non-admin callback reached a privileged handler: %v", r)
		}
	}()

	tg := &Tgbot{}
	for _, data := range []string{"get_backup", "reset_all_traffics_c", "add_client", "onlines", "inbounds", "admin_panel", "invite_links", "admin_clients", "admin_reports", "admin_messaging", "set_help", "remind_now", "remind_send"} {
		for _, level := range []userLevel{levelStranger, levelClient} {
			q := &telego.CallbackQuery{
				Data:    data,
				From:    telego.User{ID: 999999},
				Message: &telego.Message{Chat: telego.Chat{ID: 1}},
			}
			tg.answerCallback(q, level)
		}
	}
}

func TestIsClientSelfCallback(t *testing.T) {
	allowed := []string{
		"client_traffic", "client_sub_links", "client_qr_links", "client_sub_links alice@x",
		"client_configs", "client_one_link", "client_usage_refresh",
		"client_one_link alice@x", "link_one alice@x 2",
	}
	for _, d := range allowed {
		if !isClientSelfCallback(d) {
			t.Errorf("%q should be a per-user client callback", d)
		}
	}
	denied := []string{
		"get_backup", "reset_all_traffics_c", "add_client", "onlines", "get_banlogs", "get_usage",
		"client_edit alice@x", "client_edit_email alice@x", "client_edit_comment alice@x",
		"client_new_subid_c alice@x", "client_delete alice@x", "client_delete_c alice@x",
		"del_depleted", "del_depleted_c",
		"server", "server_panel_logs", "server_xray_logs", "server_xray_restart",
		"server_xray_stop_c", "server_panel_restart_c", "server_inbounds", "server_inbound_toggle 1",
		"client_roster", "admin_panel", "invite_links", "admin_clients", "admin_reports", "admin_messaging", "set_help",
		"remind_now", "remind_send",
	}
	for _, d := range denied {
		if isClientSelfCallback(d) {
			t.Errorf("%q is an admin-only callback and must not be treated as per-user", d)
		}
	}
}
