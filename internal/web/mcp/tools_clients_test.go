package mcp

import "testing"

var clientToolCases = []toolCase{
	{
		name: "list_clients defaults", tool: "list_clients", args: map[string]any{},
		wantMethod: "GET", wantPath: "/panel/api/clients/list/paged",
	},
	{
		name: "list_clients filters", tool: "list_clients", args: map[string]any{"page": 2, "page_size": 50, "search": "ali", "filter": "online,expiring", "protocol": "vless", "inbound_ids": "3,5", "group": "vip", "sort": "email", "order": "desc"},
		wantMethod: "GET", wantPath: "/panel/api/clients/list/paged", wantQuery: "filter=online%2Cexpiring&group=vip&inbound=3%2C5&order=desc&page=2&pageSize=50&protocol=vless&search=ali&sort=email",
	},
	{
		name: "get_client by email", tool: "get_client", args: map[string]any{"email": "alice"},
		wantMethod: "GET", wantPath: "/panel/api/clients/get/alice",
	},
	{
		name: "get_client by sub_id", tool: "get_client", args: map[string]any{"sub_id": "sub1"},
		wantMethod: "GET", wantPath: "/panel/api/clients/get/subId/sub1",
	},
	{
		name: "get_client by tg_id", tool: "get_client", args: map[string]any{"tg_id": 123456789},
		wantMethod: "GET", wantPath: "/panel/api/clients/get/tgId/123456789",
	},
	{
		name: "save_client create", tool: "save_client", args: map[string]any{"client": map[string]any{"email": "alice", "totalGB": 1024}, "inbound_ids": []any{3, 5}},
		wantMethod: "POST", wantPath: "/panel/api/clients/add", wantBody: `{"client":{"email":"alice","totalGB":1024},"inboundIds":[3,5]}`,
	},
	{
		name: "save_client update all inbounds", tool: "save_client", args: map[string]any{"email": "alice", "client": map[string]any{"email": "alice", "enable": false}},
		wantMethod: "POST", wantPath: "/panel/api/clients/update/alice", wantBody: `{"email":"alice","enable":false}`,
	},
	{
		name: "save_client update some inbounds", tool: "save_client", args: map[string]any{"email": "alice", "client": map[string]any{"email": "alice"}, "inbound_ids": []any{3, 5}},
		wantMethod: "POST", wantPath: "/panel/api/clients/update/alice", wantQuery: "inboundIds=3%2C5", wantBody: `{"email":"alice"}`,
	},
	{
		name: "bulk_create_clients", tool: "bulk_create_clients", args: map[string]any{"clients": []any{map[string]any{"client": map[string]any{"email": "a"}, "inbound_ids": []any{1}}}},
		wantMethod: "POST", wantPath: "/panel/api/clients/bulkCreate", wantBody: `[{"client":{"email":"a"},"inboundIds":[1]}]`,
	},
	{
		name: "delete_clients one", tool: "delete_clients", args: map[string]any{"emails": []any{"alice"}, "keep_traffic": true},
		wantMethod: "POST", wantPath: "/panel/api/clients/del/alice", wantQuery: "keepTraffic=1",
	},
	{
		name: "delete_clients many", tool: "delete_clients", args: map[string]any{"emails": []any{"a", "b"}},
		wantMethod: "POST", wantPath: "/panel/api/clients/bulkDel", wantBody: `{"emails":["a","b"],"keepTraffic":false}`,
	},
	{
		name: "delete_clients depleted", tool: "delete_clients", args: map[string]any{"depleted": true},
		wantMethod: "POST", wantPath: "/panel/api/clients/delDepleted",
	},
	{
		name: "delete_clients orphans", tool: "delete_clients", args: map[string]any{"orphans": true},
		wantMethod: "POST", wantPath: "/panel/api/clients/delOrphans",
	},
	{
		name: "set_client_inbounds attach one", tool: "set_client_inbounds", args: map[string]any{"emails": []any{"alice"}, "inbound_ids": []any{7}, "action": "attach"},
		wantMethod: "POST", wantPath: "/panel/api/clients/alice/attach", wantBody: `{"inboundIds":[7]}`,
	},
	{
		name: "set_client_inbounds attach many", tool: "set_client_inbounds", args: map[string]any{"emails": []any{"a", "b"}, "inbound_ids": []any{7}, "action": "attach"},
		wantMethod: "POST", wantPath: "/panel/api/clients/bulkAttach", wantBody: `{"emails":["a","b"],"inboundIds":[7]}`,
	},
	{
		name: "set_client_inbounds detach one", tool: "set_client_inbounds", args: map[string]any{"emails": []any{"alice"}, "inbound_ids": []any{7}, "action": "detach"},
		wantMethod: "POST", wantPath: "/panel/api/clients/alice/detach", wantBody: `{"inboundIds":[7]}`,
	},
	{
		name: "set_client_inbounds detach many", tool: "set_client_inbounds", args: map[string]any{"emails": []any{"a", "b"}, "inbound_ids": []any{7}, "action": "detach"},
		wantMethod: "POST", wantPath: "/panel/api/clients/bulkDetach", wantBody: `{"emails":["a","b"],"inboundIds":[7]}`,
	},
	{
		name: "set_clients_enabled true", tool: "set_clients_enabled", args: map[string]any{"emails": []any{"a"}, "enabled": true},
		wantMethod: "POST", wantPath: "/panel/api/clients/bulkEnable", wantBody: `{"emails":["a"]}`,
	},
	{
		name: "set_clients_enabled false", tool: "set_clients_enabled", args: map[string]any{"emails": []any{"a"}, "enabled": false},
		wantMethod: "POST", wantPath: "/panel/api/clients/bulkDisable", wantBody: `{"emails":["a"]}`,
	},
	{
		name: "reset_client_traffic one", tool: "reset_client_traffic", args: map[string]any{"emails": []any{"alice"}},
		wantMethod: "POST", wantPath: "/panel/api/clients/resetTraffic/alice",
	},
	{
		name: "reset_client_traffic many", tool: "reset_client_traffic", args: map[string]any{"emails": []any{"a", "b"}},
		wantMethod: "POST", wantPath: "/panel/api/clients/bulkResetTraffic", wantBody: `{"emails":["a","b"]}`,
	},
	{
		name: "reset_client_traffic all", tool: "reset_client_traffic", args: map[string]any{"all": true},
		wantMethod: "POST", wantPath: "/panel/api/clients/resetAllTraffics",
	},
	{
		name: "adjust_clients", tool: "adjust_clients", args: map[string]any{"emails": []any{"a"}, "add_days": 30, "add_bytes": 1073741824, "limit_hwid": 0},
		wantMethod: "POST", wantPath: "/panel/api/clients/bulkAdjust", wantBody: `{"emails":["a"],"addDays":30,"addBytes":1073741824,"flow":"","limitHwid":0,"adTag":""}`,
	},
	{
		name: "get_client_links by email", tool: "get_client_links", args: map[string]any{"email": "alice"},
		wantMethod: "GET", wantPath: "/panel/api/clients/links/alice",
	},
	{
		name: "get_client_links by sub_id", tool: "get_client_links", args: map[string]any{"sub_id": "sub1"},
		wantMethod: "GET", wantPath: "/panel/api/clients/subLinks/sub1",
	},
	{
		name: "get_client_activity online", tool: "get_client_activity", args: map[string]any{"kind": "online"},
		wantMethod: "POST", wantPath: "/panel/api/clients/onlines",
	},
	{
		name: "get_client_activity last_online", tool: "get_client_activity", args: map[string]any{"kind": "last_online"},
		wantMethod: "POST", wantPath: "/panel/api/clients/lastOnline",
	},
	{
		name: "get_client_activity ips", tool: "get_client_activity", args: map[string]any{"kind": "ips", "email": "alice"},
		wantMethod: "POST", wantPath: "/panel/api/clients/ips/alice",
	},
	{
		name: "get_client_activity hwids", tool: "get_client_activity", args: map[string]any{"kind": "hwids", "email": "alice"},
		wantMethod: "POST", wantPath: "/panel/api/clients/hwids/alice",
	},
	{
		name: "set_clients_group add", tool: "set_clients_group", args: map[string]any{"emails": []any{"a"}, "group": "vip"},
		wantMethod: "POST", wantPath: "/panel/api/clients/groups/bulkAdd", wantBody: `{"emails":["a"],"group":"vip"}`,
	},
	{
		name: "set_clients_group remove", tool: "set_clients_group", args: map[string]any{"emails": []any{"a"}},
		wantMethod: "POST", wantPath: "/panel/api/clients/groups/bulkRemove", wantBody: `{"emails":["a"]}`,
	},
	{
		name: "list_client_groups", tool: "list_client_groups", args: map[string]any{},
		wantMethod: "GET", wantPath: "/panel/api/clients/groups",
	},
}

// TestClientToolsRejectBadSelectors catches a call that would hit an ambiguous
// or wrong route, or send an empty selection to the panel.
func TestClientToolsRejectBadSelectors(t *testing.T) {
	p := newTestPanel(t)
	cases := []struct {
		name, tool, want string
		args             map[string]any
	}{
		{"get_client no selector", "get_client", "pass exactly one of email, sub_id or tg_id", map[string]any{}},
		{"get_client two selectors", "get_client", "pass exactly one of email, sub_id or tg_id", map[string]any{"email": "a", "tg_id": 5}},
		{"save_client create without inbounds", "save_client", "inbound_ids is required when creating a client", map[string]any{"client": map[string]any{"email": "a"}}},
		{"delete_clients no mode", "delete_clients", "pass exactly one of emails, depleted or orphans", map[string]any{"emails": []any{}}},
		{"delete_clients two modes", "delete_clients", "pass exactly one of emails, depleted or orphans", map[string]any{"emails": []any{"a"}, "depleted": true}},
		{"set_client_inbounds bad action", "set_client_inbounds", "action must be attach or detach", map[string]any{"emails": []any{"a"}, "inbound_ids": []any{1}, "action": "move"}},
		{"set_client_inbounds empty", "set_client_inbounds", errNoClientEmails, map[string]any{"emails": []any{}, "inbound_ids": []any{1}, "action": "attach"}},
		{"set_clients_enabled empty", "set_clients_enabled", errNoClientEmails, map[string]any{"emails": []any{}, "enabled": true}},
		{"adjust_clients empty", "adjust_clients", errNoClientEmails, map[string]any{"emails": []any{}, "add_days": 30}},
		{"set_clients_group empty", "set_clients_group", errNoClientEmails, map[string]any{"emails": []any{}, "group": "vip"}},
		{"reset_client_traffic no mode", "reset_client_traffic", "pass exactly one of emails or all", map[string]any{}},
		{"reset_client_traffic two modes", "reset_client_traffic", "pass exactly one of emails or all", map[string]any{"emails": []any{"a"}, "all": true}},
		{"get_client_links no selector", "get_client_links", "pass exactly one of email or sub_id", map[string]any{}},
		{"get_client_activity bad kind", "get_client_activity", "kind must be online, last_online, ips or hwids", map[string]any{"kind": "traffic"}},
		{"get_client_activity ips without email", "get_client_activity", "email is required for kind ips", map[string]any{"kind": "ips"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p.requests = nil
			text, isErr := p.callTool(t, tc.tool, tc.args)
			if !isErr || text != tc.want {
				t.Fatalf("got (%q, isErr=%v), want error %q", text, isErr, tc.want)
			}
			if len(p.requests) != 0 {
				t.Fatalf("rejected call still reached the API: %+v", p.requests)
			}
		})
	}
}
