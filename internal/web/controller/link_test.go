package controller

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestLinkControllerLibraryLifecycle drives the library endpoints over HTTP,
// from add and assign through what the client receives to disable and delete.
func TestLinkControllerLibraryLifecycle(t *testing.T) {
	newHostTestDB(t)
	engine := gin.New()
	NewLinkController(engine.Group("/panel/api/links"))

	db := database.GetDB()
	inbound := &model.Inbound{Tag: "links-ctl", Enable: true, Port: 5455, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	client := &model.ClientRecord{Email: "links@ctl", SubID: "links-ctl-sub", UUID: "aaaaaaaa-1111-2222-3333-444444444444", Enable: true, Group: "links-ctl-group"}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	group := &model.ClientGroup{Name: "links-ctl-group"}
	if err := db.Create(group).Error; err != nil {
		t.Fatalf("seed group: %v", err)
	}

	add := doHostReq(t, engine, http.MethodPost, "/panel/api/links/add", map[string]any{
		"kind": model.ExternalLinkKindLink, "value": "vless://uuid@library.example:443", "remark": "library",
	})
	if !add.Success {
		t.Fatalf("add not successful: %s", add.Msg)
	}
	var created model.ExternalLink
	if err := json.Unmarshal(add.Obj, &created); err != nil {
		t.Fatalf("decode created link: %v", err)
	}
	if created.Id == 0 || created.Remark != "library" {
		t.Fatalf("created link = %+v", created)
	}

	duplicate := doHostReq(t, engine, http.MethodPost, "/panel/api/links/add", map[string]any{
		"kind": model.ExternalLinkKindLink, "value": "vless://uuid@library.example:443", "remark": "again",
	})
	if duplicate.Success {
		t.Fatal("saving the same (kind, value) twice must be rejected")
	}

	assign := doHostReq(t, engine, http.MethodPost, "/panel/api/links/assign/"+strconv.Itoa(created.Id), map[string]any{
		"emails": []string{client.Email}, "group": group.Name, "newClients": true,
	})
	if !assign.Success {
		t.Fatalf("assign not successful: %s", assign.Msg)
	}
	var assigned struct {
		Affected int `json:"affected"`
	}
	_ = json.Unmarshal(assign.Obj, &assigned)
	if assigned.Affected != 3 {
		t.Fatalf("assign affected = %d, want 3 targets", assigned.Affected)
	}

	targets := doHostReq(t, engine, http.MethodGet, "/panel/api/links/targets/"+strconv.Itoa(created.Id), nil)
	if !targets.Success {
		t.Fatalf("targets not successful: %s", targets.Msg)
	}
	var targetRows []struct {
		TargetType string `json:"targetType"`
		Name       string `json:"name"`
	}
	if err := json.Unmarshal(targets.Obj, &targetRows); err != nil {
		t.Fatalf("decode targets: %v", err)
	}
	if len(targetRows) != 3 {
		t.Fatalf("targets = %+v, want three rows", targetRows)
	}

	views := doHostReq(t, engine, http.MethodGet, "/panel/api/links/client/"+strconv.Itoa(client.Id), nil)
	var clientViews []struct {
		Remark string `json:"remark"`
		Scope  string `json:"scope"`
		Own    bool   `json:"own"`
	}
	if err := json.Unmarshal(views.Obj, &clientViews); err != nil {
		t.Fatalf("decode client links: %v", err)
	}
	if len(clientViews) != 1 || clientViews[0].Scope != model.ExternalLinkTargetClient || !clientViews[0].Own {
		t.Fatalf("client links = %+v, want the direct assignment to win", clientViews)
	}

	disable := doHostReq(t, engine, http.MethodPost, "/panel/api/links/enable/"+strconv.Itoa(created.Id), map[string]any{"enable": false})
	if !disable.Success {
		t.Fatalf("enable not successful: %s", disable.Msg)
	}
	views = doHostReq(t, engine, http.MethodGet, "/panel/api/links/client/"+strconv.Itoa(client.Id), nil)
	_ = json.Unmarshal(views.Obj, &clientViews)
	if len(clientViews) != 0 {
		t.Fatalf("a disabled library entry still reaches the client: %+v", clientViews)
	}

	unassign := doHostReq(t, engine, http.MethodPost, "/panel/api/links/unassign/"+strconv.Itoa(created.Id), map[string]any{
		"emails": []string{client.Email},
	})
	if !unassign.Success {
		t.Fatalf("unassign not successful: %s", unassign.Msg)
	}

	removed := doHostReq(t, engine, http.MethodPost, "/panel/api/links/del/"+strconv.Itoa(created.Id), nil)
	if !removed.Success {
		t.Fatalf("del not successful: %s", removed.Msg)
	}
	list := doHostReq(t, engine, http.MethodGet, "/panel/api/links/list", nil)
	var rows []model.ExternalLink
	if err := json.Unmarshal(list.Obj, &rows); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("library after delete = %+v, want empty", rows)
	}
}

// TestLinkControllerRefreshRejectsNonSubscription keeps the manual refresh from
// being pointed at a plain share link, which has nothing to fetch.
func TestLinkControllerRefreshRejectsNonSubscription(t *testing.T) {
	newHostTestDB(t)
	engine := gin.New()
	NewLinkController(engine.Group("/panel/api/links"))

	add := doHostReq(t, engine, http.MethodPost, "/panel/api/links/add", map[string]any{
		"kind": model.ExternalLinkKindLink, "value": "vless://uuid@plain.example:443", "remark": "plain",
	})
	if !add.Success {
		t.Fatalf("add not successful: %s", add.Msg)
	}
	var created model.ExternalLink
	_ = json.Unmarshal(add.Obj, &created)

	refresh := doHostReq(t, engine, http.MethodPost, "/panel/api/links/refresh/"+strconv.Itoa(created.Id), nil)
	if refresh.Success {
		t.Fatal("refreshing a plain share link must be rejected")
	}
}
