package mcp

import (
	"net/url"
	"testing"
)

var serverToolCases = []toolCase{
	{
		name: "get_server_status", tool: "get_server_status", args: map[string]any{},
		wantMethod: "GET", wantPath: "/panel/api/server/status",
	},
	{
		name: "list_xray_versions", tool: "list_xray_versions", args: map[string]any{},
		wantMethod: "GET", wantPath: "/panel/api/server/getXrayVersion",
	},
	{
		name: "manage_xray restart", tool: "manage_xray", args: map[string]any{"action": "restart"},
		wantMethod: "POST", wantPath: "/panel/api/server/restartXrayService",
	},
	{
		name: "manage_xray stop", tool: "manage_xray", args: map[string]any{"action": "stop"},
		wantMethod: "POST", wantPath: "/panel/api/server/stopXrayService",
	},
	{
		name: "manage_xray install", tool: "manage_xray", args: map[string]any{"action": "install", "version": "v25.10.15"},
		wantMethod: "POST", wantPath: "/panel/api/server/installXray/v25.10.15",
	},
	{
		name: "get_logs panel defaults", tool: "get_logs", args: map[string]any{"source": "panel"},
		wantMethod: "POST", wantPath: "/panel/api/server/logs/100", wantBody: "level=info&syslog=false",
	},
	{
		name: "get_logs panel syslog", tool: "get_logs", args: map[string]any{"source": "panel", "count": 20, "level": "warning", "syslog": true},
		wantMethod: "POST", wantPath: "/panel/api/server/logs/20", wantBody: "level=warning&syslog=true",
	},
	{
		name: "get_logs xray", tool: "get_logs", args: map[string]any{"source": "xray", "count": 50, "filter": "alice", "hide_direct": true},
		wantMethod: "POST", wantPath: "/panel/api/server/xraylogs/50", wantBody: "filter=alice&showBlocked=true&showDirect=false&showProxy=true",
	},
	{
		name: "get_xray_config running", tool: "get_xray_config", args: map[string]any{"kind": "running"},
		wantMethod: "GET", wantPath: "/panel/api/server/getConfigJson",
	},
	{
		name: "get_xray_config template", tool: "get_xray_config", args: map[string]any{"kind": "template"},
		wantMethod: "POST", wantPath: "/panel/api/xray/",
	},
	{
		name: "get_xray_config default", tool: "get_xray_config", args: map[string]any{"kind": "default"},
		wantMethod: "GET", wantPath: "/panel/api/xray/getDefaultJsonConfig",
	},
	{
		name: "update_xray_template", tool: "update_xray_template",
		args:       map[string]any{"xray_setting": map[string]any{"log": map[string]any{"loglevel": "warning"}}, "outbound_test_url": "https://example.com/204"},
		wantMethod: "POST", wantPath: "/panel/api/xray/update",
		wantBody: url.Values{"xraySetting": {`{"log":{"loglevel":"warning"}}`}, "outboundTestUrl": {"https://example.com/204"}}.Encode(),
	},
	{
		name: "get_outbounds_traffic", tool: "get_outbounds_traffic", args: map[string]any{},
		wantMethod: "GET", wantPath: "/panel/api/xray/getOutboundsTraffic",
	},
	{
		name: "test_outbounds", tool: "test_outbounds",
		args:       map[string]any{"outbounds": []any{map[string]any{"tag": "direct", "protocol": "freedom"}}, "mode": "tcp"},
		wantMethod: "POST", wantPath: "/panel/api/xray/testOutbounds",
		wantBody: url.Values{"outbounds": {`[{"protocol":"freedom","tag":"direct"}]`}, "mode": {"tcp"}}.Encode(),
	},
	{
		name: "generate_credentials uuid", tool: "generate_credentials", args: map[string]any{"kind": "uuid"},
		wantMethod: "GET", wantPath: "/panel/api/server/getNewUUID",
	},
	{
		name: "generate_credentials x25519", tool: "generate_credentials", args: map[string]any{"kind": "x25519"},
		wantMethod: "GET", wantPath: "/panel/api/server/getNewX25519Cert",
	},
	{
		name: "generate_credentials mldsa65", tool: "generate_credentials", args: map[string]any{"kind": "mldsa65"},
		wantMethod: "GET", wantPath: "/panel/api/server/getNewmldsa65",
	},
	{
		name: "generate_credentials mlkem768", tool: "generate_credentials", args: map[string]any{"kind": "mlkem768"},
		wantMethod: "GET", wantPath: "/panel/api/server/getNewmlkem768",
	},
	{
		name: "generate_credentials vless_enc", tool: "generate_credentials", args: map[string]any{"kind": "vless_enc"},
		wantMethod: "GET", wantPath: "/panel/api/server/getNewVlessEnc",
	},
	{
		name: "generate_credentials ech", tool: "generate_credentials", args: map[string]any{"kind": "ech", "sni": "example.com"},
		wantMethod: "POST", wantPath: "/panel/api/server/getNewEchCert", wantBody: "sni=example.com",
	},
	{
		name: "manage_vpn_outbound warp reg", tool: "manage_vpn_outbound", args: map[string]any{"provider": "warp", "action": "reg", "private_key": "sk", "public_key": "pk"},
		wantMethod: "POST", wantPath: "/panel/api/xray/warp/reg", wantBody: "privateKey=sk&publicKey=pk",
	},
	{
		name: "manage_vpn_outbound warp interval 0", tool: "manage_vpn_outbound", args: map[string]any{"provider": "warp", "action": "interval", "interval": 0},
		wantMethod: "POST", wantPath: "/panel/api/xray/warp/interval", wantBody: "interval=0",
	},
	{
		name: "manage_vpn_outbound nord servers", tool: "manage_vpn_outbound", args: map[string]any{"provider": "nord", "action": "servers", "country_id": "81"},
		wantMethod: "POST", wantPath: "/panel/api/xray/nord/servers", wantBody: "countryId=81",
	},
	{
		name: "manage_vpn_outbound pia addKey", tool: "manage_vpn_outbound", args: map[string]any{"provider": "pia", "action": "addKey", "hostname": "nl1.pia"},
		wantMethod: "POST", wantPath: "/panel/api/xray/pia/addKey", wantBody: "hostname=nl1.pia",
	},
}

// TestServerToolsRejectBadInput: warp/nord answer an unknown action with an empty
// success, so bad selectors must be refused before any request is sent.
func TestServerToolsRejectBadInput(t *testing.T) {
	cases := []struct {
		tool string
		args map[string]any
		want string
	}{
		{"manage_vpn_outbound", map[string]any{"provider": "warp", "action": "bogus"}, "warp action must be one of: data, config, reg, regMasque, changeIp, license, interval, del"},
		{"manage_vpn_outbound", map[string]any{"provider": "nord", "action": "addKey"}, "nord action must be one of: countries, servers, reg, setKey, data, del"},
		{"manage_vpn_outbound", map[string]any{"provider": "expressvpn", "action": "data"}, "provider must be warp, nord or pia"},
		{"manage_xray", map[string]any{"action": "start"}, "action must be restart, stop or install"},
		{"manage_xray", map[string]any{"action": "install"}, "version is required for action=install"},
		{"get_logs", map[string]any{"source": "syslog"}, "source must be panel or xray"},
		{"get_xray_config", map[string]any{"kind": "live"}, "kind must be running, template or default"},
		{"generate_credentials", map[string]any{"kind": "ech"}, "sni is required for kind=ech"},
		{"generate_credentials", map[string]any{"kind": "rsa"}, "kind must be uuid, x25519, mldsa65, mlkem768, vless_enc or ech"},
		{"update_xray_template", map[string]any{"xray_setting": map[string]any{}}, "xray_setting is required"},
		{"test_outbounds", map[string]any{"outbounds": []any{}}, "outbounds is required"},
	}
	p := newTestPanel(t)
	for _, tc := range cases {
		t.Run(tc.tool+" "+tc.want, func(t *testing.T) {
			p.requests = nil
			text, isErr := p.callTool(t, tc.tool, tc.args)
			if !isErr || text != tc.want {
				t.Fatalf("got (%q, isErr=%v), want (%q, true)", text, isErr, tc.want)
			}
			if len(p.requests) != 0 {
				t.Fatalf("rejected input still reached the API: %+v", p.requests)
			}
		})
	}
}
