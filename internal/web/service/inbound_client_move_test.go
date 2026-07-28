package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// TestGetClientByEmail_AfterMoveBetweenInbounds is the #6059 regression: a
// client moved to another inbound keeps a client_traffics row pointing at the
// old inbound (which still exists), so email lookups used to fail with
// "Client Not Found In Inbound For Email" and the Telegram bot could not build
// links or QR codes for the client anymore.
func TestGetClientByEmail_AfterMoveBetweenInbounds(t *testing.T) {
	setupBulkDB(t)
	svc := &InboundService{}
	db := database.GetDB()

	oldClients := []model.Client{
		{Email: "stay@x", ID: "11111111-1111-1111-1111-111111111111", Enable: true},
	}
	movedClients := []model.Client{
		{Email: "moved@x", ID: "22222222-2222-2222-2222-222222222222", Enable: true},
	}

	oldIb := mkInbound(t, 30101, model.VLESS, clientsSettings(t, oldClients))
	newIb := mkInbound(t, 30102, model.VLESS, clientsSettings(t, movedClients))
	if err := svc.clientService.SyncInbound(nil, oldIb.Id, oldClients); err != nil {
		t.Fatalf("SyncInbound old: %v", err)
	}
	if err := svc.clientService.SyncInbound(nil, newIb.Id, movedClients); err != nil {
		t.Fatalf("SyncInbound new: %v", err)
	}

	stale := xray.ClientTraffic{InboundId: oldIb.Id, Email: "moved@x", Enable: true, Up: 5, Down: 7}
	if err := db.Create(&stale).Error; err != nil {
		t.Fatalf("seed stale traffic row: %v", err)
	}

	traffic, inbound, err := svc.GetClientInboundByEmail("moved@x")
	if err != nil {
		t.Fatalf("GetClientInboundByEmail: %v", err)
	}
	if traffic == nil || traffic.Email != "moved@x" {
		t.Fatalf("traffic = %+v, want the moved@x row", traffic)
	}
	if inbound == nil || inbound.Id != newIb.Id {
		t.Fatalf("inbound = %+v, want the new inbound %d", inbound, newIb.Id)
	}

	_, client, err := svc.GetClientByEmail("moved@x")
	if err != nil {
		t.Fatalf("GetClientByEmail: %v", err)
	}
	if client == nil || client.Email != "moved@x" {
		t.Fatalf("client = %+v, want moved@x", client)
	}
}
