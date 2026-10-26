package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const testBasePath = "/xui/"

// toolCase drives one tool through MCP and names the API request it must send.
// wantBody, when set, is compared as JSON with the forwarded request body.
type toolCase struct {
	name       string
	tool       string
	args       map[string]any
	wantMethod string
	wantPath   string
	wantQuery  string
	wantBody   string
}

type recordedRequest struct {
	method, path, query, body, auth, contentType string
}

type testPanel struct {
	session  *sdk.ClientSession
	requests []recordedRequest
	reply    func(w http.ResponseWriter, r *http.Request)
}

// newTestPanel serves NewHandler under testBasePath. Inner API requests reach a
// recorder instead of the real router; reply defaults to an entity.Msg success.
func newTestPanel(t *testing.T) *testPanel {
	t.Helper()
	p, err := connectTestPanel(t, "")
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	return p
}

func connectTestPanel(t *testing.T, rawQuery string) (*testPanel, error) {
	t.Helper()
	p := &testPanel{reply: func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"success":true,"msg":"","obj":{"ok":1}}`)
	}}
	router := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		p.requests = append(p.requests, recordedRequest{r.Method, r.URL.Path, r.URL.RawQuery, string(body), r.Header.Get("Authorization"), r.Header.Get("Content-Type")})
		p.reply(w, r)
	})
	mux := http.NewServeMux()
	mux.Handle(testBasePath+apiPrefix+Path, NewHandler(router))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	hc := &http.Client{Transport: bearerTransport{token: "test-token"}}
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	endpoint := srv.URL + testBasePath + apiPrefix + Path
	if rawQuery != "" {
		endpoint += "?" + rawQuery
	}
	session, err := client.Connect(context.Background(), &sdk.StreamableClientTransport{Endpoint: endpoint, HTTPClient: hc}, nil)
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() { _ = session.Close() })
	p.session = session
	return p, nil
}

type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func (p *testPanel) callTool(t *testing.T, name string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := p.session.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String(), res.IsError
}

// documentedRoutes parses endpoints.ts, the registry TestRouteRegistryContract
// pins to the real router, into method -> path patterns.
func documentedRoutes(t *testing.T) map[string][]*regexp.Regexp {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "frontend", "src", "pages", "api-docs", "endpoints.ts"))
	if err != nil {
		t.Fatalf("read endpoints.ts: %v", err)
	}
	entry := regexp.MustCompile(`method:\s*'([A-Z]+)',\s*\n?\s*path:\s*'([^']+)'`)
	param := regexp.MustCompile(`:[A-Za-z]+`)
	routes := map[string][]*regexp.Regexp{}
	for _, m := range entry.FindAllStringSubmatch(string(src), -1) {
		pattern := "^" + param.ReplaceAllString(regexp.QuoteMeta(m[2]), `[^/]+`) + "$"
		routes[m[1]] = append(routes[m[1]], regexp.MustCompile(pattern))
	}
	if len(routes["GET"]) < 10 {
		t.Fatalf("parsed only %d GET routes from endpoints.ts; the entry regexp no longer matches", len(routes["GET"]))
	}
	return routes
}

func allToolCases() []toolCase {
	var cases []toolCase
	for _, group := range [][]toolCase{coreToolCases, inboundToolCases, clientToolCases, serverToolCases, panelToolCases} {
		cases = append(cases, group...)
	}
	return cases
}

var coreToolCases = []toolCase{
	{
		name: "call_api GET", tool: "call_api", args: map[string]any{"method": "get", "path": "/clients/get/alice", "query": map[string]any{"x": "1"}},
		wantMethod: "GET", wantPath: "/panel/api/clients/get/alice", wantQuery: "x=1",
	},
	{
		name: "call_api POST body", tool: "call_api", args: map[string]any{"method": "POST", "path": "/inbounds/del/3", "body": map[string]any{"a": 1}},
		wantMethod: "POST", wantPath: "/panel/api/inbounds/del/3", wantBody: `{"a":1}`,
	},
	{
		name: "call_api POST form", tool: "call_api", args: map[string]any{"method": "POST", "path": "/server/logs/10", "form": map[string]any{"level": "info", "syslog": "false"}},
		wantMethod: "POST", wantPath: "/panel/api/server/logs/10", wantBody: "level=info&syslog=false",
	},
}

// TestToolsHitDocumentedRoutes catches a tool aimed at a route or method the
// panel does not serve, a wrong base path, and a tool shipped without a case.
func TestToolsHitDocumentedRoutes(t *testing.T) {
	routes := documentedRoutes(t)
	p := newTestPanel(t)

	covered := map[string]bool{"get_openapi_spec": true}
	for _, tc := range allToolCases() {
		covered[tc.tool] = true
		t.Run(tc.name, func(t *testing.T) {
			p.requests = nil
			text, isErr := p.callTool(t, tc.tool, tc.args)
			if isErr {
				t.Fatalf("tool error: %s", text)
			}
			if len(p.requests) != 1 {
				t.Fatalf("got %d API requests, want 1", len(p.requests))
			}
			got := p.requests[0]
			apiPath := strings.TrimPrefix(got.path, strings.TrimSuffix(testBasePath, "/"))
			if got.method != tc.wantMethod || apiPath != tc.wantPath || got.query != tc.wantQuery {
				t.Fatalf("sent %s %s?%s, want %s %s?%s", got.method, apiPath, got.query, tc.wantMethod, tc.wantPath, tc.wantQuery)
			}
			if got.auth != "Bearer test-token" {
				t.Fatalf("Authorization forwarded as %q", got.auth)
			}
			if tc.wantBody != "" && !bodyEqual(t, got.body, tc.wantBody) {
				t.Fatalf("body %s, want %s", got.body, tc.wantBody)
			}
			// The encoding must match the handler's binding (PostForm vs ShouldBindJSON), not just the bytes.
			if want := wantContentType(tc.wantBody); tc.wantBody != "" && got.contentType != want {
				t.Fatalf("Content-Type %q, want %q", got.contentType, want)
			}
			documented := false
			for _, re := range routes[got.method] {
				documented = documented || re.MatchString(apiPath)
			}
			if !documented {
				t.Fatalf("%s %s is not a route in endpoints.ts", got.method, apiPath)
			}
		})
	}

	list, err := p.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	for _, tool := range list.Tools {
		if !covered[tool.Name] {
			t.Errorf("tool %s has no toolCase", tool.Name)
		}
		if tool.Annotations == nil {
			t.Errorf("tool %s has no annotations", tool.Name)
		}
	}
}

func wantContentType(wantBody string) string {
	if strings.HasPrefix(wantBody, "{") || strings.HasPrefix(wantBody, "[") {
		return "application/json"
	}
	return "application/x-www-form-urlencoded"
}

// bodyEqual compares JSON bodies structurally; a want that is not JSON is a
// url.Values.Encode() form body, which is already canonical (sorted keys).
func bodyEqual(t *testing.T, a, b string) bool {
	t.Helper()
	if !strings.HasPrefix(b, "{") && !strings.HasPrefix(b, "[") {
		return a == b
	}
	var x, y any
	if err := json.Unmarshal([]byte(a), &x); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(b), &y); err != nil {
		t.Fatalf("bad wantBody %s: %v", b, err)
	}
	xs, _ := json.Marshal(x)
	ys, _ := json.Marshal(y)
	return string(xs) == string(ys)
}

// TestToolsetsFilterTools catches a filter that leaks other areas' tools, drops
// the always-on core tools, or silently accepts a misspelt toolset.
func TestToolsetsFilterTools(t *testing.T) {
	p, err := connectTestPanel(t, "toolsets=clients")
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	list, err := p.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	clientTools := map[string]bool{"call_api": true, "get_openapi_spec": true}
	for _, tc := range clientToolCases {
		clientTools[tc.tool] = true
	}
	got := map[string]bool{}
	for _, tool := range list.Tools {
		got[tool.Name] = true
		if !clientTools[tool.Name] {
			t.Errorf("toolsets=clients exposed %s", tool.Name)
		}
	}
	if len(got) != len(clientTools) {
		t.Errorf("toolsets=clients exposed %d tools, want %d", len(got), len(clientTools))
	}

	_, err = connectTestPanel(t, "toolsets=clients,bogus")
	if err == nil || !strings.Contains(err.Error(), "Bad Request") {
		t.Fatalf("unknown toolset: err = %v, want a 400 refusal", err)
	}
	if _, err := parseToolsets("clients,bogus"); err == nil || err.Error() != `unknown toolset "bogus"; valid: inbounds, clients, server, panel` {
		t.Fatalf("parseToolsets error = %v", err)
	}
}

// TestDispatcherRefusesMCPEndpoint catches a tool, present or future, whose
// own path checks let a request re-enter /mcp and recurse.
func TestDispatcherRefusesMCPEndpoint(t *testing.T) {
	reached := false
	router := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true })
	outer := httptest.NewRequest(http.MethodPost, testBasePath+apiPrefix+Path, nil)
	d := newDispatcher(router, outer)
	for _, p := range []string{"/mcp", "/%6dcp", "/./mcp"} {
		raw, status := d.do(context.Background(), http.MethodPost, p, nil, nil)
		if status != http.StatusBadRequest || string(raw) != "refusing to call the MCP endpoint from a tool" || reached {
			t.Fatalf("%s: got (%d, %q, reached=%v), want refusal before routing", p, status, raw, reached)
		}
	}
}

func TestCallAPIRejectsPathsOutsideAPI(t *testing.T) {
	p := newTestPanel(t)
	for _, path := range []string{"/mcp", "/mcp/", "//mcp", "/inbounds/../mcp", "/../login", "inbounds/list", "/inbounds/list?x=1", "/%6dcp", "/inbounds/%2e%2e/mcp", "/%256dcp", "/inbounds/list%3Fx=1"} {
		t.Run(path, func(t *testing.T) {
			p.requests = nil
			text, isErr := p.callTool(t, "call_api", map[string]any{"method": "GET", "path": path})
			if !isErr || text != errBadAPIPath.Error() {
				t.Fatalf("got (%q, isErr=%v), want rejection", text, isErr)
			}
			if len(p.requests) != 0 {
				t.Fatalf("rejected path still reached the API: %+v", p.requests)
			}
		})
	}
}

func TestToolResultMapsPanelReplies(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		want    string
		wantErr bool
	}{
		{"obj relayed", 200, `{"success":true,"msg":"","obj":[1,2]}`, "[1,2]", false},
		{"msg when no obj", 200, `{"success":true,"msg":"Inbound deleted","obj":null}`, "Inbound deleted", false},
		{"failure msg", 200, `{"success":false,"msg":"inbound not found","obj":null}`, "inbound not found", true},
		{"forbidden scope", 403, `{"success":false,"msg":"this API token is not permitted to access this endpoint"}`, "HTTP 403: this API token is not permitted to access this endpoint", true},
		{"bare status", 404, ``, "HTTP 404: Not Found", true},
		{"non-Msg JSON", 200, `{"openapi":"3.1.0"}`, `{"openapi":"3.1.0"}`, false},
		{"binary body", 200, "SQLite format 3\x00\xff\xfe", "binary response (18 bytes) cannot be returned over MCP; fetch it with a regular API client", true},
		{"empty body", 200, ``, "ok", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := toolResult(tc.status, []byte(tc.body))
			got := res.Content[0].(*sdk.TextContent).Text
			if got != tc.want || res.IsError != tc.wantErr {
				t.Fatalf("got (%q, isErr=%v), want (%q, %v)", got, res.IsError, tc.want, tc.wantErr)
			}
		})
	}
}

func TestDescribeOpenAPI(t *testing.T) {
	spec := `{"paths":{
		"/panel/api/clients/list":{"get":{"summary":"List clients","responses":{"200":{"$ref":"#/components/schemas/ClientList"}}}},
		"/panel/api/inbounds/list":{"get":{"summary":"List inbounds"}},
		"/login":{"post":{"summary":"Log in"}}},
		"components":{"schemas":{
			"ClientList":{"items":{"$ref":"#/components/schemas/Client"}},
			"Client":{"type":"object"},
			"Unused":{"type":"string"}}}}`

	index, err := describeOpenAPI([]byte(spec), "/panel/api", "")
	if err != nil {
		t.Fatal(err)
	}
	if want := "GET /clients/list - List clients\nGET /inbounds/list - List inbounds\n"; index != want {
		t.Fatalf("index:\n%s\nwant:\n%s", index, want)
	}

	filtered, err := describeOpenAPI([]byte(spec), "/panel/api", "/clients")
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Paths   map[string]any `json:"paths"`
		Schemas map[string]any `json:"schemas"`
	}
	if err := json.Unmarshal([]byte(filtered), &out); err != nil {
		t.Fatal(err)
	}
	if _, ok := out.Paths["/clients/list"]; !ok || len(out.Paths) != 1 {
		t.Fatalf("paths = %v, want only /clients/list", out.Paths)
	}
	if _, ok := out.Schemas["Client"]; !ok || len(out.Schemas) != 2 {
		t.Fatalf("schemas = %v, want ClientList and its transitive Client only", out.Schemas)
	}

	if _, err := describeOpenAPI([]byte(spec), "/panel/api", "/nope"); err == nil || err.Error() != "no routes under /panel/api/nope" {
		t.Fatalf("unknown prefix error = %v", err)
	}
}
