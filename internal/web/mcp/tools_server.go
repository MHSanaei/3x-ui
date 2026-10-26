package mcp

import (
	"context"
	"encoding/json"
	"net/url"
	"slices"
	"strconv"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type serverNoInput struct{}

type manageXrayInput struct {
	Action  string `json:"action" jsonschema:"restart, stop or install"`
	Version string `json:"version,omitempty" jsonschema:"release tag for install, from list_xray_versions, e.g. v25.10.15"`
}

type logsInput struct {
	Source      string `json:"source" jsonschema:"panel (panel log) or xray (parsed access log)"`
	Count       int    `json:"count,omitempty" jsonschema:"trailing lines, e.g. 200; default 100"`
	Level       string `json:"level,omitempty" jsonschema:"panel only: minimum level debug, info, notice, warning or error; default info"`
	Syslog      bool   `json:"syslog,omitempty" jsonschema:"panel only: read the x-ui systemd journal instead (Linux)"`
	Filter      string `json:"filter,omitempty" jsonschema:"xray only: keep lines containing this text, e.g. a client email"`
	HideDirect  bool   `json:"hide_direct,omitempty" jsonschema:"xray only: drop lines routed to freedom outbounds"`
	HideBlocked bool   `json:"hide_blocked,omitempty" jsonschema:"xray only: drop lines routed to blackhole outbounds"`
	HideProxy   bool   `json:"hide_proxy,omitempty" jsonschema:"xray only: drop lines routed to other outbounds"`
}

type xrayConfigInput struct {
	Kind string `json:"kind" jsonschema:"running (generated config.json), template (editable template) or default (factory template)"`
}

type updateXrayTemplateInput struct {
	XraySetting     map[string]any `json:"xray_setting" jsonschema:"full template object: xraySetting from get_xray_config kind=template, edited"`
	OutboundTestURL string         `json:"outbound_test_url,omitempty" jsonschema:"keep outboundTestUrl from the template response; omitted resets it to https://www.google.com/generate_204"`
}

type testOutboundsInput struct {
	Outbounds    []any  `json:"outbounds" jsonschema:"outbound objects to test, max 50"`
	AllOutbounds []any  `json:"all_outbounds,omitempty" jsonschema:"all outbounds, to resolve sockopt.dialerProxy chains"`
	Mode         string `json:"mode,omitempty" jsonschema:"tcp (dial only), real (incl. tunnel setup) or empty (warm HTTP round-trip)"`
}

type credentialsInput struct {
	Kind string `json:"kind" jsonschema:"uuid, x25519 (REALITY keys), mldsa65, mlkem768, vless_enc or ech"`
	SNI  string `json:"sni,omitempty" jsonschema:"server name for kind=ech, e.g. example.com"`
}

type vpnOutboundInput struct {
	Provider    string `json:"provider" jsonschema:"warp, nord or pia"`
	Action      string `json:"action" jsonschema:"warp: data, config, reg, regMasque, changeIp, license, interval, del; nord: countries, servers, reg, setKey, data, del; pia: countries, servers, reg, addKey, data, del"`
	PrivateKey  string `json:"private_key,omitempty" jsonschema:"warp reg: WireGuard private key"`
	PublicKey   string `json:"public_key,omitempty" jsonschema:"warp reg: WireGuard public key"`
	License     string `json:"license,omitempty" jsonschema:"warp license: Warp+ key"`
	Interval    *int   `json:"interval,omitempty" jsonschema:"warp interval: hours between IP rotations, e.g. 24; 0 disables"`
	CountryID   string `json:"country_id,omitempty" jsonschema:"nord servers: id from countries, e.g. 81"`
	Token       string `json:"token,omitempty" jsonschema:"nord reg: access token"`
	Key         string `json:"key,omitempty" jsonschema:"nord setKey: NordLynx private key"`
	CountryCode string `json:"country_code,omitempty" jsonschema:"pia servers: code from countries, e.g. NL"`
	Username    string `json:"username,omitempty" jsonschema:"pia reg: username"`
	Password    string `json:"password,omitempty" jsonschema:"pia reg: password"`
	Hostname    string `json:"hostname,omitempty" jsonschema:"pia addKey: server hostname from servers"`
}

var credentialRoutes = map[string]string{
	"uuid":      "/server/getNewUUID",
	"x25519":    "/server/getNewX25519Cert",
	"mldsa65":   "/server/getNewmldsa65",
	"mlkem768":  "/server/getNewmlkem768",
	"vless_enc": "/server/getNewVlessEnc",
}

// vpnActions guards the action: warp and nord answer an unknown one with an empty success.
var vpnActions = map[string][]string{
	"warp": {"data", "config", "reg", "regMasque", "changeIp", "license", "interval", "del"},
	"nord": {"countries", "servers", "reg", "setKey", "data", "del"},
	"pia":  {"countries", "servers", "reg", "addKey", "data", "del"},
}

// nonEmptyForm keeps only non-empty fields, so the handlers' own defaults apply.
func nonEmptyForm(kv ...string) url.Values {
	v := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] != "" {
			v.Set(kv[i], kv[i+1])
		}
	}
	return v
}

func addServerTools(s *sdk.Server, d *dispatcher) {
	sdk.AddTool(s, &sdk.Tool{
		Name:        "get_server_status",
		Description: "Returns host and Xray status: CPU, memory, disk, network, load, uptime, Xray state and version.",
		Annotations: readOnly(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, _ serverNoInput) (*sdk.CallToolResult, any, error) {
		return d.call(ctx, "GET", "/server/status", nil, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "list_xray_versions",
		Description: "Returns the Xray-core versions available to install. Pass one to manage_xray action=install.",
		Annotations: readOnly(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, _ serverNoInput) (*sdk.CallToolResult, any, error) {
		return d.call(ctx, "GET", "/server/getXrayVersion", nil, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "manage_xray",
		Description: "Restarts, stops, or installs a version of the Xray core; live connections drop. If it fails, read get_logs.",
		Annotations: destructive(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in manageXrayInput) (*sdk.CallToolResult, any, error) {
		switch in.Action {
		case "restart":
			return d.call(ctx, "POST", "/server/restartXrayService", nil, nil)
		case "stop":
			return d.call(ctx, "POST", "/server/stopXrayService", nil, nil)
		case "install":
			if in.Version == "" {
				return errorResult("version is required for action=install"), nil, nil
			}
			return d.call(ctx, "POST", "/server/installXray/"+url.PathEscape(in.Version), nil, nil)
		}
		return errorResult("action must be restart, stop or install"), nil, nil
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "get_logs",
		Description: "Returns recent panel log lines or parsed Xray access-log entries. Use panel to diagnose start errors, " +
			"xray to see which client connects where.",
		Annotations: readOnly(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in logsInput) (*sdk.CallToolResult, any, error) {
		count := in.Count
		if count <= 0 {
			count = 100
		}
		switch in.Source {
		case "panel":
			level := in.Level
			if level == "" {
				level = "info"
			}
			return d.call(ctx, "POST", "/server/logs/"+strconv.Itoa(count), nil,
				url.Values{"level": {level}, "syslog": {strconv.FormatBool(in.Syslog)}})
		case "xray":
			body := url.Values{
				"showDirect":  {strconv.FormatBool(!in.HideDirect)},
				"showBlocked": {strconv.FormatBool(!in.HideBlocked)},
				"showProxy":   {strconv.FormatBool(!in.HideProxy)},
			}
			if in.Filter != "" {
				body.Set("filter", in.Filter)
			}
			return d.call(ctx, "POST", "/server/xraylogs/"+strconv.Itoa(count), nil, body)
		}
		return errorResult("source must be panel or xray"), nil, nil
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "get_xray_config",
		Description: "Returns an Xray config: the running config.json, the editable template (with inboundTags and outboundTestUrl), " +
			"or the factory default. Edit the template and save it with update_xray_template.",
		Annotations: readOnly(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in xrayConfigInput) (*sdk.CallToolResult, any, error) {
		switch in.Kind {
		case "running":
			return d.call(ctx, "GET", "/server/getConfigJson", nil, nil)
		case "template":
			return d.call(ctx, "POST", "/xray/", nil, nil)
		case "default":
			return d.call(ctx, "GET", "/xray/getDefaultJsonConfig", nil, nil)
		}
		return errorResult("kind must be running, template or default"), nil, nil
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "update_xray_template",
		Description: "Replaces the whole Xray config template and applies it to the running core. " +
			"Start from get_xray_config kind=template; kind=default gives the factory template.",
		Annotations: destructive(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in updateXrayTemplateInput) (*sdk.CallToolResult, any, error) {
		if len(in.XraySetting) == 0 {
			return errorResult("xray_setting is required"), nil, nil
		}
		raw, err := json.Marshal(in.XraySetting)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		return d.call(ctx, "POST", "/xray/update", nil, nonEmptyForm("xraySetting", string(raw), "outboundTestUrl", in.OutboundTestURL))
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "get_outbounds_traffic",
		Description: "Returns upload and download totals per outbound tag.",
		Annotations: readOnly(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, _ serverNoInput) (*sdk.CallToolResult, any, error) {
		return d.call(ctx, "GET", "/xray/getOutboundsTraffic", nil, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "test_outbounds",
		Description: "Returns delay, HTTP status and timings per outbound, in input order, probed through a temporary Xray instance. " +
			"Outbound shapes are in get_xray_config kind=template.",
		Annotations: readOnly(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in testOutboundsInput) (*sdk.CallToolResult, any, error) {
		if len(in.Outbounds) == 0 {
			return errorResult("outbounds is required"), nil, nil
		}
		outbounds, err := json.Marshal(in.Outbounds)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		all := ""
		if len(in.AllOutbounds) > 0 {
			raw, err := json.Marshal(in.AllOutbounds)
			if err != nil {
				return errorResult(err.Error()), nil, nil
			}
			all = string(raw)
		}
		return d.call(ctx, "POST", "/xray/testOutbounds", nil, nonEmptyForm("outbounds", string(outbounds), "allOutbounds", all, "mode", in.Mode))
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "generate_credentials",
		Description: "Returns fresh keys or IDs for inbound and client settings: a UUID, REALITY x25519 keys, ML-DSA-65 or ML-KEM-768 keys, " +
			"a VLESS encryption pair, or an ECH config. Nothing is saved.",
		Annotations: readOnly(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in credentialsInput) (*sdk.CallToolResult, any, error) {
		if in.Kind == "ech" {
			if in.SNI == "" {
				return errorResult("sni is required for kind=ech"), nil, nil
			}
			return d.call(ctx, "POST", "/server/getNewEchCert", nil, url.Values{"sni": {in.SNI}})
		}
		route, ok := credentialRoutes[in.Kind]
		if !ok {
			return errorResult("kind must be uuid, x25519, mldsa65, mlkem768, vless_enc or ech"), nil, nil
		}
		return d.call(ctx, "GET", route, nil, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "manage_vpn_outbound",
		Description: "Runs one action of the Cloudflare WARP, NordVPN or PIA outbound integration: list countries/servers, register, " +
			"read stored data or delete it. For warp reg, make keys with generate_credentials kind=x25519.",
		Annotations: destructive(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in vpnOutboundInput) (*sdk.CallToolResult, any, error) {
		actions, ok := vpnActions[in.Provider]
		if !ok {
			return errorResult("provider must be warp, nord or pia"), nil, nil
		}
		if !slices.Contains(actions, in.Action) {
			return errorResult(in.Provider + " action must be one of: " + strings.Join(actions, ", ")), nil, nil
		}
		interval := ""
		if in.Interval != nil {
			interval = strconv.Itoa(*in.Interval)
		}
		body := nonEmptyForm(
			"privateKey", in.PrivateKey, "publicKey", in.PublicKey, "license", in.License, "interval", interval,
			"countryId", in.CountryID, "token", in.Token, "key", in.Key,
			"countryCode", in.CountryCode, "username", in.Username, "password", in.Password, "hostname", in.Hostname)
		return d.call(ctx, "POST", "/xray/"+in.Provider+"/"+in.Action, nil, body)
	})
}
