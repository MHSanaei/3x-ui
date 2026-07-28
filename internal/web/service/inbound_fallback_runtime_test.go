package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// TestBuildRuntimeInboundForAPI_InjectsFallbacks is the #5963 regression: the
// runtime inbound sent to nodes never carried the inbound_fallbacks rows, so
// fallbacks configured on the master silently vanished from every node.
func TestBuildRuntimeInboundForAPI_InjectsFallbacks(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	clients := []model.Client{
		{Email: "fb@x", ID: "11111111-1111-1111-1111-111111111111", Enable: true},
	}
	master := mkInbound(t, 30201, model.VLESS, clientsSettings(t, clients))
	master.StreamSettings = `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"s"}}`
	if err := db.Save(master).Error; err != nil {
		t.Fatalf("save master stream: %v", err)
	}

	fb := model.InboundFallback{MasterId: master.Id, ChildId: 0, Dest: "8081", Alpn: "h2", Path: "/fb", Xver: 1}
	if err := db.Create(&fb).Error; err != nil {
		t.Fatalf("seed fallback: %v", err)
	}

	runtimeIb, err := svc.buildRuntimeInboundForAPI(db, master)
	if err != nil {
		t.Fatalf("buildRuntimeInboundForAPI: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal([]byte(runtimeIb.Settings), &settings); err != nil {
		t.Fatalf("runtime settings not valid json: %v", err)
	}
	fallbacks, ok := settings["fallbacks"].([]any)
	if !ok || len(fallbacks) != 1 {
		t.Fatalf("runtime settings must carry the fallback, got: %s", runtimeIb.Settings)
	}
	if !strings.Contains(runtimeIb.Settings, `"dest"`) || !strings.Contains(runtimeIb.Settings, "8081") {
		t.Fatalf("fallback dest missing from runtime settings: %s", runtimeIb.Settings)
	}
}

// A non-fallback-capable inbound (ws transport) must stay untouched.
func TestBuildRuntimeInboundForAPI_NoFallbacksOnWsInbound(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	clients := []model.Client{
		{Email: "ws@x", ID: "22222222-2222-2222-2222-222222222222", Enable: true},
	}
	ib := mkInbound(t, 30202, model.VLESS, clientsSettings(t, clients))
	ib.StreamSettings = `{"network":"ws","security":"none"}`
	if err := db.Save(ib).Error; err != nil {
		t.Fatalf("save stream: %v", err)
	}
	fb := model.InboundFallback{MasterId: ib.Id, Dest: "8082"}
	if err := db.Create(&fb).Error; err != nil {
		t.Fatalf("seed fallback: %v", err)
	}

	runtimeIb, err := svc.buildRuntimeInboundForAPI(db, ib)
	if err != nil {
		t.Fatalf("buildRuntimeInboundForAPI: %v", err)
	}
	if strings.Contains(runtimeIb.Settings, "fallbacks") {
		t.Fatalf("ws inbound must not receive fallbacks: %s", runtimeIb.Settings)
	}
}
