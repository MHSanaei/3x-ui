package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestTrafficDisableImmediatelyUpdatesNodeRuntime(t *testing.T) {
	setupConflictDB(t)
	nodeID, fake := setupNodeRuntime(t)
	client := model.Client{Email: "spent-node", Enable: true}
	ib := nodeInbound(t, nodeID, 46301, []model.Client{client})
	if err := database.GetDB().Create(&xray.ClientTraffic{
		InboundId: ib.Id, Email: client.Email, Enable: true, Up: 100, Total: 100,
	}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	if _, _, _, err := (&InboundService{}).addTrafficLocked(nil, nil); err != nil {
		t.Fatalf("addTrafficLocked: %v", err)
	}
	if got := fake.updateInbound.Load(); got != 1 {
		t.Fatalf("remote UpdateInbound calls = %d, want 1 after commit", got)
	}
}

func TestTrafficDisableRefreshesLocalMTProtoSidecar(t *testing.T) {
	setupConflictDB(t)
	mgr := runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }})
	fake := &fakeNodeRuntime{}
	mgr.SetLocalRuntimeOverride(fake)
	runtime.SetManager(mgr)
	t.Cleanup(func() { runtime.SetManager(nil) })

	seedInboundConflict(t, "mt-spent", "", 46302, model.MTProto, "",
		`{"clients":[{"email":"spent-mt","secret":"`+mtprotoTestSecretA+`","enable":true}]}`)
	ib := loadInboundByTag(t, "mt-spent")
	clients, err := (&InboundService{}).GetClients(ib)
	if err != nil {
		t.Fatalf("GetClients: %v", err)
	}
	if err := (&ClientService{}).SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}
	seedClientTraffic(t, ib.Id, "spent-mt", true)
	if err := database.GetDB().Model(&xray.ClientTraffic{}).Where("email = ?", "spent-mt").
		Updates(map[string]any{"up": 100, "total": 100}).Error; err != nil {
		t.Fatalf("deplete traffic: %v", err)
	}

	if _, _, _, err := (&InboundService{}).addTrafficLocked(nil, nil); err != nil {
		t.Fatalf("addTrafficLocked: %v", err)
	}
	if got := fake.updateInbound.Load(); got != 1 {
		t.Fatalf("MTProto sidecar UpdateInbound calls = %d, want 1 after commit", got)
	}
}

func TestDelDepletedClientsCleansRuntimeAfterCommit(t *testing.T) {
	setupConflictDB(t)
	mgr := runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }})
	fake := &fakeNodeRuntime{}
	mgr.SetLocalRuntimeOverride(fake)
	runtime.SetManager(mgr)
	t.Cleanup(func() { runtime.SetManager(nil) })

	seedInboundConflict(t, "depleted-only", "", 46303, model.VLESS, `{"network":"tcp"}`,
		`{"clients":[{"email":"gone","enable":true}]}`)
	ib := loadInboundByTag(t, "depleted-only")
	seedClientTraffic(t, ib.Id, "gone", true)
	if err := database.GetDB().Model(&xray.ClientTraffic{}).Where("email = ?", "gone").
		Updates(map[string]any{"up": 100, "total": 100, "reset": 0}).Error; err != nil {
		t.Fatalf("deplete traffic: %v", err)
	}

	if err := (&InboundService{}).DelDepletedClients(-1); err != nil {
		t.Fatalf("DelDepletedClients: %v", err)
	}
	if got := fake.delInbound.Load(); got != 1 {
		t.Fatalf("runtime DelInbound calls = %d, want 1 after commit", got)
	}
}
