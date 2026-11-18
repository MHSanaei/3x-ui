package ha

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func snapshotSource(s Snapshot) func() Snapshot { return func() Snapshot { return s } }

func serve(h http.Handler, method, target string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestEndpointLeader(t *testing.T) {
	master := Snapshot{State: State{Role: Master, Master: "de"}, MasterPanelURL: "https://de:2053"}
	replica := Snapshot{State: State{Role: Replica, Master: "nl"}, MasterPanelURL: "https://nl:2053"}

	rec := serve(EndpointHandler("de", "s", Self{}, snapshotSource(master)), "GET", "/ha/leader", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("master: status %d", rec.Code)
	}

	rec = serve(EndpointHandler("pl", "s", Self{}, snapshotSource(replica)), "GET", "/ha/leader", nil)
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusServiceUnavailable || body["master_url"] != "https://nl:2053" || rec.Header().Get(MasterHeader) != "https://nl:2053" {
		t.Fatalf("replica: want 503 with the master hint, got %d %v", rec.Code, body)
	}
}

func TestEndpointPeerRequiresSecret(t *testing.T) {
	h := EndpointHandler("de", "s3cret", Self{PanelURL: "https://de:2053", SubURL: "https://de:2096"},
		snapshotSource(Snapshot{State: State{Role: Replica}}))

	if rec := serve(h, "GET", PeerPath, map[string]string{"Authorization": "Bearer nope"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong secret: status %d", rec.Code)
	}
	rec := serve(h, "GET", PeerPath, map[string]string{"Authorization": "Bearer s3cret"})
	var ping Ping
	if err := json.Unmarshal(rec.Body.Bytes(), &ping); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	if ping != (Ping{Code: "de", Role: Replica, PanelURL: "https://de:2053", SubURL: "https://de:2096"}) {
		t.Fatalf("got %+v", ping)
	}
}

func TestStandbyHandler(t *testing.T) {
	known := Snapshot{State: State{Role: Replica, Master: "nl"}, MasterPanelURL: "https://nl:2053", MasterSubURL: "https://nl:2096"}
	none := Snapshot{State: State{Role: Fenced, Reason: "no quorum of voting machines"}}

	cases := []struct {
		name       string
		snap       Snapshot
		pick       func(Snapshot) string
		method     string
		target     string
		wantCode   int
		wantTarget string
	}{
		{"api write gets 409", known, PanelTarget, "POST", "/base/panel/api/inbounds/add", http.StatusConflict, "https://nl:2053"},
		{"api read gets 409, not a redirect the bot would follow blindly", known, PanelTarget, "GET", "/base/panel/api/inbounds/list", http.StatusConflict, "https://nl:2053"},
		{"login post gets 409", known, PanelTarget, "POST", "/base/login", http.StatusConflict, "https://nl:2053"},
		{"browser page redirects keeping the path", known, PanelTarget, "GET", "/base/panel/inbounds?x=1", http.StatusTemporaryRedirect, "https://nl:2053/base/panel/inbounds?x=1"},
		{"subscription redirects to the master's sub server", known, SubTarget, "GET", "/sub/abc", http.StatusTemporaryRedirect, "https://nl:2096/sub/abc"},
		{"no known master", none, PanelTarget, "GET", "/base/", http.StatusServiceUnavailable, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := serve(StandbyHandler(snapshotSource(c.snap), c.pick), c.method, c.target, nil)
			if rec.Code != c.wantCode {
				t.Fatalf("status %d, want %d (%s)", rec.Code, c.wantCode, rec.Body)
			}
			switch c.wantCode {
			case http.StatusTemporaryRedirect:
				if got := rec.Header().Get("Location"); got != c.wantTarget {
					t.Fatalf("Location %q, want %q", got, c.wantTarget)
				}
			case http.StatusConflict:
				var body map[string]any
				_ = json.Unmarshal(rec.Body.Bytes(), &body)
				if body["error"] != "not_master" || body["master_url"] != c.wantTarget || rec.Header().Get(MasterHeader) != c.wantTarget {
					t.Fatalf("want not_master with master_url %q, got %v", c.wantTarget, body)
				}
			}
		})
	}
}

func TestGinGuardStopsAFormerMaster(t *testing.T) {
	gin.SetMode(gin.TestMode)
	snap := Snapshot{State: State{Role: Master}}
	engine := gin.New()
	engine.Use(GinGuard(func() Snapshot { return snap }, PanelTarget))
	reached := 0
	engine.POST("/panel/api/inbounds/add", func(c *gin.Context) { reached++; c.Status(http.StatusOK) })

	if rec := serve(engine, "POST", "/panel/api/inbounds/add", nil); rec.Code != http.StatusOK || reached != 1 {
		t.Fatalf("master must serve: %d reached=%d", rec.Code, reached)
	}
	snap = Snapshot{State: State{Role: Fenced, Master: "nl"}, MasterPanelURL: "https://nl:2053"}
	rec := serve(engine, "POST", "/panel/api/inbounds/add", nil)
	if rec.Code != http.StatusConflict || reached != 1 {
		t.Fatalf("a fenced node must stop writing at once: %d reached=%d", rec.Code, reached)
	}
}
