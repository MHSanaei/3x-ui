package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// restoreVisionFlowForEligibleInbound must re-add Vision to a client whose flow
// was stripped while the XHTTP inbound was not yet vlessenc-encrypted, but only
// when the client's intended flow (its flow_override on a sibling) is Vision,
// only on now-eligible inbounds, and never overwriting an explicit flow.
func TestRestoreVisionFlowForEligibleInbound(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	db := database.GetDB()

	const vision = "xtls-rprx-vision"
	const realityStream = `{"network":"tcp","security":"reality"}`
	const xhttpEnc = `{"network":"xhttp","security":"reality"}`
	const encSettings = `"decryption":"mlkem768x25519plus.native.0rtt.KEY","encryption":"mlkem768x25519plus.native.0rtt.KEY"`

	cs := &ClientService{}
	ibSvc := &InboundService{}

	// Sibling reality inbound where the client legitimately has Vision.
	sibling := &model.Inbound{
		Tag: "sib", Enable: true, Port: 51001, Protocol: model.VLESS, StreamSettings: realityStream,
		Settings: `{"clients":[{"id":"u1","email":"keep@x","flow":"` + vision + `","subId":"s1","enable":true}]}`,
	}
	if err := db.Create(sibling).Error; err != nil {
		t.Fatalf("create sibling: %v", err)
	}
	keep, _ := ibSvc.GetClients(sibling)
	if err := cs.SyncInbound(nil, sibling.Id, keep); err != nil {
		t.Fatalf("sync sibling: %v", err)
	}

	// A client with no intended Vision anywhere — must NOT be touched.
	other := &model.Inbound{
		Tag: "oth", Enable: true, Port: 51002, Protocol: model.VLESS, StreamSettings: realityStream,
		Settings: `{"clients":[{"id":"u2","email":"none@x","subId":"s2","enable":true}]}`,
	}
	if err := db.Create(other).Error; err != nil {
		t.Fatalf("create other: %v", err)
	}
	oc, _ := ibSvc.GetClients(other)
	if err := cs.SyncInbound(nil, other.Id, oc); err != nil {
		t.Fatalf("sync other: %v", err)
	}

	// The now-eligible XHTTP inbound: keep@x has empty flow (was stripped),
	// none@x has empty flow (no Vision anywhere), set@x has an explicit empty
	// stays empty unless intended Vision.
	target := `{` + encSettings + `,"clients":[` +
		`{"id":"u1","email":"keep@x","flow":"","subId":"s1","enable":true},` +
		`{"id":"u2","email":"none@x","flow":"","subId":"s2","enable":true}` +
		`]}`

	out, changed := ibSvc.restoreVisionFlowForEligibleInbound(nil, target, xhttpEnc, model.VLESS)
	if !changed {
		t.Fatal("expected changed=true")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("parse out: %v", err)
	}
	flows := map[string]string{}
	for _, c := range parsed["clients"].([]any) {
		cm := c.(map[string]any)
		flows[cm["email"].(string)], _ = cm["flow"].(string)
	}
	if flows["keep@x"] != vision {
		t.Errorf("keep@x flow = %q, want Vision (intended on sibling)", flows["keep@x"])
	}
	if flows["none@x"] != "" {
		t.Errorf("none@x flow = %q, want empty (no Vision intent)", flows["none@x"])
	}

	// Ineligible inbound (xhttp without encryption) must be a no-op.
	noenc := `{"clients":[{"id":"u1","email":"keep@x","flow":"","subId":"s1","enable":true}]}`
	if _, ch := ibSvc.restoreVisionFlowForEligibleInbound(nil, noenc, `{"network":"xhttp","security":"reality"}`, model.VLESS); ch {
		t.Error("ineligible xhttp (no vlessenc) must not change")
	}
	// Non-VLESS must be a no-op.
	if _, ch := ibSvc.restoreVisionFlowForEligibleInbound(nil, target, xhttpEnc, model.VMESS); ch {
		t.Error("non-VLESS must not change")
	}
}
