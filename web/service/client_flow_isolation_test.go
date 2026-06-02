package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
)

func TestClientWithInboundFlow_GatesByInboundCapability(t *testing.T) {
	const vision = "xtls-rprx-vision"
	cases := []struct {
		name           string
		protocol       model.Protocol
		streamSettings string
		wantFlow       string
	}{
		{"vless tcp reality keeps flow", model.VLESS, `{"network":"tcp","security":"reality"}`, vision},
		{"vless tcp tls keeps flow", model.VLESS, `{"network":"tcp","security":"tls"}`, vision},
		{"vless ws tls clears flow", model.VLESS, `{"network":"ws","security":"tls"}`, ""},
		{"vless grpc tls clears flow", model.VLESS, `{"network":"grpc","security":"tls"}`, ""},
		{"vless tcp none clears flow", model.VLESS, `{"network":"tcp","security":"none"}`, ""},
		{"vmess tcp tls clears flow", model.VMESS, `{"network":"tcp","security":"tls"}`, ""},
		{"empty stream clears flow", model.VLESS, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ib := &model.Inbound{Protocol: tc.protocol, StreamSettings: tc.streamSettings}
			got := clientWithInboundFlow(model.Client{Email: "x@example.com", Flow: vision}, ib)
			if got.Flow != tc.wantFlow {
				t.Errorf("Flow = %q, want %q", got.Flow, tc.wantFlow)
			}
		})
	}
}

func TestFlowIsolation_VisionDoesNotLeakToWsInbound(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()

	wsTls := &model.Inbound{Tag: "vless-ws", Enable: true, Port: 30001, Protocol: model.VLESS, StreamSettings: `{"network":"ws","security":"tls"}`}
	if err := db.Create(wsTls).Error; err != nil {
		t.Fatalf("create ws+tls inbound: %v", err)
	}
	reality := &model.Inbound{Tag: "vless-reality", Enable: true, Port: 30002, Protocol: model.VLESS, StreamSettings: `{"network":"tcp","security":"reality"}`}
	if err := db.Create(reality).Error; err != nil {
		t.Fatalf("create reality inbound: %v", err)
	}

	svc := ClientService{}
	const email = "shared@example.com"
	const uid = "ce8d33df-3a64-4f10-8f9b-91c3a8e0c003"
	const vision = "xtls-rprx-vision"

	source := model.Client{Email: email, ID: uid, Enable: true, Flow: vision}
	for _, ib := range []*model.Inbound{wsTls, reality} {
		gated := clientWithInboundFlow(source, ib)
		if err := svc.SyncInbound(nil, ib.Id, []model.Client{gated}); err != nil {
			t.Fatalf("SyncInbound(%s): %v", ib.Tag, err)
		}
	}

	realityList, err := svc.ListForInbound(nil, reality.Id)
	if err != nil {
		t.Fatalf("ListForInbound(reality): %v", err)
	}
	if len(realityList) != 1 || realityList[0].Flow != vision {
		t.Errorf("Reality inbound should keep flow=%q, got %#v", vision, realityList)
	}

	wsList, err := svc.ListForInbound(nil, wsTls.Id)
	if err != nil {
		t.Fatalf("ListForInbound(ws): %v", err)
	}
	if len(wsList) != 1 || wsList[0].Flow != "" {
		t.Errorf("WS+TLS inbound must not inherit Vision flow (#4628), got %#v", wsList)
	}
}

func TestEffectiveFlow_NonFlowInboundSyncedLastDoesNotHideVision(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()
	reality := &model.Inbound{Tag: "vless-reality", Enable: true, Port: 40001, Protocol: model.VLESS, StreamSettings: `{"network":"tcp","security":"reality"}`}
	if err := db.Create(reality).Error; err != nil {
		t.Fatalf("create reality inbound: %v", err)
	}
	hysteria := &model.Inbound{Tag: "hysteria", Enable: true, Port: 40002, Protocol: model.Hysteria, StreamSettings: `{"security":"tls"}`}
	if err := db.Create(hysteria).Error; err != nil {
		t.Fatalf("create hysteria inbound: %v", err)
	}

	svc := ClientService{}
	const email = "shared@example.com"
	const uid = "ce8d33df-3a64-4f10-8f9b-91c3a8e0c099"
	const vision = "xtls-rprx-vision"

	source := model.Client{Email: email, ID: uid, Auth: uid, Enable: true, Flow: vision}
	// Reproduce #4792 ordering: the flow-capable inbound (Reality) syncs first,
	// the non-flow inbound (Hysteria) syncs last and wipes clients.Flow to "".
	for _, ib := range []*model.Inbound{reality, hysteria} {
		gated := clientWithInboundFlow(source, ib)
		if err := svc.SyncInbound(nil, ib.Id, []model.Client{gated}); err != nil {
			t.Fatalf("SyncInbound(%s): %v", ib.Tag, err)
		}
	}

	rec, err := svc.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	if rec.Flow != "" {
		t.Logf("note: canonical clients.Flow = %q (denormalized, not authoritative)", rec.Flow)
	}

	got, err := svc.EffectiveFlow(nil, rec.Id)
	if err != nil {
		t.Fatalf("EffectiveFlow: %v", err)
	}
	if got != vision {
		t.Errorf("EffectiveFlow = %q, want %q — the edit form would show a blank flow (#4792)", got, vision)
	}
}

func TestEffectiveFlow_ClearedFlowStaysCleared(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()
	reality := &model.Inbound{Tag: "vless-reality", Enable: true, Port: 41001, Protocol: model.VLESS, StreamSettings: `{"network":"tcp","security":"reality"}`}
	if err := db.Create(reality).Error; err != nil {
		t.Fatalf("create reality inbound: %v", err)
	}
	hysteria := &model.Inbound{Tag: "hysteria", Enable: true, Port: 41002, Protocol: model.Hysteria, StreamSettings: `{"security":"tls"}`}
	if err := db.Create(hysteria).Error; err != nil {
		t.Fatalf("create hysteria inbound: %v", err)
	}

	svc := ClientService{}
	const email = "noflow@example.com"
	const uid = "ce8d33df-3a64-4f10-8f9b-91c3a8e0c0aa"

	// User chose no flow: every inbound carries "". A non-empty guard in
	// SyncInbound would make this impossible to express; EffectiveFlow must
	// still report "".
	source := model.Client{Email: email, ID: uid, Auth: uid, Enable: true, Flow: ""}
	for _, ib := range []*model.Inbound{reality, hysteria} {
		gated := clientWithInboundFlow(source, ib)
		if err := svc.SyncInbound(nil, ib.Id, []model.Client{gated}); err != nil {
			t.Fatalf("SyncInbound(%s): %v", ib.Tag, err)
		}
	}

	rec, err := svc.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	got, err := svc.EffectiveFlow(nil, rec.Id)
	if err != nil {
		t.Fatalf("EffectiveFlow: %v", err)
	}
	if got != "" {
		t.Errorf("EffectiveFlow = %q, want empty (cleared flow must stay cleared)", got)
	}
}
