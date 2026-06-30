package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

const visionDur = "xtls-rprx-vision"

const encDur = `"decryption":"mlkem768x25519plus.native.0rtt.KEY","encryption":"mlkem768x25519plus.native.0rtt.KEY"`

func initDurDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	return database.GetDB()
}

func settingsFlow(t *testing.T, ibSvc *InboundService, id int, email string) (string, bool) {
	t.Helper()
	ib, err := ibSvc.GetInbound(id)
	if err != nil {
		t.Fatalf("GetInbound(%d): %v", id, err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(ib.Settings), &parsed); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	clients, _ := parsed["clients"].([]any)
	for _, c := range clients {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if cm["email"] == email {
			flow, _ := cm["flow"].(string)
			lock, _ := cm["flowLock"].(bool)
			return flow, lock
		}
	}
	t.Fatalf("client %s not found on inbound %d", email, id)
	return "", false
}

func overrideFlow(t *testing.T, cs *ClientService, id int, email string) string {
	t.Helper()
	list, err := cs.ListForInbound(nil, id)
	if err != nil {
		t.Fatalf("ListForInbound(%d): %v", id, err)
	}
	for i := range list {
		if list[i].Email == email {
			return list[i].Flow
		}
	}
	t.Fatalf("client %s not in ListForInbound(%d)", email, id)
	return ""
}

func TestSetInboundClientFlow_ClearedOverrideSurvivesRestoreAndEdit(t *testing.T) {
	db := initDurDB(t)
	ibSvc := &InboundService{}
	cs := &ClientService{}

	const email = "mixed@x"
	const uid = "ce8d33df-3a64-4f10-8f9b-91c3a8e0e001"

	reality := &model.Inbound{
		Tag: "reality", Enable: true, Port: 53001, Protocol: model.VLESS,
		StreamSettings: `{"network":"tcp","security":"reality"}`,
		Settings:       `{"clients":[{"id":"` + uid + `","email":"` + email + `","flow":"` + visionDur + `","subId":"s1","enable":true}]}`,
	}
	if err := db.Create(reality).Error; err != nil {
		t.Fatalf("create reality: %v", err)
	}
	rc, _ := ibSvc.GetClients(reality)
	if err := cs.SyncInbound(nil, reality.Id, rc); err != nil {
		t.Fatalf("sync reality: %v", err)
	}

	xhttp := &model.Inbound{
		Tag: "xhttp", Enable: true, Port: 53002, Protocol: model.VLESS,
		StreamSettings: `{"network":"xhttp","security":"reality"}`,
		Settings:       `{` + encDur + `,"clients":[{"id":"` + uid + `","email":"` + email + `","flow":"` + visionDur + `","subId":"s1","enable":true}]}`,
	}
	if err := db.Create(xhttp).Error; err != nil {
		t.Fatalf("create xhttp: %v", err)
	}
	xc, _ := ibSvc.GetClients(xhttp)
	if err := cs.SyncInbound(nil, xhttp.Id, xc); err != nil {
		t.Fatalf("sync xhttp: %v", err)
	}

	if f := overrideFlow(t, cs, xhttp.Id, email); f != visionDur {
		t.Fatalf("precondition: xhttp flow_override = %q, want Vision", f)
	}

	if _, err := cs.SetInboundClientFlow(ibSvc, xhttp.Id, email, "none"); err != nil {
		t.Fatalf("SetInboundClientFlow clear: %v", err)
	}

	assertState := func(stage string) {
		if f, lock := settingsFlow(t, ibSvc, xhttp.Id, email); f != "" || !lock {
			t.Errorf("[%s] xhttp settings flow=%q lock=%v, want \"\"/true", stage, f, lock)
		}
		if f := overrideFlow(t, cs, xhttp.Id, email); f != "" {
			t.Errorf("[%s] xhttp flow_override = %q, want empty", stage, f)
		}
		if f := overrideFlow(t, cs, reality.Id, email); f != visionDur {
			t.Errorf("[%s] reality flow_override = %q, want Vision preserved", stage, f)
		}
	}
	assertState("after clear")

	ibSvc.MigrationRestoreVisionFlow()
	assertState("after MigrationRestoreVisionFlow")

	rec, err := cs.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if _, err := cs.Update(ibSvc, rec.Id, model.Client{
		Email: email, ID: uid, Flow: visionDur, SubID: "s1", Enable: true, Comment: "edited",
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	assertState("after whole-client edit")
}

func TestRestoreVisionFlow_SkipsFlowLockedClient(t *testing.T) {
	db := initDurDB(t)
	ibSvc := &InboundService{}
	cs := &ClientService{}

	const email = "pinned@x"
	const uid = "ce8d33df-3a64-4f10-8f9b-91c3a8e0e002"

	sib := &model.Inbound{
		Tag: "sib", Enable: true, Port: 53101, Protocol: model.VLESS,
		StreamSettings: `{"network":"tcp","security":"reality"}`,
		Settings:       `{"clients":[{"id":"` + uid + `","email":"` + email + `","flow":"` + visionDur + `","subId":"s1","enable":true}]}`,
	}
	if err := db.Create(sib).Error; err != nil {
		t.Fatalf("create sib: %v", err)
	}
	sc, _ := ibSvc.GetClients(sib)
	if err := cs.SyncInbound(nil, sib.Id, sc); err != nil {
		t.Fatalf("sync sib: %v", err)
	}

	target := `{` + encDur + `,"clients":[{"id":"` + uid + `","email":"` + email + `","flow":"","flowLock":true,"subId":"s1","enable":true}]}`
	out, changed := ibSvc.restoreVisionFlowForEligibleInbound(nil, target, `{"network":"xhttp","security":"reality"}`, model.VLESS)
	if changed {
		t.Errorf("flow-locked client must not be restored; got changed=true, out=%s", out)
	}

	unpinned := `{` + encDur + `,"clients":[{"id":"` + uid + `","email":"` + email + `","flow":"","subId":"s1","enable":true}]}`
	if _, ch := ibSvc.restoreVisionFlowForEligibleInbound(nil, unpinned, `{"network":"xhttp","security":"reality"}`, model.VLESS); !ch {
		t.Error("control: un-pinned empty-flow client with Vision intent should be restored")
	}
}

func TestSetInboundClientFlow_Validation(t *testing.T) {
	db := initDurDB(t)
	ibSvc := &InboundService{}
	cs := &ClientService{}

	const email = "v@x"
	const uid = "ce8d33df-3a64-4f10-8f9b-91c3a8e0e003"

	reality := &model.Inbound{
		Tag: "reality", Enable: true, Port: 53201, Protocol: model.VLESS,
		StreamSettings: `{"network":"tcp","security":"reality"}`,
		Settings:       `{"clients":[{"id":"` + uid + `","email":"` + email + `","flow":"","subId":"s1","enable":true}]}`,
	}
	if err := db.Create(reality).Error; err != nil {
		t.Fatalf("create reality: %v", err)
	}
	rc, _ := ibSvc.GetClients(reality)
	if err := cs.SyncInbound(nil, reality.Id, rc); err != nil {
		t.Fatalf("sync reality: %v", err)
	}
	ws := &model.Inbound{
		Tag: "ws", Enable: true, Port: 53202, Protocol: model.VLESS,
		StreamSettings: `{"network":"ws","security":"tls"}`,
		Settings:       `{"clients":[{"id":"` + uid + `","email":"` + email + `","flow":"","subId":"s1","enable":true}]}`,
	}
	if err := db.Create(ws).Error; err != nil {
		t.Fatalf("create ws: %v", err)
	}
	wc, _ := ibSvc.GetClients(ws)
	if err := cs.SyncInbound(nil, ws.Id, wc); err != nil {
		t.Fatalf("sync ws: %v", err)
	}

	if _, err := cs.SetInboundClientFlow(ibSvc, reality.Id, email, "bogus-flow"); err == nil {
		t.Error("expected error for unsupported flow value")
	}
	if _, err := cs.SetInboundClientFlow(ibSvc, ws.Id, email, visionDur); err == nil {
		t.Error("expected error setting Vision on a non-flow-capable inbound")
	}
	if _, err := cs.SetInboundClientFlow(ibSvc, reality.Id, "ghost@x", "none"); err == nil {
		t.Error("expected client-not-found error")
	}
	if _, err := cs.SetInboundClientFlow(ibSvc, reality.Id, email, visionDur); err != nil {
		t.Fatalf("SetInboundClientFlow vision: %v", err)
	}
	if f, lock := settingsFlow(t, ibSvc, reality.Id, email); f != visionDur || !lock {
		t.Errorf("reality settings flow=%q lock=%v, want Vision/true", f, lock)
	}
	if f := overrideFlow(t, cs, reality.Id, email); f != visionDur {
		t.Errorf("reality flow_override = %q, want Vision", f)
	}
}
