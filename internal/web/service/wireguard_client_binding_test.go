package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func wireGuardSettings(t *testing.T, peers []map[string]any) string {
	t.Helper()
	rawPeers := make([]any, 0, len(peers))
	for _, peer := range peers {
		rawPeers = append(rawPeers, peer)
	}
	data, err := json.Marshal(map[string]any{
		"secretKey":   "server-private-key",
		"peers":       rawPeers,
		"noKernelTun": false,
	})
	if err != nil {
		t.Fatalf("marshal wg settings: %v", err)
	}
	return string(data)
}

func loadWireGuardPeers(t *testing.T, inboundID int) []map[string]any {
	t.Helper()
	var inbound model.Inbound
	if err := database.GetDB().First(&inbound, inboundID).Error; err != nil {
		t.Fatalf("load inbound: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	rawPeers, _ := settings["peers"].([]any)
	peers := make([]map[string]any, 0, len(rawPeers))
	for _, rawPeer := range rawPeers {
		peer, ok := rawPeer.(map[string]any)
		if !ok {
			t.Fatalf("peer has unexpected shape: %#v", rawPeer)
		}
		peers = append(peers, peer)
	}
	return peers
}

func TestWireGuardPeerBindingMatchesByClientIDAndEmail(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	svc := &ClientService{}

	alice := model.ClientRecord{Email: "alice@example.com", SubID: "sub-a", Comment: "Alice", Enable: true}
	bob := model.ClientRecord{Email: "bob@example.com", SubID: "sub-b", Comment: "Bob", Enable: true}
	if err := db.Create(&alice).Error; err != nil {
		t.Fatalf("create alice: %v", err)
	}
	if err := db.Create(&bob).Error; err != nil {
		t.Fatalf("create bob: %v", err)
	}

	wg := mkInbound(t, 51820, model.WireGuard, wireGuardSettings(t, []map[string]any{
		{"publicKey": "peer-a", "allowedIPs": []any{"10.0.0.2/32"}, "clientId": float64(alice.Id), "comment": ""},
		{"publicKey": "peer-b", "allowedIPs": []any{"10.0.0.3/32"}, "clientEmail": "BOB@example.com", "comment": "BOB@example.com"},
	}))
	if err := db.Create(&model.ClientInbound{ClientId: alice.Id, InboundId: wg.Id}).Error; err != nil {
		t.Fatalf("link alice: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: bob.Id, InboundId: wg.Id}).Error; err != nil {
		t.Fatalf("link bob: %v", err)
	}

	if err := svc.syncWireGuardInboundPeerBindings(nil, wg.Id); err != nil {
		t.Fatalf("sync wg: %v", err)
	}

	peers := loadWireGuardPeers(t, wg.Id)
	if got, ok := jsonInt(peers[0]["clientId"]); !ok || got != alice.Id {
		t.Fatalf("alice clientId = %v, want %d", peers[0]["clientId"], alice.Id)
	}
	if peers[0]["clientEmail"] != alice.Email || peers[0]["clientSubId"] != alice.SubID || peers[0]["clientComment"] != alice.Comment {
		t.Fatalf("alice metadata not synced: %#v", peers[0])
	}
	if peers[0]["comment"] != alice.Email {
		t.Fatalf("alice empty comment should become email, got %#v", peers[0]["comment"])
	}
	if got, ok := jsonInt(peers[1]["clientId"]); !ok || got != bob.Id {
		t.Fatalf("bob clientId = %v, want %d", peers[1]["clientId"], bob.Id)
	}
	if peers[1]["clientEmail"] != bob.Email || peers[1]["clientSubId"] != bob.SubID || peers[1]["clientComment"] != bob.Comment {
		t.Fatalf("bob metadata not synced: %#v", peers[1])
	}
	if peers[1]["comment"] != bob.Email {
		t.Fatalf("bob old email comment should normalize to current email, got %#v", peers[1]["comment"])
	}
}

func TestWireGuardPeerBindingGeneratesPeerWhenNoSlotExists(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	rec := model.ClientRecord{Email: "alice@example.com", SubID: "sub-a", Enable: true}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	wg := mkInbound(t, 51821, model.WireGuard, wireGuardSettings(t, nil))

	needRestart, err := svc.Attach(inboundSvc, rec.Id, []int{wg.Id})
	if err != nil {
		t.Fatalf("attach wg: %v", err)
	}
	if !needRestart {
		t.Fatalf("WireGuard attach should request restart")
	}

	peers := loadWireGuardPeers(t, wg.Id)
	if len(peers) != 1 {
		t.Fatalf("generated peers = %d, want 1", len(peers))
	}
	peer := peers[0]
	if peer["privateKey"] == "" || peer["publicKey"] == "" {
		t.Fatalf("generated peer missing keypair: %#v", peer)
	}
	if peer["clientEmail"] != rec.Email {
		t.Fatalf("generated peer not bound to client: %#v", peer)
	}
	allowed, _ := peer["allowedIPs"].([]any)
	if len(allowed) != 1 || allowed[0] != "10.0.0.2/32" {
		t.Fatalf("generated allowedIPs = %#v, want 10.0.0.2/32", allowed)
	}
}

func TestWireGuardDetachMarksPeerDetachedAndRuntimeFiltersIt(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	rec := model.ClientRecord{Email: "alice@example.com", SubID: "sub-a", Enable: true}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	wg := mkInbound(t, 51822, model.WireGuard, wireGuardSettings(t, []map[string]any{
		{
			"privateKey":    "client-private",
			"publicKey":     "client-public",
			"allowedIPs":    []any{"10.0.0.2/32"},
			"clientId":      rec.Id,
			"clientEmail":   rec.Email,
			"clientSubId":   rec.SubID,
			"clientComment": "Alice",
		},
		{"publicKey": "legacy-public", "allowedIPs": []any{"10.0.0.3/32"}},
	}))
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: wg.Id}).Error; err != nil {
		t.Fatalf("link client: %v", err)
	}

	needRestart, err := svc.Detach(inboundSvc, rec.Id, []int{wg.Id})
	if err != nil {
		t.Fatalf("detach wg: %v", err)
	}
	if !needRestart {
		t.Fatalf("WireGuard detach should request restart")
	}

	peers := loadWireGuardPeers(t, wg.Id)
	if peers[0]["clientDetached"] != true {
		t.Fatalf("detached peer should keep metadata and mark clientDetached: %#v", peers[0])
	}

	var reloaded model.Inbound
	if err := db.First(&reloaded, wg.Id).Error; err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	runtimeInbound, err := inboundSvc.buildRuntimeInboundForAPI(db, &reloaded)
	if err != nil {
		t.Fatalf("build runtime inbound: %v", err)
	}
	var runtimeSettings map[string]any
	if err := json.Unmarshal([]byte(runtimeInbound.Settings), &runtimeSettings); err != nil {
		t.Fatalf("parse runtime settings: %v", err)
	}
	runtimePeers, _ := runtimeSettings["peers"].([]any)
	if len(runtimePeers) != 1 {
		t.Fatalf("runtime peers = %d, want only legacy unmanaged peer", len(runtimePeers))
	}
	runtimePeer := runtimePeers[0].(map[string]any)
	if runtimePeer["publicKey"] != "legacy-public" {
		t.Fatalf("unexpected runtime peer: %#v", runtimePeer)
	}
	if _, exists := runtimePeer["privateKey"]; exists {
		t.Fatalf("runtime peer should not include panel privateKey: %#v", runtimePeer)
	}
}

func TestWireGuardBindingNonWireGuardNoop(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	settings := `{"clients":[],"peers":[{"publicKey":"not-wg","clientEmail":"stale@example.com"}]}`
	ib := mkInbound(t, 51823, model.VLESS, settings)

	if err := svc.syncWireGuardInboundPeerBindings(nil, ib.Id); err != nil {
		t.Fatalf("sync non-wg: %v", err)
	}

	var reloaded model.Inbound
	if err := database.GetDB().First(&reloaded, ib.Id).Error; err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	if reloaded.Settings != settings {
		t.Fatalf("non-WireGuard settings changed:\ngot  %s\nwant %s", reloaded.Settings, settings)
	}
}
