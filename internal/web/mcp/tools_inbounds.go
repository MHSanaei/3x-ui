package mcp

import (
	"context"
	"strconv"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type listInboundsInput struct {
	View string `json:"view,omitempty" jsonschema:"brief (default): id, remark, tag, protocol, port; slim: full rows, clients cut to email/enable/comment; full: everything incl. client secrets"`
}

type inboundIDInput struct {
	InboundID int `json:"inbound_id" jsonschema:"inbound ID from list_inbounds, e.g. 3"`
}

type saveInboundInput struct {
	InboundID int            `json:"inbound_id,omitempty" jsonschema:"inbound ID to replace, e.g. 3; omit to create a new inbound"`
	Inbound   map[string]any `json:"inbound" jsonschema:"inbound object as get_inbound returns it: protocol, port, remark, settings, streamSettings, sniffing (nested objects)"`
}

type deleteInboundsInput struct {
	InboundIDs []int `json:"inbound_ids" jsonschema:"inbound IDs, e.g. [3, 4]"`
}

type setInboundEnabledInput struct {
	InboundID int  `json:"inbound_id" jsonschema:"inbound ID from list_inbounds, e.g. 3"`
	Enabled   bool `json:"enabled" jsonschema:"true to enable, false to disable"`
}

type resetInboundTrafficInput struct {
	InboundID int  `json:"inbound_id,omitempty" jsonschema:"inbound ID to reset, e.g. 3; omit when all is true"`
	All       bool `json:"all,omitempty" jsonschema:"true to reset every inbound instead of one"`
}

func addInboundTools(s *sdk.Server, d *dispatcher) {
	sdk.AddTool(s, &sdk.Tool{
		Name: "list_inbounds",
		Description: "Lists the panel's inbounds. Start here to find inbound IDs; the default brief view stays small on large panels. " +
			"Use view slim for configs, full only for client secrets, or get_inbound for one inbound.",
		Annotations: readOnly(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in listInboundsInput) (*sdk.CallToolResult, any, error) {
		switch in.View {
		case "", "brief":
			return d.call(ctx, "GET", "/inbounds/options", nil, nil)
		case "slim":
			return d.call(ctx, "GET", "/inbounds/list/slim", nil, nil)
		case "full":
			return d.call(ctx, "GET", "/inbounds/list", nil, nil)
		}
		return errorResult("view must be brief, slim or full"), nil, nil
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "get_inbound",
		Description: "Returns one inbound with its full settings, stream settings, clients and traffic. Call it before save_inbound to get the shape to edit.",
		Annotations: readOnly(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in inboundIDInput) (*sdk.CallToolResult, any, error) {
		return d.call(ctx, "GET", "/inbounds/get/"+strconv.Itoa(in.InboundID), nil, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "save_inbound",
		Description: "Creates an inbound, or replaces one when inbound_id is given, and returns it. To edit, send the whole get_inbound object back: omitted fields reset. " +
			"Clients and enable are kept on edit; use the client tools and set_inbound_enabled.",
		Annotations: destructive(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in saveInboundInput) (*sdk.CallToolResult, any, error) {
		if in.InboundID == 0 {
			return d.call(ctx, "POST", "/inbounds/add", nil, in.Inbound)
		}
		return d.call(ctx, "POST", "/inbounds/update/"+strconv.Itoa(in.InboundID), nil, in.Inbound)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "delete_inbounds",
		Description: "Deletes inbounds and their clients' traffic rows; cannot be undone. With several IDs, failures are reported per ID and the rest still go.",
		Annotations: destructive(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in deleteInboundsInput) (*sdk.CallToolResult, any, error) {
		switch len(in.InboundIDs) {
		case 0:
			return errorResult("inbound_ids must not be empty"), nil, nil
		case 1:
			return d.call(ctx, "POST", "/inbounds/del/"+strconv.Itoa(in.InboundIDs[0]), nil, nil)
		}
		return d.call(ctx, "POST", "/inbounds/bulkDel", nil, map[string]any{"ids": in.InboundIDs})
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "set_inbound_enabled",
		Description: "Enables or disables an inbound without touching its configuration.",
		Annotations: destructive(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in setInboundEnabledInput) (*sdk.CallToolResult, any, error) {
		return d.call(ctx, "POST", "/inbounds/setEnable/"+strconv.Itoa(in.InboundID), nil, map[string]any{"enable": in.Enabled})
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "reset_inbound_traffic",
		Description: "Zeroes the upload and download counters of one inbound, or of every inbound with all. Per-client counters are not touched.",
		Annotations: destructive(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in resetInboundTrafficInput) (*sdk.CallToolResult, any, error) {
		switch {
		case in.All && in.InboundID == 0:
			return d.call(ctx, "POST", "/inbounds/resetAllTraffics", nil, nil)
		case !in.All && in.InboundID != 0:
			return d.call(ctx, "POST", "/inbounds/"+strconv.Itoa(in.InboundID)+"/resetTraffic", nil, nil)
		}
		return errorResult("give exactly one of inbound_id or all"), nil, nil
	})
}
