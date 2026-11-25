package mcp

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type listClientsInput struct {
	Page       int    `json:"page,omitempty" jsonschema:"page number, e.g. 2; default 1"`
	PageSize   int    `json:"page_size,omitempty" jsonschema:"rows per page, e.g. 50; default 25"`
	Search     string `json:"search,omitempty" jsonschema:"substring of email, subId or comment, e.g. alice"`
	Filter     string `json:"filter,omitempty" jsonschema:"comma-separated statuses from active, deactive, depleted, expiring, online"`
	Protocol   string `json:"protocol,omitempty" jsonschema:"comma-separated inbound protocols, e.g. vless,trojan"`
	InboundIDs string `json:"inbound_ids,omitempty" jsonschema:"comma-separated inbound IDs, e.g. 3,5"`
	Group      string `json:"group,omitempty" jsonschema:"group name, e.g. vip"`
	Sort       string `json:"sort,omitempty" jsonschema:"one of enable, email, inboundIds, traffic, remaining, expiryTime, createdAt, updatedAt, lastOnline"`
	Order      string `json:"order,omitempty" jsonschema:"asc or desc"`
}

type getClientInput struct {
	Email string `json:"email,omitempty" jsonschema:"client email, e.g. alice"`
	SubID string `json:"sub_id,omitempty" jsonschema:"subscription ID; returns every client sharing it"`
	TgID  int64  `json:"tg_id,omitempty" jsonschema:"Telegram user ID, e.g. 123456789; returns every linked client"`
}

type clientEmailsInput struct {
	Emails []string `json:"emails" jsonschema:"client emails, e.g. [\"alice\"]"`
}

type saveClientInput struct {
	Client     map[string]any `json:"client" jsonschema:"client object: email, totalGB (bytes, 0 = unlimited), expiryTime (Unix ms, e.g. 1767225600000; 0 = never), limitIp, limitHwid, tgId, subId, comment, group, enable, flow; secrets are generated when omitted"`
	Email      string         `json:"email,omitempty" jsonschema:"current email of the client to update; omit to create"`
	InboundIDs []int          `json:"inbound_ids,omitempty" jsonschema:"create: inbounds to attach to (required); update: limit the edit to these inbounds"`
}

type clientCreatePayload struct {
	Client     map[string]any `json:"client"`
	InboundIds []int          `json:"inboundIds"`
}

type newClientInput struct {
	Client     map[string]any `json:"client" jsonschema:"client object, same fields as save_client"`
	InboundIDs []int          `json:"inbound_ids" jsonschema:"inbound IDs, e.g. [3,5]"`
}

type bulkCreateClientsInput struct {
	Clients []newClientInput `json:"clients" jsonschema:"clients to create"`
}

type deleteClientsInput struct {
	Emails      []string `json:"emails,omitempty" jsonschema:"client emails to delete"`
	Depleted    bool     `json:"depleted,omitempty" jsonschema:"delete every client that is out of traffic or expired"`
	Orphans     bool     `json:"orphans,omitempty" jsonschema:"delete every client attached to no inbound"`
	KeepTraffic bool     `json:"keep_traffic,omitempty" jsonschema:"with emails: keep their traffic records"`
}

type setClientInboundsInput struct {
	Emails     []string `json:"emails" jsonschema:"client emails"`
	InboundIDs []int    `json:"inbound_ids" jsonschema:"inbound IDs, e.g. [3,5]"`
	Action     string   `json:"action" jsonschema:"attach or detach"`
}

type setClientsEnabledInput struct {
	Emails  []string `json:"emails" jsonschema:"client emails"`
	Enabled bool     `json:"enabled" jsonschema:"true to enable, false to disable"`
}

type resetClientTrafficInput struct {
	Emails []string `json:"emails,omitempty" jsonschema:"client emails"`
	All    bool     `json:"all,omitempty" jsonschema:"reset every client on the panel"`
}

type adjustClientsInput struct {
	Emails    []string `json:"emails" jsonschema:"client emails"`
	AddDays   int      `json:"add_days,omitempty" jsonschema:"days added to expiry, e.g. 30; negative shortens"`
	AddBytes  int64    `json:"add_bytes,omitempty" jsonschema:"bytes added to quota, e.g. 10737418240 for 10 GB; negative lowers"`
	Flow      string   `json:"flow,omitempty" jsonschema:"VLESS flow to set, e.g. xtls-rprx-vision"`
	LimitHwid *int     `json:"limit_hwid,omitempty" jsonschema:"device limit to set, e.g. 2; 0 = unlimited"`
	AdTag     string   `json:"ad_tag,omitempty" jsonschema:"mtproto ad tag to set"`
}

type clientLinksInput struct {
	Email string `json:"email,omitempty" jsonschema:"client email: returns its share links"`
	SubID string `json:"sub_id,omitempty" jsonschema:"subscription ID: returns its subscription URLs"`
}

type clientActivityInput struct {
	Kind  string `json:"kind" jsonschema:"online, last_online, ips or hwids"`
	Email string `json:"email,omitempty" jsonschema:"client email; required for ips and hwids"`
}

type setClientsGroupInput struct {
	Emails []string `json:"emails" jsonschema:"client emails"`
	Group  string   `json:"group,omitempty" jsonschema:"group name, e.g. vip; empty removes the clients from their group"`
}

func joinClientInboundIDs(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	return strings.Join(parts, ",")
}

const errNoClientEmails = "emails must list at least one client"

// countClientSelectors reports how many of the mutually exclusive selectors were given.
func countClientSelectors(flags ...bool) int {
	n := 0
	for _, f := range flags {
		if f {
			n++
		}
	}
	return n
}

func addClientTools(s *sdk.Server, d *dispatcher) {
	sdk.AddTool(s, &sdk.Tool{
		Name: "list_clients",
		Description: "Returns one page of clients plus panel-wide totals, filtered by search, status, protocol, inbound or group. " +
			"Use it to find emails; call get_client for one client's full record.",
		Annotations: readOnly(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in listClientsInput) (*sdk.CallToolResult, any, error) {
		q := url.Values{}
		set := func(k, v string) {
			if v != "" {
				q.Set(k, v)
			}
		}
		if in.Page > 0 {
			q.Set("page", strconv.Itoa(in.Page))
		}
		if in.PageSize > 0 {
			q.Set("pageSize", strconv.Itoa(in.PageSize))
		}
		set("search", in.Search)
		set("filter", in.Filter)
		set("protocol", in.Protocol)
		set("inbound", in.InboundIDs)
		set("group", in.Group)
		set("sort", in.Sort)
		set("order", in.Order)
		return d.call(ctx, "GET", "/clients/list/paged", q, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "get_client",
		Description: "Returns a client's full record: settings, attached inbound IDs and used traffic. " +
			"Pass exactly one of email, sub_id or tg_id (the last two return a list). Its client field is what save_client takes.",
		Annotations: readOnly(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in getClientInput) (*sdk.CallToolResult, any, error) {
		if countClientSelectors(in.Email != "", in.SubID != "", in.TgID != 0) != 1 {
			return errorResult("pass exactly one of email, sub_id or tg_id"), nil, nil
		}
		p := "/clients/get/" + url.PathEscape(in.Email)
		if in.SubID != "" {
			p = "/clients/get/subId/" + url.PathEscape(in.SubID)
		} else if in.TgID != 0 {
			p = "/clients/get/tgId/" + strconv.FormatInt(in.TgID, 10)
		}
		return d.call(ctx, "GET", p, nil, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "save_client",
		Description: "Creates a client on inbound_ids, or with email replaces that client's settings. " +
			"An update overwrites the row, so send the full client object from get_client. To extend expiry or quota use adjust_clients.",
		Annotations: destructive(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in saveClientInput) (*sdk.CallToolResult, any, error) {
		if in.Email == "" {
			if len(in.InboundIDs) == 0 {
				return errorResult("inbound_ids is required when creating a client"), nil, nil
			}
			return d.call(ctx, "POST", "/clients/add", nil, clientCreatePayload{in.Client, in.InboundIDs})
		}
		q := url.Values{}
		if len(in.InboundIDs) > 0 {
			q.Set("inboundIds", joinClientInboundIDs(in.InboundIDs))
		}
		return d.call(ctx, "POST", "/clients/update/"+url.PathEscape(in.Email), q, in.Client)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "bulk_create_clients",
		Description: "Creates many clients in one call, each with its own inbounds, and returns per-client results. Use save_client for one client.",
		Annotations: additive(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in bulkCreateClientsInput) (*sdk.CallToolResult, any, error) {
		payloads := make([]clientCreatePayload, len(in.Clients))
		for i, c := range in.Clients {
			payloads[i] = clientCreatePayload{c.Client, c.InboundIDs}
		}
		return d.call(ctx, "POST", "/clients/bulkCreate", nil, payloads)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "delete_clients",
		Description: "Deletes clients from every inbound: the listed emails, all depleted (out of traffic or expired) clients, or all orphans (on no inbound). " +
			"Pick exactly one mode.",
		Annotations: destructive(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in deleteClientsInput) (*sdk.CallToolResult, any, error) {
		if countClientSelectors(len(in.Emails) > 0, in.Depleted, in.Orphans) != 1 {
			return errorResult("pass exactly one of emails, depleted or orphans"), nil, nil
		}
		switch {
		case in.Depleted:
			return d.call(ctx, "POST", "/clients/delDepleted", nil, nil)
		case in.Orphans:
			return d.call(ctx, "POST", "/clients/delOrphans", nil, nil)
		case len(in.Emails) == 1:
			q := url.Values{}
			if in.KeepTraffic {
				q.Set("keepTraffic", "1")
			}
			return d.call(ctx, "POST", "/clients/del/"+url.PathEscape(in.Emails[0]), q, nil)
		}
		return d.call(ctx, "POST", "/clients/bulkDel", nil, map[string]any{"emails": in.Emails, "keepTraffic": in.KeepTraffic})
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "set_client_inbounds",
		Description: "Attaches clients to more inbounds or detaches them, keeping the client records and secrets. " +
			"Use list_inbounds for inbound IDs and delete_clients to remove clients entirely.",
		Annotations: destructive(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in setClientInboundsInput) (*sdk.CallToolResult, any, error) {
		if in.Action != "attach" && in.Action != "detach" {
			return errorResult("action must be attach or detach"), nil, nil
		}
		switch len(in.Emails) {
		case 0:
			return errorResult(errNoClientEmails), nil, nil
		case 1:
			return d.call(ctx, "POST", "/clients/"+url.PathEscape(in.Emails[0])+"/"+in.Action, nil, map[string]any{"inboundIds": in.InboundIDs})
		}
		p := "/clients/bulkAttach"
		if in.Action == "detach" {
			p = "/clients/bulkDetach"
		}
		return d.call(ctx, "POST", p, nil, map[string]any{"emails": in.Emails, "inboundIds": in.InboundIDs})
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "set_clients_enabled",
		Description: "Enables or disables clients on every inbound they are attached to.",
		Annotations: destructive(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in setClientsEnabledInput) (*sdk.CallToolResult, any, error) {
		if len(in.Emails) == 0 {
			return errorResult(errNoClientEmails), nil, nil
		}
		p := "/clients/bulkDisable"
		if in.Enabled {
			p = "/clients/bulkEnable"
		}
		return d.call(ctx, "POST", p, nil, clientEmailsInput{in.Emails})
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "reset_client_traffic",
		Description: "Resets used upload/download counters to zero for the listed clients, or for every client with all. Pass exactly one.",
		Annotations: destructive(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in resetClientTrafficInput) (*sdk.CallToolResult, any, error) {
		if countClientSelectors(len(in.Emails) > 0, in.All) != 1 {
			return errorResult("pass exactly one of emails or all"), nil, nil
		}
		switch {
		case in.All:
			return d.call(ctx, "POST", "/clients/resetAllTraffics", nil, nil)
		case len(in.Emails) == 1:
			return d.call(ctx, "POST", "/clients/resetTraffic/"+url.PathEscape(in.Emails[0]), nil, nil)
		}
		return d.call(ctx, "POST", "/clients/bulkResetTraffic", nil, clientEmailsInput{in.Emails})
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "adjust_clients",
		Description: "Extends or shortens clients' expiry and quota, optionally setting flow, device limit or ad tag, and keeps every other setting. " +
			"Use it for renewals instead of save_client.",
		Annotations: destructive(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in adjustClientsInput) (*sdk.CallToolResult, any, error) {
		if len(in.Emails) == 0 {
			return errorResult(errNoClientEmails), nil, nil
		}
		return d.call(ctx, "POST", "/clients/bulkAdjust", nil, map[string]any{
			"emails": in.Emails, "addDays": in.AddDays, "addBytes": in.AddBytes,
			"flow": in.Flow, "limitHwid": in.LimitHwid, "adTag": in.AdTag,
		})
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "get_client_links",
		Description: "Returns a client's share links (vless://, trojan://...) by email, or the subscription URLs for a sub_id. " +
			"Pass exactly one; get_client returns a client's subId.",
		Annotations: readOnly(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in clientLinksInput) (*sdk.CallToolResult, any, error) {
		if countClientSelectors(in.Email != "", in.SubID != "") != 1 {
			return errorResult("pass exactly one of email or sub_id"), nil, nil
		}
		if in.SubID != "" {
			return d.call(ctx, "GET", "/clients/subLinks/"+url.PathEscape(in.SubID), nil, nil)
		}
		return d.call(ctx, "GET", "/clients/links/"+url.PathEscape(in.Email), nil, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "get_client_activity",
		Description: "Returns connection activity: emails online now (online), last-seen Unix ms per email (last_online), " +
			"or one client's recent source IPs (ips) or registered devices (hwids).",
		Annotations: readOnly(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in clientActivityInput) (*sdk.CallToolResult, any, error) {
		switch in.Kind {
		case "online":
			return d.call(ctx, "POST", "/clients/onlines", nil, nil)
		case "last_online":
			return d.call(ctx, "POST", "/clients/lastOnline", nil, nil)
		case "ips", "hwids":
			if in.Email == "" {
				return errorResult("email is required for kind " + in.Kind), nil, nil
			}
			return d.call(ctx, "POST", "/clients/"+in.Kind+"/"+url.PathEscape(in.Email), nil, nil)
		}
		return errorResult("kind must be online, last_online, ips or hwids"), nil, nil
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "set_clients_group",
		Description: "Moves clients into a group, creating it if needed, or removes them from their group when group is empty.",
		Annotations: destructive(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in setClientsGroupInput) (*sdk.CallToolResult, any, error) {
		if len(in.Emails) == 0 {
			return errorResult(errNoClientEmails), nil, nil
		}
		if strings.TrimSpace(in.Group) == "" {
			return d.call(ctx, "POST", "/clients/groups/bulkRemove", nil, clientEmailsInput{in.Emails})
		}
		return d.call(ctx, "POST", "/clients/groups/bulkAdd", nil, in)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "list_client_groups",
		Description: "Returns client groups with their member counts. Filter list_clients by group to see the members.",
		Annotations: readOnly(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, _ struct{}) (*sdk.CallToolResult, any, error) {
		return d.call(ctx, "GET", "/clients/groups", nil, nil)
	})
}
