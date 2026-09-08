package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// seedPartlyApplyingClient puts one client on two inbounds and corrupts the second
// one's settings, so a later op succeeds on one inbound and fails on the other.
func seedPartlyApplyingClient(t *testing.T, email string, basePort int) (healthyID, brokenID int) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()
	ids := make([]int, 0, 2)
	for i := range 2 {
		ib := &model.Inbound{
			UserId: 1, Enable: true, Port: basePort + i,
			Tag:      "in-" + string(rune('a'+i)) + "-partial",
			Protocol: model.VLESS, Settings: `{"clients": []}`,
			StreamSettings: `{"network":"tcp","security":"none"}`,
		}
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create inbound %d: %v", i, err)
		}
		ids = append(ids, ib.Id)
	}

	if _, err := (&service.ClientService{}).Create(&service.InboundService{}, &service.ClientCreatePayload{
		Client:     model.Client{Email: email, ID: "11111111-2222-3333-4444-555555555555", SubID: "sub-" + email, Enable: true},
		InboundIds: ids,
	}); err != nil {
		t.Fatalf("seed Create across both inbounds: %v", err)
	}

	if err := db.Model(&model.Inbound{}).Where("id = ?", ids[1]).
		Update("settings", `{"clients":`).Error; err != nil {
		t.Fatalf("corrupt inbound %d settings: %v", ids[1], err)
	}
	return ids[0], ids[1]
}

func postCtx(t *testing.T, email string, body any) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "email", Value: email}}
	payload := []byte("{}")
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

// assertPartialApply pins that the op really failed on one inbound, so a green
// test cannot be a plain full success that never exercised the error path.
func assertPartialApply(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	var msg entity.Msg
	if err := json.Unmarshal(w.Body.Bytes(), &msg); err != nil {
		t.Fatalf("decode response %q: %v", w.Body.String(), err)
	}
	if msg.Success {
		t.Fatalf("response reports success=true, want the partial apply to report failure: %q", w.Body.String())
	}
}

// TestUpdateHandlerFlagsRestartOnPartialApply pins that an edit committed on some
// inbounds and failed on others still flags Xray, as create/attach already did.
func TestUpdateHandlerFlagsRestartOnPartialApply(t *testing.T) {
	const email = "partial-update@example.com"
	seedPartlyApplyingClient(t, email, 43310)

	a := &ClientController{}
	a.xrayService.IsNeedRestartAndSetFalse()
	c, w := postCtx(t, email, map[string]any{
		"email": email, "id": "11111111-2222-3333-4444-555555555555",
		"subId": "sub-" + email, "enable": true, "comment": "edited",
	})
	a.update(c)

	assertPartialApply(t, w)
	if !a.xrayService.IsNeedRestartAndSetFalse() {
		t.Fatal("a partly-applied client edit left Xray unflagged for restart")
	}
}

// TestDeleteHandlerFlagsRestartOnPartialApply is the delete-side twin: the
// removals that landed still need the restart the error path used to discard.
func TestDeleteHandlerFlagsRestartOnPartialApply(t *testing.T) {
	const email = "partial-delete@example.com"
	seedPartlyApplyingClient(t, email, 43320)

	a := &ClientController{}
	a.xrayService.IsNeedRestartAndSetFalse()
	c, w := postCtx(t, email, nil)
	a.delete(c)

	assertPartialApply(t, w)
	if !a.xrayService.IsNeedRestartAndSetFalse() {
		t.Fatal("a partly-applied client delete left Xray unflagged for restart")
	}
}

// TestDetachHandlerFlagsRestartOnPartialApply covers the third converted path.
func TestDetachHandlerFlagsRestartOnPartialApply(t *testing.T) {
	const email = "partial-detach@example.com"
	healthyID, brokenID := seedPartlyApplyingClient(t, email, 43330)

	a := &ClientController{}
	a.xrayService.IsNeedRestartAndSetFalse()
	c, w := postCtx(t, email, attachDetachBody{InboundIds: []int{healthyID, brokenID}})
	a.detach(c)

	assertPartialApply(t, w)
	if !a.xrayService.IsNeedRestartAndSetFalse() {
		t.Fatal("a partly-applied client detach left Xray unflagged for restart")
	}
}
