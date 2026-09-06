package tgbot

import (
	"slices"
	"testing"

	"github.com/mymmrac/telego"
)

// The five sections the admin panel itself opens.
func adminSections(t *Tgbot) map[string]*telego.InlineKeyboardMarkup {
	return map[string]*telego.InlineKeyboardMarkup{
		"admin_clients":   t.adminClientsKeyboard(),
		"admin_reports":   t.adminReportsKeyboard(),
		"admin_messaging": t.adminMessagingKeyboard(),
		"admin_settings":  t.adminSettingsKeyboard(),
		"server":          t.serverKeyboard(true),
	}
}

// Those plus the menus nested inside them.
func adminMenus(t *Tgbot) map[string]*telego.InlineKeyboardMarkup {
	menus := adminSections(t)
	menus["bulk_menu"] = t.bulkKeyboard()
	return menus
}

// The console is admin-only by default-deny: a callback reaches a customer only
// by passing isClientSelfCallback, so no button in here may.
func TestAdminMenusRenderNothingCustomerReachable(t *testing.T) {
	initLangDB(t)
	tg := new(Tgbot)

	for section, markup := range adminMenus(tg) {
		for _, data := range callbackData(markup) {
			if isClientSelfCallback(data) {
				t.Errorf("%s renders %q, which a customer may fire", section, data)
			}
		}
	}
}

// The Server menu used to be the one dead end in the bot. Every section must
// offer a way back out of it.
func TestAdminMenusAllHaveAWayBack(t *testing.T) {
	initLangDB(t)
	tg := new(Tgbot)

	for section, markup := range adminMenus(tg) {
		data := callbackData(markup)
		if !contains(data, "admin_panel") && !contains(data, "admin_clients") {
			t.Errorf("%s has no way back: %v", section, data)
		}
	}
}

// The top level must open every section, and the sections between them must
// still offer every admin action, so a reshuffle cannot strand one.
func TestAdminPanelReachesEveryAction(t *testing.T) {
	initLangDB(t)
	tg := new(Tgbot)

	top := callbackData(tg.adminKeyboard())
	for section := range adminSections(tg) {
		if !contains(top, section) {
			t.Errorf("admin panel does not open %q: %v", section, top)
		}
	}

	var reachable []string
	for _, markup := range adminMenus(tg) {
		reachable = append(reachable, callbackData(markup)...)
	}
	for _, action := range []string{
		// Clients
		"client_roster", "add_client", "get_inbounds", "onlines", "invite_links", "bulk_menu",
		// Reports
		"get_sorted_traffic_usage_report", "inbounds", "deplete_soon", "get_banlogs",
		// Server
		"get_usage", "get_backup", "server_panel_logs", "server_xray_logs",
		"server_xray_restart", "server_inbounds", "server_panel_restart",
		// Messaging
		"broadcast", "remind_now", "set_help",
		// Bot settings
		"notify_settings", "admin_features", "settings_hour", "settings_bindings", "commands",
		// Bulk
		"bulk_preview extend", "bulk_preview disable", "del_depleted", "reset_all_traffics",
	} {
		if !slices.Contains(reachable, action) {
			t.Errorf("%q is not reachable from any admin section", action)
		}
	}
}

// Every fleet-wide action belongs behind Bulk actions, which reports its reach first;
// one left on the Clients menu put a panel-wide reset one tap from browsing clients.
func TestFleetWideActionsLiveOnlyInBulkMenu(t *testing.T) {
	initLangDB(t)
	tg := new(Tgbot)

	clients := callbackData(tg.adminClientsKeyboard())
	for _, sweep := range []string{"del_depleted", "reset_all_traffics"} {
		if contains(clients, sweep) {
			t.Errorf("%q is still on the Clients menu: %v", sweep, clients)
		}
		if !contains(callbackData(tg.bulkKeyboard()), sweep) {
			t.Errorf("%q is missing from Bulk actions", sweep)
		}
	}
}
