package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"
	xraygeodata "github.com/xtls/xray-core/common/geodata"
	"google.golang.org/protobuf/proto"

	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray/geodata"
)

func newGeodataEngine(t *testing.T) *gin.Engine {
	t.Helper()
	xuilogger.InitLogger(logging.ERROR)
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", dir)
	writeGeositeDB(t, dir)
	writeGeoipDB(t, dir)

	engine := gin.New()
	NewXraySettingController(engine.Group("/panel/api"))
	return engine
}

func writeGeositeDB(t *testing.T, dir string) {
	t.Helper()
	data, err := proto.Marshal(&xraygeodata.GeoSiteList{Entry: []*xraygeodata.GeoSite{
		{Code: "google", Domain: []*xraygeodata.Domain{
			{Type: xraygeodata.Domain_Domain, Value: "google.com"},
			{Type: xraygeodata.Domain_Full, Value: "ads.google.com", Attribute: []*xraygeodata.Domain_Attribute{
				{Key: "ads", TypedValue: &xraygeodata.Domain_Attribute_BoolValue{BoolValue: true}},
			}},
		}},
		{Code: "cn", Domain: []*xraygeodata.Domain{{Type: xraygeodata.Domain_Domain, Value: "baidu.com"}}},
	}})
	if err != nil {
		t.Fatalf("marshal geosite: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "geosite.dat"), data, 0o644); err != nil {
		t.Fatalf("write geosite.dat: %v", err)
	}
}

func writeGeoipDB(t *testing.T, dir string) {
	t.Helper()
	prefix := netip.MustParsePrefix("10.0.0.0/8")
	data, err := proto.Marshal(&xraygeodata.GeoIPList{Entry: []*xraygeodata.GeoIP{
		{Code: "private", Cidr: []*xraygeodata.CIDR{{Ip: prefix.Addr().AsSlice(), Prefix: uint32(prefix.Bits())}}},
	}})
	if err != nil {
		t.Fatalf("marshal geoip: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "geoip.dat"), data, 0o644); err != nil {
		t.Fatalf("write geoip.dat: %v", err)
	}
}

type geodataEnvelope struct {
	Success bool            `json:"success"`
	Msg     string          `json:"msg"`
	Obj     json.RawMessage `json:"obj"`
}

func doGeodataGet(t *testing.T, engine *gin.Engine, path string) geodataEnvelope {
	t.Helper()
	return doGeodataReq(t, engine, httptest.NewRequest(http.MethodGet, path, nil))
}

func doGeodataPost(t *testing.T, engine *gin.Engine, path string, form url.Values) geodataEnvelope {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return doGeodataReq(t, engine, req)
}

func doGeodataReq(t *testing.T, engine *gin.Engine, req *http.Request) geodataEnvelope {
	t.Helper()
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("%s %s: status %d, body=%s", req.Method, req.URL, w.Code, w.Body.String())
	}
	var env geodataEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v body=%s", err, w.Body.String())
	}
	return env
}

func TestGeodataFiles(t *testing.T) {
	engine := newGeodataEngine(t)

	env := doGeodataGet(t, engine, "/panel/api/xray/geodata/files")
	if !env.Success {
		t.Fatalf("files not successful: %s", env.Msg)
	}
	var files []geodata.GeoFile
	if err := json.Unmarshal(env.Obj, &files); err != nil {
		t.Fatalf("decode files: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %+v, want 2 entries", files)
	}
	byName := make(map[string]geodata.GeoFile, len(files))
	for _, file := range files {
		byName[file.Name] = file
	}
	if got := byName["geosite.dat"]; got.Kind != geodata.KindSite || got.Categories != 2 {
		t.Errorf("geosite.dat = %+v, want kind site with 2 categories", got)
	}
	if got := byName["geoip.dat"]; got.Kind != geodata.KindIP || got.Categories != 1 {
		t.Errorf("geoip.dat = %+v, want kind ip with 1 category", got)
	}
}

func TestGeodataCategoriesAndEntries(t *testing.T) {
	engine := newGeodataEngine(t)

	env := doGeodataGet(t, engine, "/panel/api/xray/geodata/categories?file=geosite.dat&q=goo&limit=10")
	var categories geodata.GeoCategoryPage
	if err := json.Unmarshal(env.Obj, &categories); err != nil {
		t.Fatalf("decode categories: %v", err)
	}
	if categories.Total != 1 || categories.Items[0].Code != "google" {
		t.Fatalf("categories = %+v, want only google", categories)
	}

	env = doGeodataGet(t, engine, "/panel/api/xray/geodata/entries?file=geosite.dat&code=google&limit=1&offset=1")
	var entries geodata.GeoEntryPage
	if err := json.Unmarshal(env.Obj, &entries); err != nil {
		t.Fatalf("decode entries: %v", err)
	}
	if entries.Total != 2 {
		t.Errorf("entries total = %d, want 2", entries.Total)
	}
	if len(entries.Items) != 1 || entries.Items[0].Value != "ads.google.com" || entries.Items[0].Kind != "full" {
		t.Errorf("entries items = %+v, want the second entry ads.google.com", entries.Items)
	}
}

func TestGeodataRejectsBadRequests(t *testing.T) {
	engine := newGeodataEngine(t)

	tests := []struct {
		name string
		path string
	}{
		{name: "missing code", path: "/panel/api/xray/geodata/entries?file=geosite.dat"},
		{name: "unknown category", path: "/panel/api/xray/geodata/entries?file=geosite.dat&code=nope"},
		{name: "path traversal", path: "/panel/api/xray/geodata/categories?file=../../etc/passwd.dat"},
		{name: "non dat file", path: "/panel/api/xray/geodata/categories?file=x-ui.db"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if env := doGeodataGet(t, engine, tt.path); env.Success {
				t.Errorf("request succeeded, want failure: %s", env.Obj)
			}
		})
	}
}

func TestGeodataValidate(t *testing.T) {
	engine := newGeodataEngine(t)

	tests := []struct {
		name       string
		kind       string
		tokens     string
		wantTokens []string
		wantReason string
	}{
		{name: "known categories pass", kind: "domain", tokens: "geosite:google,geosite:cn,google.com"},
		{name: "attribute filter passes", kind: "domain", tokens: "geosite:google@ads"},
		{
			name:       "attribute the category does not carry",
			kind:       "domain",
			tokens:     "geosite:google@typo",
			wantTokens: []string{"geosite:google@typo"},
			wantReason: "attributeMissing",
		},
		{
			name:       "empty attribute is a syntax error",
			kind:       "domain",
			tokens:     "geosite:google@",
			wantTokens: []string{"geosite:google@"},
			wantReason: "syntax",
		},
		{
			name:       "missing category",
			kind:       "domain",
			tokens:     "geosite:google,geosite:blabla",
			wantTokens: []string{"geosite:blabla"},
			wantReason: "categoryMissing",
		},
		{
			name:       "missing database",
			kind:       "domain",
			tokens:     "ext:absent.dat:corp",
			wantTokens: []string{"ext:absent.dat:corp"},
			wantReason: "fileMissing",
		},
		{name: "plain cidr passes", kind: "ip", tokens: "10.0.0.0/8,geoip:private"},
		{
			name:       "missing ip category",
			kind:       "ip",
			tokens:     "geoip:nowhere",
			wantTokens: []string{"geoip:nowhere"},
			wantReason: "categoryMissing",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := doGeodataPost(t, engine, "/panel/api/xray/geodata/validate", url.Values{
				"kind":   {tt.kind},
				"tokens": {tt.tokens},
			})
			if !env.Success {
				t.Fatalf("validate not successful: %s", env.Msg)
			}
			var issues []service.GeodataTokenIssue
			if err := json.Unmarshal(env.Obj, &issues); err != nil {
				t.Fatalf("decode issues: %v", err)
			}
			if len(issues) != len(tt.wantTokens) {
				t.Fatalf("issues = %+v, want %d", issues, len(tt.wantTokens))
			}
			for i, wantToken := range tt.wantTokens {
				if issues[i].Token != wantToken {
					t.Errorf("issue %d token = %q, want %q", i, issues[i].Token, wantToken)
				}
				if issues[i].Reason != tt.wantReason {
					t.Errorf("issue %d reason = %q, want %q", i, issues[i].Reason, tt.wantReason)
				}
			}
		})
	}
}
