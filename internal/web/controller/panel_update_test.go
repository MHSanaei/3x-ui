package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/config"

	"github.com/gin-gonic/gin"
)

func newPanelUpdateTestEngine() *gin.Engine {
	a := &ServerController{}
	engine := gin.New()
	engine.GET("/panel/api/server/getUpdateStatus", a.getUpdateStatus)
	engine.POST("/panel/api/server/updatePanel", a.offlineOnly)
	engine.POST("/panel/api/server/uploadUpdate", a.updatePanel)
	return engine
}

func doPanelUpdateReq(t *testing.T, engine *gin.Engine, method, path string) hostEnvelope {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("%s %s: status %d, body=%s", method, path, w.Code, w.Body.String())
	}
	var env hostEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("%s %s: decode envelope: %v body=%s", method, path, err, w.Body.String())
	}
	return env
}

// TestGetUpdateStatus_NoStatusFileYet exercises the read-only status endpoint
// with no prior update having run: it must report "pending" (not an error),
// since a missing status file is an expected, ordinary state, not a failure.
func TestGetUpdateStatus_NoStatusFileYet(t *testing.T) {
	newHostTestDB(t)
	engine := newPanelUpdateTestEngine()

	env := doPanelUpdateReq(t, engine, http.MethodGet, "/panel/api/server/getUpdateStatus")
	if !env.Success {
		t.Fatalf("getUpdateStatus should always report success=true (it's a best-effort read): msg=%s", env.Msg)
	}
	var status struct {
		RunID string `json:"runId"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(env.Obj, &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.State != "pending" {
		t.Fatalf("State = %q, want %q", status.State, "pending")
	}
}

// TestGetUpdateStatus_RunIdIsAlwaysAString is the regression test for the
// precision bug found in review: RunID is a 19-digit UnixNano timestamp, so
// it must round-trip over the wire as a JSON string, never a bare number -- a
// bare number would silently lose precision in JavaScript past
// Number.MAX_SAFE_INTEGER, breaking every future runId comparison on the
// frontend. Decoding into a Go string field below only succeeds if the wire
// value is actually a JSON string; a bare number there would fail to decode,
// so this test doubles as the wire-format check.
func TestGetUpdateStatus_RunIdIsAlwaysAString(t *testing.T) {
	newHostTestDB(t)
	engine := newPanelUpdateTestEngine()

	statusPath := config.GetUpdateStatusFilePath()
	body := `{"runId":"1735689600123456789","state":"success","exitCode":0,"finishedAt":1735689612}`
	if err := os.WriteFile(statusPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	env := doPanelUpdateReq(t, engine, http.MethodGet, "/panel/api/server/getUpdateStatus")
	var status struct {
		RunID string `json:"runId"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(env.Obj, &status); err != nil {
		t.Fatalf("decode status (would fail here if runId were a bare JSON number instead of a string): %v, body=%s", err, env.Obj)
	}
	if status.RunID != "1735689600123456789" {
		t.Fatalf("RunID = %q, want %q", status.RunID, "1735689600123456789")
	}
	if status.State != "success" {
		t.Fatalf("State = %q, want %q", status.State, "success")
	}
}

func TestOfflineUpdateRequiresLocalZIP(t *testing.T) {
	engine := newPanelUpdateTestEngine()
	for _, tt := range []struct {
		endpoint, contentType string
		size                  int64
		want                  int
	}{
		{"updatePanel", "application/x-www-form-urlencoded", 0, http.StatusGone},
		{"uploadUpdate", "application/x-www-form-urlencoded", 0, http.StatusUnsupportedMediaType},
		{"uploadUpdate", "application/zip", 257 << 20, http.StatusRequestEntityTooLarge},
	} {
		req := httptest.NewRequest(http.MethodPost, "/panel/api/server/"+tt.endpoint, strings.NewReader("version=latest"))
		req.Header.Set("Content-Type", tt.contentType)
		req.ContentLength = tt.size
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != tt.want {
			t.Errorf("%s: got %d, want %d: %s", tt.endpoint, w.Code, tt.want, w.Body)
		}
	}
}
