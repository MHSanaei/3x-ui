package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/robfig/cron/v3"

	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/global"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"
)

func newMCPTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	previous := global.GetWebServer()
	s := NewServer(nil)
	s.cron = cron.New(cron.WithLocation(time.Local), cron.WithSeconds())
	global.SetWebServer(s)
	t.Cleanup(func() {
		s.cancel()
		global.SetWebServer(previous)
	})
	engine, err := s.initRouter()
	if err != nil {
		t.Fatalf("init router: %v", err)
	}
	srv := httptest.NewServer(engine)
	t.Cleanup(srv.Close)
	return srv
}

func createToken(t *testing.T, name, scope string) string {
	t.Helper()
	view, err := (&panel.ApiTokenService{}).Create(name, scope, 0)
	if err != nil {
		t.Fatalf("create %s token: %v", scope, err)
	}
	return view.Token
}

type bearer struct{ token string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

type csrfHeader struct{ token string }

func (c csrfHeader) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	if c.token != "" {
		r.Header.Set(session.CSRFHeaderName, c.token)
	}
	return http.DefaultTransport.RoundTrip(r)
}

func fetchCSRFToken(t *testing.T, hc *http.Client, base string) string {
	t.Helper()
	resp, err := hc.Get(base + "/csrf-token")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		Obj string `json:"obj"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.Obj == "" {
		t.Fatalf("csrf token: %v (%q)", err, body.Obj)
	}
	return body.Obj
}

func connectMCP(t *testing.T, endpoint, token string) (*sdk.ClientSession, error) {
	t.Helper()
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
	return client.Connect(context.Background(), &sdk.StreamableClientTransport{
		Endpoint: endpoint, HTTPClient: &http.Client{Transport: bearer{token}},
	}, nil)
}

// TestMCPEndpointUsesPanelAuth catches the MCP route escaping checkAPIAuth or
// token scopes, and tool calls that drop the caller's credentials on re-entry.
func TestMCPEndpointUsesPanelAuth(t *testing.T) {
	srv := newMCPTestServer(t)
	endpoint := srv.URL + "/panel/api/mcp"

	t.Run("anonymous request is hidden", func(t *testing.T) {
		resp, err := http.Post(endpoint, "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d, want 404", resp.StatusCode)
		}
	})

	t.Run("monitor token is refused", func(t *testing.T) {
		token := createToken(t, "monitor", model.ApiScopeMonitor)
		if _, err := connectMCP(t, endpoint, token); err == nil || !strings.HasSuffix(err.Error(), http.StatusText(http.StatusForbidden)) {
			t.Fatalf("connect err = %v, want a 403 refusal", err)
		}
	})

	t.Run("browser session needs CSRF and carries it into tool calls", func(t *testing.T) {
		if err := (&panel.UserService{}).UpdateFirstUser("mcpuser", "mcp-pass-123"); err != nil {
			t.Fatal(err)
		}
		jar, _ := cookiejar.New(nil)
		hc := &http.Client{Jar: jar}
		login := url.Values{"username": {"mcpuser"}, "password": {"mcp-pass-123"}}
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/login", strings.NewReader(login.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set(session.CSRFHeaderName, fetchCSRFToken(t, hc, srv.URL))
		resp, err := hc.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		csrf := fetchCSRFToken(t, hc, srv.URL)

		connect := func(token string) (*sdk.ClientSession, error) {
			client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "0"}, nil)
			return client.Connect(context.Background(), &sdk.StreamableClientTransport{
				Endpoint: endpoint, HTTPClient: &http.Client{Jar: jar, Transport: csrfHeader{token}},
			}, nil)
		}
		if _, err := connect(""); err == nil || !strings.HasSuffix(err.Error(), http.StatusText(http.StatusForbidden)) {
			t.Fatalf("session without CSRF: err = %v, want a 403 refusal", err)
		}
		mcpSession, err := connect(csrf)
		if err != nil {
			t.Fatalf("connect with CSRF: %v", err)
		}
		defer func() { _ = mcpSession.Close() }()
		// An inner POST passes CSRFMiddleware only if the cookie and token were forwarded.
		res, err := mcpSession.CallTool(context.Background(), &sdk.CallToolParams{
			Name: "call_api", Arguments: map[string]any{"method": "POST", "path": "/inbounds/resetAllTraffics"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if text := res.Content[0].(*sdk.TextContent).Text; res.IsError {
			t.Fatalf("inner POST failed: %s", text)
		}
	})

	t.Run("admin token reaches real handlers", func(t *testing.T) {
		session, err := connectMCP(t, endpoint, createToken(t, "admin", model.ApiScopeAdmin))
		if err != nil {
			t.Fatalf("connect: %v", err)
		}
		defer func() { _ = session.Close() }()
		res, err := session.CallTool(context.Background(), &sdk.CallToolParams{
			Name: "call_api", Arguments: map[string]any{"method": "GET", "path": "/inbounds/list"},
		})
		if err != nil {
			t.Fatal(err)
		}
		text := res.Content[0].(*sdk.TextContent).Text
		if res.IsError || text != "[]" {
			t.Fatalf("got (%q, isErr=%v), want the empty inbound list", text, res.IsError)
		}
	})
}
