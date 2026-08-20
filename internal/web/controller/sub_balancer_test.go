package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
)

func setupSubBalancerRouter(t *testing.T) *gin.Engine {
	t.Helper()
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewSubBalancerController(router.Group("/panel/api"))
	return router
}

func subBalancerPost(t *testing.T, router *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func responseObj(t *testing.T, body string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("unmarshal response %q: %v", body, err)
	}
	return m
}

// enabled absent on create defaults to true; "false" disables; a non-boolean
// value is rejected so a malformed toggle can't silently flip the row.
func TestSubBalancerController_EnabledParsing(t *testing.T) {
	router := setupSubBalancerRouter(t)
	base := "remark=auto&strategy=random&sortOrder=1&inboundIds=1"

	resp := subBalancerPost(t, router, "/panel/api/sub-balancers", base)
	if !strings.Contains(resp.Body.String(), `"success":true`) {
		t.Fatalf("create no enabled: %s", resp.Body.String())
	}
	bal := responseObj(t, resp.Body.String())["obj"].(map[string]any)
	if bal["enabled"] != true {
		t.Fatalf("absent enabled = %v, want true", bal["enabled"])
	}

	resp = subBalancerPost(t, router, "/panel/api/sub-balancers", base+"&enabled=false")
	bal = responseObj(t, resp.Body.String())["obj"].(map[string]any)
	if bal["enabled"] != false {
		t.Fatalf("enabled=false -> %v, want false", bal["enabled"])
	}

	resp = subBalancerPost(t, router, "/panel/api/sub-balancers", base+"&enabled=bogus")
	if !strings.Contains(resp.Body.String(), `"success":false`) {
		t.Fatalf("enabled=bogus should be rejected: %s", resp.Body.String())
	}
}

// An update omitting enabled preserves the stored value instead of resetting it
// to the create default — a partial PATCH must not clobber the toggle.
func TestSubBalancerController_UpdatePreservesEnabledWhenAbsent(t *testing.T) {
	router := setupSubBalancerRouter(t)
	base := "remark=auto&strategy=random&sortOrder=1&inboundIds=1"

	resp := subBalancerPost(t, router, "/panel/api/sub-balancers", base+"&enabled=false")
	bal := responseObj(t, resp.Body.String())["obj"].(map[string]any)
	id := strconv.Itoa(int(bal["id"].(float64)))
	if bal["enabled"] != false {
		t.Fatalf("setup: enabled = %v, want false", bal["enabled"])
	}

	resp = subBalancerPost(t, router, "/panel/api/sub-balancers/"+id, "remark=renamed&strategy=random&sortOrder=1&inboundIds=1")
	if !strings.Contains(resp.Body.String(), `"success":true`) {
		t.Fatalf("update: %s", resp.Body.String())
	}
	bal = responseObj(t, resp.Body.String())["obj"].(map[string]any)
	if bal["enabled"] != false {
		t.Fatalf("update without enabled = %v, want preserved false", bal["enabled"])
	}
	if bal["remark"] != "renamed" {
		t.Fatalf("remark = %v, want renamed", bal["remark"])
	}

	resp = subBalancerPost(t, router, "/panel/api/sub-balancers/"+id, "remark=renamed&strategy=random&sortOrder=1&inboundIds=1&enabled=true")
	bal = responseObj(t, resp.Body.String())["obj"].(map[string]any)
	if bal["enabled"] != true {
		t.Fatalf("enabled=true -> %v, want true", bal["enabled"])
	}
}
