package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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

// TestLinkControllerReorderAppliesOrder drives the drag-and-drop endpoint: the
// order the page posts has to be the order the next list returns, or the row
// snaps back on refresh.
func TestLinkControllerReorderAppliesOrder(t *testing.T) {
	newHostTestDB(t)
	engine := gin.New()
	NewLinkController(engine.Group("/panel/api/links"))

	db := database.GetDB()
	rows := []model.ExternalLink{
		{Kind: model.ExternalLinkKindLink, Value: "trojan://reorder-1", Remark: "r1"},
		{Kind: model.ExternalLinkKindLink, Value: "trojan://reorder-2", Remark: "r2"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed library: %v", err)
	}

	reorder := doHostReq(t, engine, http.MethodPost, "/panel/api/links/reorder", map[string]any{
		"ids": []int{rows[1].Id, rows[0].Id},
	})
	if !reorder.Success {
		t.Fatalf("reorder not successful: %s", reorder.Msg)
	}
	var echo struct {
		Ids []int `json:"ids"`
	}
	if err := json.Unmarshal(reorder.Obj, &echo); err != nil {
		t.Fatalf("decode reorder response: %v", err)
	}
	if len(echo.Ids) != 2 || echo.Ids[0] != rows[1].Id {
		t.Fatalf("reorder response ids = %v, want the posted order", echo.Ids)
	}

	list := doHostReq(t, engine, http.MethodGet, "/panel/api/links/list", nil)
	var listed []model.ExternalLink
	if err := json.Unmarshal(list.Obj, &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed) != 2 || listed[0].Id != rows[1].Id || listed[1].Id != rows[0].Id {
		t.Fatalf("list order after reorder = %v, want %d then %d",
			[]int{listed[0].Id, listed[1].Id}, rows[1].Id, rows[0].Id)
	}

	unknown := doHostReq(t, engine, http.MethodPost, "/panel/api/links/reorder", map[string]any{
		"ids": []int{987654},
	})
	if unknown.Success {
		t.Fatal("reorder accepted an id that is not in the library")
	}
}

// TestLinkControllerSyncStoresTheMastersSnapshot drives the receiving half of
// the master-to-node push over HTTP: this is the only place that pins the body
// the master serializes against the endpoint that binds it, so a renamed JSON
// field here would silently stop every node from receiving its links.
func TestLinkControllerSyncStoresTheMastersSnapshot(t *testing.T) {
	newHostTestDB(t)
	engine := gin.New()
	NewLinkController(engine.Group("/panel/api/links"))

	db := database.GetDB()
	const (
		email     = "nodesync@ctl"
		value     = "trojan://master@node.example:443"
		unusedVal = "trojan://master@node.example:8443"
	)
	client := &model.ClientRecord{Email: email, SubID: "nodesync-sub", UUID: "bbbbbbbb-1111-2222-3333-444444444444", Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}

	push := doHostReq(t, engine, http.MethodPost, "/panel/api/links/sync", map[string]any{
		"links": []map[string]any{{
			"kind": model.ExternalLinkKindLink, "value": value, "remark": "from-master",
			"enable": true, "userAgent": "master-ua", "cacheTtl": 90,
			"headers": map[string]string{"X-Token": "abc"},
		}},
		"clients": []map[string]any{{
			"email": email,
			"links": []map[string]any{{
				"kind": model.ExternalLinkKindLink, "value": value, "enable": true,
				"expiryTime": 0, "remark": "resolved-by-master",
			}},
		}},
	})
	if !push.Success {
		t.Fatalf("sync not successful: %s", push.Msg)
	}
	var counts struct {
		Links   int `json:"links"`
		Clients int `json:"clients"`
	}
	if err := json.Unmarshal(push.Obj, &counts); err != nil {
		t.Fatalf("decode sync response: %v", err)
	}
	if counts.Links != 1 || counts.Clients != 1 {
		t.Fatalf("sync response = %+v, want one link and one client", counts)
	}

	var row model.ExternalLink
	if err := db.Where("value = ?", value).First(&row).Error; err != nil {
		t.Fatalf("the pushed link was not stored: %v", err)
	}
	if row.Origin != model.ExternalLinkOriginNode || row.Remark != "from-master" || row.UserAgent != "master-ua" {
		t.Fatalf("stored pushed row = %+v, want an origin=node row carrying the master's fields", row)
	}
	if row.CacheTTL != 90 || row.Headers["X-Token"] != "abc" || row.Enable == nil || !*row.Enable {
		t.Fatalf("stored pushed fetch settings = cacheTtl %d headers %v enable %v", row.CacheTTL, row.Headers, row.Enable)
	}

	var assignment model.ExternalLinkAssignment
	if err := db.Where("link_id = ? AND target_type = ? AND target_id = ?",
		row.Id, model.ExternalLinkTargetClient, client.Id).First(&assignment).Error; err != nil {
		t.Fatalf("the pushed assignment was not stored: %v", err)
	}
	if assignment.ExpiryTime != model.ExternalLinkExpiryNever {
		t.Fatalf("pushed expiry = %d, want ExternalLinkExpiryNever so the row cannot inherit the shared one", assignment.ExpiryTime)
	}
	if assignment.Origin != model.ExternalLinkOriginNode || assignment.Remark != "resolved-by-master" {
		t.Fatalf("stored assignment = %+v, want the master's resolution", assignment)
	}

	// A row nobody inherits is released again, so a node's library does not grow
	// with links the master has already stopped handing out.
	drop := doHostReq(t, engine, http.MethodPost, "/panel/api/links/sync", map[string]any{
		"links": []map[string]any{{"kind": model.ExternalLinkKindLink, "value": unusedVal, "enable": true}},
	})
	if !drop.Success {
		t.Fatalf("sync of an unassigned link not successful: %s", drop.Msg)
	}
	var remaining []model.ExternalLink
	if err := db.Where("value = ?", unusedVal).Find(&remaining).Error; err != nil {
		t.Fatalf("look up the unassigned row: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("a pushed row nobody inherits stayed in the library: %+v", remaining)
	}
	var keptCount int64
	if err := db.Model(&model.ExternalLink{}).Count(&keptCount).Error; err != nil {
		t.Fatalf("count library rows: %v", err)
	}
	if keptCount != 1 {
		t.Fatalf("library rows after the second push = %d, want the assigned one only", keptCount)
	}

	bad := doHostReq(t, engine, http.MethodPost, "/panel/api/links/sync", map[string]any{
		"links": []map[string]any{{"kind": "bogus-kind", "value": value}},
	})
	if bad.Success {
		t.Fatal("a push carrying an unknown kind was accepted")
	}

	// A later push of the same identity is taken over, not duplicated: the
	// master owns that row from here on, and a node keeps serving the newest
	// resolution instead of two rows for one link.
	repush := doHostReq(t, engine, http.MethodPost, "/panel/api/links/sync", map[string]any{
		"links": []map[string]any{{
			"kind": model.ExternalLinkKindLink, "value": value, "remark": "re-pushed",
			"enable": false, "userAgent": "second-ua",
		}},
		"clients": []map[string]any{{
			"email": email,
			"links": []map[string]any{{
				"kind": model.ExternalLinkKindLink, "value": value, "enable": false,
				"expiryTime": 1893456000000, "remark": "resolved-again",
			}},
		}},
	})
	if !repush.Success {
		t.Fatalf("re-push not successful: %s", repush.Msg)
	}
	var after model.ExternalLink
	if err := db.Where("value = ?", value).First(&after).Error; err != nil {
		t.Fatalf("reload the pushed row: %v", err)
	}
	if after.Id != row.Id {
		t.Fatalf("re-push created a second row (%d beside %d) instead of taking the first over", after.Id, row.Id)
	}
	if after.Remark != "re-pushed" || after.UserAgent != "second-ua" || after.Enable == nil || *after.Enable {
		t.Fatalf("re-pushed row = %+v, want the master's newest fields", after)
	}
	var reAssignment model.ExternalLinkAssignment
	if err := db.Where("link_id = ? AND target_type = ? AND target_id = ?",
		row.Id, model.ExternalLinkTargetClient, client.Id).First(&reAssignment).Error; err != nil {
		t.Fatalf("the re-pushed assignment was not stored: %v", err)
	}
	if reAssignment.ExpiryTime != 1893456000000 || reAssignment.Remark != "resolved-again" {
		t.Fatalf("re-pushed assignment = %+v, want the newest resolution", reAssignment)
	}
	var assignments int64
	if err := db.Model(&model.ExternalLinkAssignment{}).
		Where("link_id = ? AND target_id = ?", row.Id, client.Id).Count(&assignments).Error; err != nil {
		t.Fatalf("count assignments: %v", err)
	}
	if assignments != 1 {
		t.Fatalf("assignments after the re-push = %d, want one", assignments)
	}
}

// TestLinkControllerRejectsIdsItCannotUse pins the failure envelope the page
// reads: a row id that is not a positive integer must come back as a failed
// call, not as a silent success with nothing done.
func TestLinkControllerRejectsIdsItCannotUse(t *testing.T) {
	newHostTestDB(t)
	engine := gin.New()
	NewLinkController(engine.Group("/panel/api/links"))

	cases := []struct {
		name string
		meth string
		path string
		body any
	}{
		{"delete without an id", http.MethodPost, "/panel/api/links/del/0", nil},
		{"delete with a word", http.MethodPost, "/panel/api/links/del/not-a-number", nil},
		{"enable without an id", http.MethodPost, "/panel/api/links/enable/0", map[string]any{"enable": true}},
		{"enable with a word", http.MethodPost, "/panel/api/links/enable/x", map[string]any{"enable": true}},
		{"targets without an id", http.MethodGet, "/panel/api/links/targets/0", nil},
		{"targets with a word", http.MethodGet, "/panel/api/links/targets/x", nil},
		{"client links without an id", http.MethodGet, "/panel/api/links/client/0", nil},
		{"client links with a word", http.MethodGet, "/panel/api/links/client/x", nil},
		{"assign without an id", http.MethodPost, "/panel/api/links/assign/0", map[string]any{"global": true}},
		{"refresh without an id", http.MethodPost, "/panel/api/links/refresh/0", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := doHostReq(t, engine, tc.meth, tc.path, tc.body)
			if env.Success {
				t.Fatalf("%s %s succeeded, want a failed call", tc.meth, tc.path)
			}
			if env.Msg == "" {
				t.Fatalf("%s %s failed without a message", tc.meth, tc.path)
			}
		})
	}
}

// TestLinkControllerAnswersAMalformedBodyWithAFailedCall pins the failure shape
// the page reads: it branches on the envelope, not on the status code, so a body
// it cannot bind has to come back as a failed call the UI can toast - a 400 here
// would surface as a raw HTTP error with no message.
func TestLinkControllerAnswersAMalformedBodyWithAFailedCall(t *testing.T) {
	newHostTestDB(t)
	engine := gin.New()
	NewLinkController(engine.Group("/panel/api/links"))

	paths := []string{
		"/panel/api/links/add",
		"/panel/api/links/enable/1",
		"/panel/api/links/reorder",
		"/panel/api/links/assign/1",
		"/panel/api/links/unassign/1",
		"/panel/api/links/sync",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 with a failed envelope, body=%s", w.Code, w.Body.String())
			}
			var env hostEnvelope
			if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
				t.Fatalf("decode envelope: %v body=%s", err, w.Body.String())
			}
			if env.Success {
				t.Fatalf("a malformed body was accepted as a successful call")
			}
			if env.Msg == "" {
				t.Fatal("the failed call carries no message for the UI")
			}
		})
	}
}

// TestLinkControllerRefreshRedownloadsTheSubscription drives the operator's
// refresh button against a real provider: the cache is bypassed, the outcome is
// stamped on the row the page displays, and a provider outage surfaces as a
// failed call rather than as an empty link set.
func TestLinkControllerRefreshRedownloadsTheSubscription(t *testing.T) {
	newHostTestDB(t)
	engine := gin.New()
	NewLinkController(engine.Group("/panel/api/links"))

	body := "trojan://refresh-1@provider.example:443\n# comment\ntrojan://refresh-2@provider.example:443\n"
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer provider.Close()

	add := doHostReq(t, engine, http.MethodPost, "/panel/api/links/add", map[string]any{
		"kind": model.ExternalLinkKindSubscription, "value": provider.URL, "remark": "provider",
	})
	if !add.Success {
		t.Fatalf("add not successful: %s", add.Msg)
	}
	var created model.ExternalLink
	if err := json.Unmarshal(add.Obj, &created); err != nil {
		t.Fatalf("decode created subscription: %v", err)
	}

	refresh := doHostReq(t, engine, http.MethodPost, "/panel/api/links/refresh/"+strconv.Itoa(created.Id), nil)
	if !refresh.Success {
		t.Fatalf("refresh not successful: %s", refresh.Msg)
	}
	var reported struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(refresh.Obj, &reported); err != nil {
		t.Fatalf("decode refresh response: %v", err)
	}
	if reported.Count != 2 {
		t.Fatalf("refresh count = %d, want 2 - the comment line and the empty one are not links", reported.Count)
	}

	var row model.ExternalLink
	if err := database.GetDB().Where("id = ?", created.Id).First(&row).Error; err != nil {
		t.Fatalf("reload the row: %v", err)
	}
	if row.LastFetchAt == 0 || row.LastFetchError != "" {
		t.Fatalf("row after a good refresh = lastFetchAt %d lastFetchError %q, want a timestamp and no error",
			row.LastFetchAt, row.LastFetchError)
	}

	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer broken.Close()
	brokenAdd := doHostReq(t, engine, http.MethodPost, "/panel/api/links/add", map[string]any{
		"kind": model.ExternalLinkKindSubscription, "value": broken.URL, "remark": "broken",
	})
	if !brokenAdd.Success {
		t.Fatalf("add broken provider not successful: %s", brokenAdd.Msg)
	}
	var brokenRow model.ExternalLink
	if err := json.Unmarshal(brokenAdd.Obj, &brokenRow); err != nil {
		t.Fatalf("decode broken provider row: %v", err)
	}

	failed := doHostReq(t, engine, http.MethodPost, "/panel/api/links/refresh/"+strconv.Itoa(brokenRow.Id), nil)
	if failed.Success {
		t.Fatal("refreshing against a provider answering 502 reported success")
	}
	if err := database.GetDB().Where("id = ?", brokenRow.Id).First(&brokenRow).Error; err != nil {
		t.Fatalf("reload the broken provider row: %v", err)
	}
	if brokenRow.LastFetchError == "" || brokenRow.LastFetchAt == 0 {
		t.Fatalf("a failed refresh left no trace on the row: lastFetchAt %d lastFetchError %q",
			brokenRow.LastFetchAt, brokenRow.LastFetchError)
	}
}
