package mcp

import "testing"

var panelToolCases = []toolCase{
	{name: "list_nodes", tool: "list_nodes", wantMethod: "GET", wantPath: "/panel/api/nodes/list"},
	{name: "get_node", tool: "get_node", args: map[string]any{"id": 4}, wantMethod: "GET", wantPath: "/panel/api/nodes/get/4"},
	{
		name: "save_node add", tool: "save_node", args: map[string]any{"node": map[string]any{"name": "n1", "address": "10.0.0.2", "port": 2053, "apiToken": "t"}},
		wantMethod: "POST", wantPath: "/panel/api/nodes/add", wantBody: `{"name":"n1","address":"10.0.0.2","port":2053,"apiToken":"t"}`,
	},
	{
		name: "save_node update", tool: "save_node", args: map[string]any{"id": 4, "node": map[string]any{"name": "n1", "address": "10.0.0.2", "port": 2053}},
		wantMethod: "POST", wantPath: "/panel/api/nodes/update/4", wantBody: `{"name":"n1","address":"10.0.0.2","port":2053}`,
	},
	{name: "delete_node", tool: "delete_node", args: map[string]any{"id": 4}, wantMethod: "POST", wantPath: "/panel/api/nodes/del/4"},
	{name: "probe_node", tool: "probe_node", args: map[string]any{"id": 4}, wantMethod: "POST", wantPath: "/panel/api/nodes/probe/4"},

	{name: "list_hosts all", tool: "list_hosts", wantMethod: "GET", wantPath: "/panel/api/hosts/list"},
	{name: "list_hosts by inbound", tool: "list_hosts", args: map[string]any{"inbound_id": 7}, wantMethod: "GET", wantPath: "/panel/api/hosts/byInbound/7"},
	{
		name: "save_host_group add", tool: "save_host_group", args: map[string]any{"host": map[string]any{"remark": "cdn", "inboundIds": []any{7}}},
		wantMethod: "POST", wantPath: "/panel/api/hosts/add", wantBody: `{"remark":"cdn","inboundIds":[7]}`,
	},
	{
		name: "save_host_group update", tool: "save_host_group", args: map[string]any{"group_id": "g1", "host": map[string]any{"remark": "cdn", "inboundIds": []any{7}}},
		wantMethod: "POST", wantPath: "/panel/api/hosts/update/g1", wantBody: `{"remark":"cdn","inboundIds":[7]}`,
	},
	{name: "delete_host_group", tool: "delete_host_group", args: map[string]any{"group_id": "g1"}, wantMethod: "POST", wantPath: "/panel/api/hosts/del/g1"},

	{name: "get_settings", tool: "get_settings", wantMethod: "POST", wantPath: "/panel/api/setting/all"},
	{
		name: "update_settings", tool: "update_settings", args: map[string]any{"settings": map[string]any{"webPort": 2053, "clearSmtpPassword": true}},
		wantMethod: "POST", wantPath: "/panel/api/setting/update", wantBody: `{"webPort":2053,"clearSmtpPassword":true}`,
	},
	{name: "restart_panel", tool: "restart_panel", wantMethod: "POST", wantPath: "/panel/api/setting/restartPanel"},
	{name: "test_notifier smtp", tool: "test_notifier", args: map[string]any{"channel": "smtp"}, wantMethod: "POST", wantPath: "/panel/api/setting/testSmtp"},
	{name: "test_notifier telegram", tool: "test_notifier", args: map[string]any{"channel": "telegram"}, wantMethod: "POST", wantPath: "/panel/api/setting/testTgBot"},
	{name: "test_notifier discord", tool: "test_notifier", args: map[string]any{"channel": "discord"}, wantMethod: "POST", wantPath: "/panel/api/setting/testDiscord"},
	{name: "backup_to_telegram", tool: "backup_to_telegram", wantMethod: "POST", wantPath: "/panel/api/backuptotgbot"},
}

// An unknown channel must be refused before any request, not sent to a guessed route.
func TestTestNotifierRejectsUnknownChannel(t *testing.T) {
	p := newTestPanel(t)
	text, isErr := p.callTool(t, "test_notifier", map[string]any{"channel": "slack"})
	if !isErr || text != "channel must be smtp, telegram or discord" {
		t.Fatalf("got (%q, isErr=%v), want rejection", text, isErr)
	}
	if len(p.requests) != 0 {
		t.Fatalf("rejected channel still reached the API: %+v", p.requests)
	}
}
