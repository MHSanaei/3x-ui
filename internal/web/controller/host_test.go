package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func newHostTestDB(t *testing.T) {
	t.Helper()
	// I18nWeb logs a warning when the localizer is absent (as in tests); the
	// logger must be initialised so that warning does not nil-panic.
	xuilogger.InitLogger(logging.ERROR)
	gin.SetMode(gin.TestMode)
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

type hostEnvelope struct {
	Success bool            `json:"success"`
	Msg     string          `json:"msg"`
	Obj     json.RawMessage `json:"obj"`
}

func doHostReq(t *testing.T, engine *gin.Engine, method, path string, body any) hostEnvelope {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
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

// TestHostController_AddListGetDelete exercises the CRUD round-trip and asserts
// the {success,msg,obj} envelope convention through the registered routes.
func TestHostController_AddListGetDelete(t *testing.T) {
	newHostTestDB(t)
	engine := gin.New()
	NewHostController(engine.Group("/panel/api/hosts"))

	ib := &model.Inbound{Tag: "ctl", Enable: true, Port: 5443, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}

	// add
	add := doHostReq(t, engine, http.MethodPost, "/panel/api/hosts/add", map[string]any{
		"inboundId": ib.Id, "remark": "h1", "address": "h1.example.com", "port": 8443,
	})
	if !add.Success {
		t.Fatalf("add not successful: %s", add.Msg)
	}
	var created model.Host
	if err := json.Unmarshal(add.Obj, &created); err != nil {
		t.Fatalf("decode created host: %v", err)
	}
	if created.Id == 0 || created.Remark != "h1" {
		t.Fatalf("created host = %+v", created)
	}

	// list
	list := doHostReq(t, engine, http.MethodGet, "/panel/api/hosts/list", nil)
	var hosts []model.Host
	if err := json.Unmarshal(list.Obj, &hosts); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(hosts) != 1 || hosts[0].Id != created.Id {
		t.Fatalf("list = %+v, want one host id=%d", hosts, created.Id)
	}

	// get
	get := doHostReq(t, engine, http.MethodGet, "/panel/api/hosts/get/"+itoa(created.Id), nil)
	if !get.Success {
		t.Fatalf("get not successful: %s", get.Msg)
	}

	// del
	del := doHostReq(t, engine, http.MethodPost, "/panel/api/hosts/del/"+itoa(created.Id), nil)
	if !del.Success {
		t.Fatalf("del not successful: %s", del.Msg)
	}
	list2 := doHostReq(t, engine, http.MethodGet, "/panel/api/hosts/list", nil)
	var hosts2 []model.Host
	_ = json.Unmarshal(list2.Obj, &hosts2)
	if len(hosts2) != 0 {
		t.Fatalf("after delete, list = %+v, want empty", hosts2)
	}
}

// TestHostController_AuthInherited mirrors production wiring: the hosts group is
// nested under the api group guarded by checkAPIAuth, so an unauthenticated XHR
// to a hosts route is rejected (401) — the auth is inherited, not re-declared.
func TestHostController_AuthInherited(t *testing.T) {
	newHostTestDB(t)
	engine := gin.New()
	store := cookie.NewStore([]byte("host-auth-test-secret"))
	engine.Use(sessions.Sessions("3x-ui", store))

	a := &APIController{}
	api := engine.Group("/panel/api")
	api.Use(a.checkAPIAuth)
	NewHostController(api.Group("/hosts"))

	req := httptest.NewRequest(http.MethodGet, "/panel/api/hosts/list", nil)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated hosts/list = %d, want 401 (auth inherited)", w.Code)
	}
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
