package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	xraygeodata "github.com/xtls/xray-core/common/geodata"
	"google.golang.org/protobuf/proto"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/logview"
	"github.com/mhsanaei/3x-ui/v3/internal/xray/geodata"
)

const (
	logsTestLine = "2026/03/04 05:06:07 from 203.0.113.9:1 accepted tcp:a.example:443 [in-1 >> direct] email: alice"
	logsTestErr  = "2026/03/04 05:06:08 [Error] [7] app/dns: failed"
)

func newLogsTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	engine, a := newAPIAuthTestEngine(t)
	logDir := t.TempDir()
	t.Setenv("XUI_LOG_FOLDER", logDir)
	t.Setenv("XUI_BIN_FOLDER", t.TempDir())
	if err := os.WriteFile(filepath.Join(logDir, "a.log"), []byte(logsTestLine+"\n"+logsTestErr+"\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(logDir), "outside.log"), []byte("outside\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	api := engine.Group("/panel/api")
	api.Use(a.checkAPIAuth, a.enforceTokenScope)
	NewLogController(api.Group("/logs"))
	return engine
}

func seedLogsToken(t *testing.T, scope string) string {
	t.Helper()
	plaintext := "logs-" + scope
	if err := database.GetDB().Create(&model.ApiToken{
		Name: plaintext, Token: crypto.HashTokenSHA256(plaintext), Enabled: true, Scope: scope,
	}).Error; err != nil {
		t.Fatalf("seed token: %v", err)
	}
	return plaintext
}

func logsGet(engine *gin.Engine, target, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

// Logs name clients and their destinations; restricted tokens exist to keep that data out of reach.
func TestLogRoutesRefuseRestrictedTokens(t *testing.T) {
	engine := newLogsTestEngine(t)
	targets := []string{
		"/panel/api/logs/files",
		"/panel/api/logs/read?file=a.log",
		"/panel/api/logs/stats?file=a.log",
	}
	for _, scope := range []string{model.ApiScopeMonitor, model.ApiScopeNodeSync, model.ApiScopeNodeAdmin} {
		token := seedLogsToken(t, scope)
		for _, target := range targets {
			w := logsGet(engine, target, token)
			if w.Code != http.StatusForbidden || strings.Contains(w.Body.String(), "alice") {
				t.Errorf("%s token on %s: status %d body %s, want 403", scope, target, w.Code, w.Body.String())
			}
		}
	}
	for _, target := range targets {
		if w := logsGet(engine, target, ""); w.Code != http.StatusNotFound {
			t.Errorf("anonymous on %s: status %d, want 404", target, w.Code)
		}
	}
}

func TestLogRoutesAreReadOnly(t *testing.T) {
	engine := newLogsTestEngine(t)
	found := 0
	for _, route := range engine.Routes() {
		if strings.HasPrefix(route.Path, "/panel/api/logs") {
			found++
			if route.Method != http.MethodGet {
				t.Errorf("%s %s is not a read", route.Method, route.Path)
			}
		}
	}
	if found != 3 {
		t.Errorf("found %d log routes, want 3", found)
	}
}

func TestLogReadBindsQueryToFilter(t *testing.T) {
	engine := newLogsTestEngine(t)
	token := seedLogsToken(t, model.ApiScopeAdmin)

	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{"newest first", "file=a.log", []string{logsTestErr, logsTestLine}},
		{"limit", "file=a.log&limit=1", []string{logsTestErr}},
		{"levels", "file=a.log&levels=info", []string{logsTestLine}},
		{"ip", "file=a.log&ip=203.0", []string{logsTestLine}},
		{"user", "file=a.log&user=ALICE", []string{logsTestLine}},
		{"host", "file=a.log&host=a.exam", []string{logsTestLine}},
		{"inbound", "file=a.log&inbound=in-1", []string{logsTestLine}},
		{"protocol", "file=a.log&protocol=tcp", []string{logsTestLine}},
		{"status", "file=a.log&status=app/dns", []string{logsTestErr}},
		{"free text", "file=a.log&query=failed", []string{logsTestErr}},
		{"until before everything", "file=a.log&until=1000", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := logsGet(engine, "/panel/api/logs/read?"+tc.query, token)
			var body struct {
				Success bool            `json:"success"`
				Obj     logview.LogPage `json:"obj"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode %s: %v", w.Body.String(), err)
			}
			if !body.Success {
				t.Fatalf("not successful: %s", w.Body.String())
			}
			var got []string
			for _, line := range body.Obj.Lines {
				got = append(got, line.Raw)
			}
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLogStatsBindsQueryToFilter(t *testing.T) {
	engine := newLogsTestEngine(t)
	token := seedLogsToken(t, model.ApiScopeAdmin)
	w := logsGet(engine, "/panel/api/logs/stats?file=a.log&levels=error", token)
	var body struct {
		Obj logview.LogStats `json:"obj"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %s: %v", w.Body.String(), err)
	}
	if body.Obj.MatchedLines != 1 || body.Obj.Errors != 1 || body.Obj.ScannedLines != 2 {
		t.Errorf("matched=%d errors=%d scanned=%d, want 1/1/2", body.Obj.MatchedLines, body.Obj.Errors, body.Obj.ScannedLines)
	}
}

func TestLogReadBindsCountryFilter(t *testing.T) {
	engine := newLogsTestEngine(t)
	token := seedLogsToken(t, model.ApiScopeAdmin)
	data, err := proto.Marshal(&xraygeodata.GeoIPList{Entry: []*xraygeodata.GeoIP{{
		Code: "DE",
		Cidr: []*xraygeodata.CIDR{{Ip: netip.MustParseAddr("203.0.113.0").AsSlice(), Prefix: 24}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(os.Getenv("XUI_BIN_FOLDER"), geodata.CountryFileName), data, 0o640); err != nil {
		t.Fatal(err)
	}
	for query, want := range map[string]int{"country=de": 1, "country=FR": 0} {
		w := logsGet(engine, "/panel/api/logs/read?file=a.log&"+query, token)
		var body struct {
			Obj logview.LogPage `json:"obj"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode %s: %v", w.Body.String(), err)
		}
		if len(body.Obj.Lines) != want {
			t.Errorf("%s returned %d lines, want %d", query, len(body.Obj.Lines), want)
		}
	}
}

func TestLogRoutesReportBadInputWithoutLeakingPaths(t *testing.T) {
	engine := newLogsTestEngine(t)
	token := seedLogsToken(t, model.ApiScopeAdmin)
	cases := []struct {
		target string
		want   string
	}{
		{"/panel/api/logs/read?file=../outside.log", `{"success":false,"msg":" (invalid log file name)","obj":null}`},
		{"/panel/api/logs/read?file=..%2Foutside.log", `{"success":false,"msg":" (invalid log file name)","obj":null}`},
		{"/panel/api/logs/stats?file=x-ui.db", `{"success":false,"msg":" (invalid log file name)","obj":null}`},
		{"/panel/api/logs/read?file=gone.log", `{"success":false,"msg":" (log file not found)","obj":null}`},
		{"/panel/api/logs/read?file=a.log&limit=abc", `{"success":false,"msg":" (invalid log filter)","obj":null}`},
		{"/panel/api/logs/stats?file=a.log&since=abc", `{"success":false,"msg":" (invalid log filter)","obj":null}`},
		{"/panel/api/logs/read?file=a.log&levels=loud", `{"success":false,"msg":" (invalid log filter: unknown level)","obj":null}`},
	}
	for _, tc := range cases {
		if w := logsGet(engine, tc.target, token); w.Body.String() != tc.want {
			t.Errorf("%s = %s, want %s", tc.target, w.Body.String(), tc.want)
		}
	}
}
