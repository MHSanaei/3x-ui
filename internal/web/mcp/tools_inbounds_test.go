package mcp

import "testing"

var inboundToolCases = []toolCase{
	{
		name: "list_inbounds brief", tool: "list_inbounds", args: map[string]any{},
		wantMethod: "GET", wantPath: "/panel/api/inbounds/options",
	},
	{
		name: "list_inbounds slim", tool: "list_inbounds", args: map[string]any{"view": "slim"},
		wantMethod: "GET", wantPath: "/panel/api/inbounds/list/slim",
	},
	{
		name: "list_inbounds full", tool: "list_inbounds", args: map[string]any{"view": "full"},
		wantMethod: "GET", wantPath: "/panel/api/inbounds/list",
	},
	{
		name: "get_inbound", tool: "get_inbound", args: map[string]any{"inbound_id": 7},
		wantMethod: "GET", wantPath: "/panel/api/inbounds/get/7",
	},
	{
		name: "save_inbound create", tool: "save_inbound", args: map[string]any{"inbound": map[string]any{"protocol": "vless", "port": 443, "settings": map[string]any{"decryption": "none"}}},
		wantMethod: "POST", wantPath: "/panel/api/inbounds/add", wantBody: `{"protocol":"vless","port":443,"settings":{"decryption":"none"}}`,
	},
	{
		name: "save_inbound update", tool: "save_inbound", args: map[string]any{"inbound_id": 7, "inbound": map[string]any{"remark": "x", "port": 8443}},
		wantMethod: "POST", wantPath: "/panel/api/inbounds/update/7", wantBody: `{"remark":"x","port":8443}`,
	},
	{
		name: "delete_inbounds one", tool: "delete_inbounds", args: map[string]any{"inbound_ids": []any{7}},
		wantMethod: "POST", wantPath: "/panel/api/inbounds/del/7",
	},
	{
		name: "delete_inbounds many", tool: "delete_inbounds", args: map[string]any{"inbound_ids": []any{7, 8}},
		wantMethod: "POST", wantPath: "/panel/api/inbounds/bulkDel", wantBody: `{"ids":[7,8]}`,
	},
	{
		name: "set_inbound_enabled", tool: "set_inbound_enabled", args: map[string]any{"inbound_id": 7, "enabled": false},
		wantMethod: "POST", wantPath: "/panel/api/inbounds/setEnable/7", wantBody: `{"enable":false}`,
	},
	{
		name: "reset_inbound_traffic one", tool: "reset_inbound_traffic", args: map[string]any{"inbound_id": 7},
		wantMethod: "POST", wantPath: "/panel/api/inbounds/7/resetTraffic",
	},
	{
		name: "reset_inbound_traffic all", tool: "reset_inbound_traffic", args: map[string]any{"all": true},
		wantMethod: "POST", wantPath: "/panel/api/inbounds/resetAllTraffics",
	},
}

// Bad selectors must fail locally: an unknown view must not fall back to the
// heavy full list, and an ambiguous reset must not wipe every inbound.
func TestInboundToolsRejectBadSelectors(t *testing.T) {
	p := newTestPanel(t)
	cases := []struct {
		name, tool, want string
		args             map[string]any
	}{
		{"unknown view", "list_inbounds", "view must be brief, slim or full", map[string]any{"view": "everything"}},
		{"empty delete", "delete_inbounds", "inbound_ids must not be empty", map[string]any{"inbound_ids": []any{}}},
		{"reset neither", "reset_inbound_traffic", "give exactly one of inbound_id or all", map[string]any{}},
		{"reset both", "reset_inbound_traffic", "give exactly one of inbound_id or all", map[string]any{"inbound_id": 7, "all": true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p.requests = nil
			text, isErr := p.callTool(t, tc.tool, tc.args)
			if !isErr || text != tc.want {
				t.Fatalf("got (%q, isErr=%v), want rejection %q", text, isErr, tc.want)
			}
			if len(p.requests) != 0 {
				t.Fatalf("rejected call still reached the API: %+v", p.requests)
			}
		})
	}
}
