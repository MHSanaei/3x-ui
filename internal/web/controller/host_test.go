package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/op/go-logging"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func newHostTestDB(t *testing.T) {
	t.Helper()
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

func TestHostController_AddListGetDelete(t *testing.T) {
	newHostTestDB(t)
	engine := gin.New()
	NewHostController(engine.Group("/panel/api/hosts"))

	ib := &model.Inbound{Tag: "ctl", Enable: true, Port: 5443, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}

	add := doHostReq(t, engine, http.MethodPost, "/panel/api/hosts/add", map[string]any{
		"inboundIds": []int{ib.Id}, "remark": "h1", "hosts": []string{"h1.example.com"}, "port": 8443,
	})
	if !add.Success {
		t.Fatalf("add not successful: %s", add.Msg)
	}
	var created []*model.Host
	if err := json.Unmarshal(add.Obj, &created); err != nil {
		t.Fatalf("decode created hosts: %v", err)
	}
	if len(created) != 1 || created[0].GroupId == "" || created[0].Remark != "h1" {
		t.Fatalf("created hosts = %+v", created)
	}
	groupId := created[0].GroupId

	list := doHostReq(t, engine, http.MethodGet, "/panel/api/hosts/list", nil)
	var groups []entity.HostGroup
	if err := json.Unmarshal(list.Obj, &groups); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(groups) != 1 || groups[0].GroupId != groupId {
		t.Fatalf("list = %+v, want one group groupId=%s", groups, groupId)
	}

	get := doHostReq(t, engine, http.MethodGet, "/panel/api/hosts/get/"+groupId, nil)
	if !get.Success {
		t.Fatalf("get not successful: %s", get.Msg)
	}

	update := doHostReq(t, engine, http.MethodPost, "/panel/api/hosts/update/"+groupId, map[string]any{
		"inboundIds": []int{ib.Id}, "remark": "h1-updated", "hosts": []string{"h1.example.com"}, "port": 8443,
	})
	if !update.Success {
		t.Fatalf("update not successful: %s", update.Msg)
	}
	get2 := doHostReq(t, engine, http.MethodGet, "/panel/api/hosts/get/"+groupId, nil)
	var group2 entity.HostGroup
	_ = json.Unmarshal(get2.Obj, &group2)
	if group2.Remark != "h1-updated" {
		t.Fatalf("update did not change remark: %s", group2.Remark)
	}

	setEn := doHostReq(t, engine, http.MethodPost, "/panel/api/hosts/bulk/setEnable", map[string]any{
		"ids": []string{groupId}, "enable": false,
	})
	if !setEn.Success {
		t.Fatalf("bulk/setEnable not successful: %s", setEn.Msg)
	}
	get3 := doHostReq(t, engine, http.MethodGet, "/panel/api/hosts/get/"+groupId, nil)
	var group3 entity.HostGroup
	_ = json.Unmarshal(get3.Obj, &group3)
	if !group3.IsDisabled {
		t.Fatalf("bulk/setEnable did not disable host group")
	}

	add2 := doHostReq(t, engine, http.MethodPost, "/panel/api/hosts/bulk/add", map[string]any{
		"inboundIds": []int{ib.Id}, "remark": "h2", "hosts": []string{"h2.example.com"}, "port": 8443,
	})
	var created2 []*model.Host
	_ = json.Unmarshal(add2.Obj, &created2)
	groupId2 := created2[0].GroupId

	bulkDel := doHostReq(t, engine, http.MethodPost, "/panel/api/hosts/bulk/del", map[string]any{
		"ids": []string{groupId, groupId2},
	})
	if !bulkDel.Success {
		t.Fatalf("bulk/del not successful: %s", bulkDel.Msg)
	}

	list2 := doHostReq(t, engine, http.MethodGet, "/panel/api/hosts/list", nil)
	var groups2 []entity.HostGroup
	_ = json.Unmarshal(list2.Obj, &groups2)
	if len(groups2) != 0 {
		t.Fatalf("after delete, list = %+v, want empty", groups2)
	}
}

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
