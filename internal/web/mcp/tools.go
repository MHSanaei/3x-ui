package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// toolsets let clients with small tool limits (Cursor caps near 40) load only
// the areas they need via ?toolsets=clients,inbounds; call_api is always on.
var toolsets = map[string]func(*sdk.Server, *dispatcher){
	"inbounds": addInboundTools,
	"clients":  addClientTools,
	"server":   addServerTools,
	"panel":    addPanelTools,
}

func parseToolsets(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		names := make([]string, 0, len(toolsets))
		for name := range toolsets {
			names = append(names, name)
		}
		sort.Strings(names)
		return names, nil
	}
	var names []string
	seen := map[string]bool{}
	for _, name := range strings.Split(raw, ",") {
		name = strings.TrimSpace(name)
		if _, ok := toolsets[name]; !ok {
			return nil, fmt.Errorf("unknown toolset %q; valid: inbounds, clients, server, panel", name)
		}
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names, nil
}

func registerTools(s *sdk.Server, d *dispatcher, names []string) {
	for _, name := range names {
		toolsets[name](s, d)
	}
	addCallAPITool(s, d)
}

func readOnly() *sdk.ToolAnnotations { return &sdk.ToolAnnotations{ReadOnlyHint: true} }

func additive() *sdk.ToolAnnotations {
	f := false
	return &sdk.ToolAnnotations{DestructiveHint: &f}
}

func destructive() *sdk.ToolAnnotations {
	t := true
	return &sdk.ToolAnnotations{DestructiveHint: &t}
}

type callAPIInput struct {
	Method string            `json:"method" jsonschema:"HTTP method: GET or POST (a few routes also take PUT or DELETE)"`
	Path   string            `json:"path" jsonschema:"route under /panel/api, e.g. /inbounds/list or /clients/get/alice; get_openapi_spec lists them"`
	Query  map[string]string `json:"query,omitempty" jsonschema:"optional query-string parameters"`
	Body   any               `json:"body,omitempty" jsonschema:"optional JSON request body"`
	Form   map[string]string `json:"form,omitempty" jsonschema:"optional form fields, for routes that read form values instead of JSON; ignored when body is set"`
}

var errBadAPIPath = errors.New("path must be an absolute route under /panel/api, without .. segments, and not the MCP endpoint itself")

// cleanAPIPath rejects traversal out of /panel/api and recursion into the MCP
// endpoint; checks run on the decoded path because url.Parse decodes %6dcp to mcp.
func cleanAPIPath(p string) (string, error) {
	decoded, err := url.PathUnescape(p)
	if err != nil || !strings.HasPrefix(decoded, "/") || strings.Contains(decoded, "..") || strings.ContainsAny(decoded, "?#%") {
		return "", errBadAPIPath
	}
	if cleaned := path.Clean(decoded); cleaned == Path || strings.HasPrefix(cleaned, Path+"/") {
		return "", errBadAPIPath
	}
	return path.Clean(p), nil
}

func addCallAPITool(s *sdk.Server, d *dispatcher) {
	sdk.AddTool(s, &sdk.Tool{
		Name: "call_api",
		Description: "Call any 3x-ui panel API route directly. Use it only when no dedicated tool fits; " +
			"call get_openapi_spec first to find the route, its method and body shape.",
		Annotations: destructive(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in callAPIInput) (*sdk.CallToolResult, any, error) {
		method := strings.ToUpper(in.Method)
		switch method {
		case "GET", "POST", "PUT", "DELETE":
		default:
			return errorResult("method must be GET, POST, PUT or DELETE"), nil, nil
		}
		p, err := cleanAPIPath(in.Path)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		query := url.Values{}
		for k, v := range in.Query {
			query.Set(k, v)
		}
		body := in.Body
		if body == nil && len(in.Form) > 0 {
			form := url.Values{}
			for k, v := range in.Form {
				form.Set(k, v)
			}
			body = form
		}
		return d.call(ctx, method, p, query, body)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "get_openapi_spec",
		Description: "Describe panel API routes for call_api. Without a prefix it lists every route as 'METHOD path - summary'; " +
			"with a prefix such as /clients it returns those routes' full OpenAPI operations and the schemas they reference.",
		Annotations: readOnly(),
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in openAPIInput) (*sdk.CallToolResult, any, error) {
		raw, status := d.do(ctx, "GET", "/openapi.json", nil, nil)
		if status != http.StatusOK {
			return toolResult(status, raw), nil, nil
		}
		text, err := describeOpenAPI(raw, "/"+apiPrefix, in.Prefix)
		if err != nil {
			return errorResult(err.Error()), nil, nil
		}
		return textResult(text), nil, nil
	})
}

type openAPIInput struct {
	Prefix string `json:"prefix,omitempty" jsonschema:"route prefix under /panel/api, e.g. /clients; omit to list all routes"`
}

type openAPIDoc struct {
	Paths      map[string]map[string]json.RawMessage `json:"paths"`
	Components struct {
		Schemas map[string]json.RawMessage `json:"schemas"`
	} `json:"components"`
}

var schemaRefPattern = regexp.MustCompile(`"#/components/schemas/([^"]+)"`)

// describeOpenAPI trims the ~500 KB spec to what one agent turn can hold: an
// index of routes, or one prefix's operations plus their transitive schemas.
func describeOpenAPI(raw []byte, apiPath, prefix string) (string, error) {
	var doc openAPIDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", err
	}
	routes := make([]string, 0, len(doc.Paths))
	for p := range doc.Paths {
		if rel, ok := strings.CutPrefix(p, apiPath); ok && strings.HasPrefix(rel, prefix) {
			routes = append(routes, p)
		}
	}
	sort.Strings(routes)
	if len(routes) == 0 {
		return "", fmt.Errorf("no routes under %s%s", apiPath, prefix)
	}

	if prefix == "" {
		var b strings.Builder
		for _, p := range routes {
			methods := make([]string, 0, len(doc.Paths[p]))
			for m := range doc.Paths[p] {
				methods = append(methods, m)
			}
			sort.Strings(methods)
			for _, m := range methods {
				var op struct {
					Summary string `json:"summary"`
				}
				_ = json.Unmarshal(doc.Paths[p][m], &op)
				fmt.Fprintf(&b, "%s %s - %s\n", strings.ToUpper(m), strings.TrimPrefix(p, apiPath), op.Summary)
			}
		}
		return b.String(), nil
	}

	out := struct {
		Paths   map[string]map[string]json.RawMessage `json:"paths"`
		Schemas map[string]json.RawMessage            `json:"schemas,omitempty"`
	}{Paths: map[string]map[string]json.RawMessage{}, Schemas: map[string]json.RawMessage{}}
	var pending []string
	for _, p := range routes {
		out.Paths[strings.TrimPrefix(p, apiPath)] = doc.Paths[p]
		for _, op := range doc.Paths[p] {
			for _, m := range schemaRefPattern.FindAllSubmatch(op, -1) {
				pending = append(pending, string(m[1]))
			}
		}
	}
	for len(pending) > 0 {
		name := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		schema, ok := doc.Components.Schemas[name]
		if _, seen := out.Schemas[name]; seen || !ok {
			continue
		}
		out.Schemas[name] = schema
		for _, m := range schemaRefPattern.FindAllSubmatch(schema, -1) {
			pending = append(pending, string(m[1]))
		}
	}
	text, err := json.Marshal(out)
	return string(text), err
}
