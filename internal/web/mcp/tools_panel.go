package mcp

import (
	"context"
	"net/url"
	"strconv"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type nodeIDInput struct {
	ID int `json:"id" jsonschema:"node id from list_nodes, e.g. 4"`
}

type nodeSaveInput struct {
	ID   int            `json:"id,omitempty" jsonschema:"node id to update, e.g. 4; omit to add a new node"`
	Node map[string]any `json:"node" jsonschema:"node object: name, address, port, apiToken, enable, tlsVerifyMode (verify|skip|pin|mtls), inboundSyncMode (all|selected), inboundTags...; omit apiToken on update to keep the stored one"`
}

type hostListInput struct {
	InboundID int `json:"inbound_id,omitempty" jsonschema:"only host groups of this inbound, e.g. 7; omit for all"`
}

type hostGroupIDInput struct {
	GroupID string `json:"group_id" jsonschema:"host group id from list_hosts"`
}

type hostGroupSaveInput struct {
	GroupID string         `json:"group_id,omitempty" jsonschema:"host group id to update; omit to add a new group"`
	Host    map[string]any `json:"host" jsonschema:"host group object as list_hosts returns it; inboundIds and remark are required"`
}

type settingsUpdateInput struct {
	Settings map[string]any `json:"settings" jsonschema:"full object from get_settings with changes; blank secrets stay unchanged, clearSmtpPassword etc. remove them, twoFactorCode needed to change 2FA"`
}

type notifierTestInput struct {
	Channel string `json:"channel" jsonschema:"smtp, telegram or discord"`
}

var notifierTestRoutes = map[string]string{
	"smtp":     "/setting/testSmtp",
	"telegram": "/setting/testTgBot",
	"discord":  "/setting/testDiscord",
}

func addPanelTools(s *sdk.Server, d *dispatcher) {
	sdk.AddTool(s, &sdk.Tool{
		Name: "list_nodes", Annotations: readOnly(),
		Description: "Returns remote nodes as a tree with status, health and counts. Use get_node for one node's full settings.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, _ struct{}) (*sdk.CallToolResult, any, error) {
		return d.call(ctx, "GET", "/nodes/list", nil, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "get_node", Annotations: readOnly(),
		Description: "Returns one node's full settings (API token is write-only and never shown). Call it before save_node to edit a node.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in nodeIDInput) (*sdk.CallToolResult, any, error) {
		return d.call(ctx, "GET", "/nodes/get/"+strconv.Itoa(in.ID), nil, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "save_node", Annotations: destructive(),
		Description: "Adds a node, or replaces one when id is given. The panel probes the node first and refuses an unreachable one. On update send the whole node from get_node.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in nodeSaveInput) (*sdk.CallToolResult, any, error) {
		if in.ID == 0 {
			return d.call(ctx, "POST", "/nodes/add", nil, in.Node)
		}
		return d.call(ctx, "POST", "/nodes/update/"+strconv.Itoa(in.ID), nil, in.Node)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "delete_node", Annotations: destructive(),
		Description: "Deletes a node from this panel.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in nodeIDInput) (*sdk.CallToolResult, any, error) {
		return d.call(ctx, "POST", "/nodes/del/"+strconv.Itoa(in.ID), nil, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "probe_node", Annotations: additive(),
		Description: "Returns a saved node's live reachability and health, and records it as its status. Use to check a node after save_node.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in nodeIDInput) (*sdk.CallToolResult, any, error) {
		return d.call(ctx, "POST", "/nodes/probe/"+strconv.Itoa(in.ID), nil, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "list_hosts", Annotations: readOnly(),
		Description: "Returns host groups: the addresses and TLS overrides subscription links use for inbounds. Use before save_host_group.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in hostListInput) (*sdk.CallToolResult, any, error) {
		if in.InboundID > 0 {
			return d.call(ctx, "GET", "/hosts/byInbound/"+strconv.Itoa(in.InboundID), nil, nil)
		}
		return d.call(ctx, "GET", "/hosts/list", nil, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "save_host_group", Annotations: destructive(),
		Description: "Adds a host group, or replaces one when group_id is given. On update send the whole group from list_hosts.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in hostGroupSaveInput) (*sdk.CallToolResult, any, error) {
		if in.GroupID == "" {
			return d.call(ctx, "POST", "/hosts/add", nil, in.Host)
		}
		return d.call(ctx, "POST", "/hosts/update/"+url.PathEscape(in.GroupID), nil, in.Host)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "delete_host_group", Annotations: destructive(),
		Description: "Deletes a host group.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in hostGroupIDInput) (*sdk.CallToolResult, any, error) {
		return d.call(ctx, "POST", "/hosts/del/"+url.PathEscape(in.GroupID), nil, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "get_settings", Annotations: readOnly(),
		Description: "Returns all panel settings, with secrets redacted to has* flags. Use before update_settings.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, _ struct{}) (*sdk.CallToolResult, any, error) {
		return d.call(ctx, "POST", "/setting/all", nil, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "update_settings", Annotations: destructive(),
		Description: "Replaces all panel settings; omitted fields reset. Send the whole object from get_settings. Port or certificate changes need restart_panel.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in settingsUpdateInput) (*sdk.CallToolResult, any, error) {
		return d.call(ctx, "POST", "/setting/update", nil, in.Settings)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "restart_panel", Annotations: destructive(),
		Description: "Restarts the panel after 3 seconds; connections to the panel drop briefly. Use after update_settings changed the port or certificates.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, _ struct{}) (*sdk.CallToolResult, any, error) {
		return d.call(ctx, "POST", "/setting/restartPanel", nil, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "test_notifier", Annotations: additive(),
		Description: "Sends a test message through the saved SMTP, Telegram bot or Discord bot settings.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in notifierTestInput) (*sdk.CallToolResult, any, error) {
		route, ok := notifierTestRoutes[in.Channel]
		if !ok {
			return errorResult("channel must be smtp, telegram or discord"), nil, nil
		}
		return d.call(ctx, "POST", route, nil, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "backup_to_telegram", Annotations: additive(),
		Description: "Sends a database backup to the Telegram bot admins. Needs the Telegram bot enabled; the panel reports no result.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, _ struct{}) (*sdk.CallToolResult, any, error) {
		return d.call(ctx, "POST", "/backuptotgbot", nil, nil)
	})
}
