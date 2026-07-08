package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func wgServerSettings() string {
	return `{"secretKey":"` + wgTestSecretKey() + `","mtu":1420,"clients":[]}`
}

func lookupClientRecord(t *testing.T, email string) model.ClientRecord {
	t.Helper()
	var rec model.ClientRecord
	if err := database.GetDB().Where("email = ?", email).First(&rec).Error; err != nil {
		t.Fatalf("lookup client %q: %v", email, err)
	}
	return rec
}

func TestWireGuardClientAddUpdateDeleteRoundTrip(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	ib := mkInbound(t, 51900, model.WireGuard, wgServerSettings())

	add := &model.Inbound{Id: ib.Id, Protocol: model.WireGuard, Settings: clientsSettings(t, []model.Client{
		{Email: "alice@wg", Enable: true},
	})}
	if _, err := svc.AddInboundClient(inboundSvc, add); err != nil {
		t.Fatalf("AddInboundClient: %v", err)
	}

	list, err := svc.ListForInbound(nil, ib.Id)
	if err != nil {
		t.Fatalf("ListForInbound: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 attached client, got %d", len(list))
	}
	created := list[0]
	if created.PrivateKey == "" || created.PublicKey == "" {
		t.Fatalf("keys not generated/persisted: %+v", created)
	}
	if len(created.AllowedIPs) == 0 {
		t.Fatalf("allowedIPs not allocated: %+v", created)
	}

	rec := lookupClientRecord(t, "alice@wg")
	if rec.PrivateKey == "" || rec.AllowedIPs == "" {
		t.Fatalf("client record missing wg columns: %+v", rec)
	}

	update := &model.Inbound{Id: ib.Id, Protocol: model.WireGuard, Settings: clientsSettings(t, []model.Client{
		{Email: "alice@wg", Enable: true, Comment: "renamed laptop"},
	})}
	if _, err := svc.UpdateInboundClient(inboundSvc, update, "alice@wg"); err != nil {
		t.Fatalf("UpdateInboundClient: %v", err)
	}

	afterUpdate := lookupClientRecord(t, "alice@wg")
	if afterUpdate.PrivateKey != created.PrivateKey {
		t.Fatalf("private key rotated on metadata edit: was %q now %q", created.PrivateKey, afterUpdate.PrivateKey)
	}
	if afterUpdate.PublicKey != created.PublicKey {
		t.Fatalf("public key rotated on metadata edit: was %q now %q", created.PublicKey, afterUpdate.PublicKey)
	}
	if afterUpdate.Comment != "renamed laptop" {
		t.Fatalf("comment not updated: %q", afterUpdate.Comment)
	}

	listAfter, err := svc.ListForInbound(nil, ib.Id)
	if err != nil {
		t.Fatalf("ListForInbound after update: %v", err)
	}
	if len(listAfter) != 1 || len(listAfter[0].AllowedIPs) == 0 {
		t.Fatalf("settings lost wg fields after metadata edit: %+v", listAfter)
	}

	if _, err := svc.DelInboundClientByEmail(inboundSvc, ib.Id, "alice@wg", false, false); err != nil {
		t.Fatalf("DelInboundClientByEmail: %v", err)
	}
	final, err := svc.ListForInbound(nil, ib.Id)
	if err != nil {
		t.Fatalf("ListForInbound after delete: %v", err)
	}
	if len(final) != 0 {
		t.Fatalf("client not detached after delete: %+v", final)
	}
}

func TestWireGuardClientAddToInboundWithoutClientsKey(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	ib := mkInbound(t, 51902, model.WireGuard, `{"secretKey":"`+wgTestSecretKey()+`","mtu":1420,"peers":[]}`)

	add := &model.Inbound{Id: ib.Id, Protocol: model.WireGuard, Settings: clientsSettings(t, []model.Client{
		{Email: "first@wg", Enable: true},
	})}
	if _, err := svc.AddInboundClient(inboundSvc, add); err != nil {
		t.Fatalf("AddInboundClient onto clients-less wireguard inbound: %v", err)
	}

	list, err := svc.ListForInbound(nil, ib.Id)
	if err != nil {
		t.Fatalf("ListForInbound: %v", err)
	}
	if len(list) != 1 || list[0].PrivateKey == "" || len(list[0].AllowedIPs) == 0 {
		t.Fatalf("client not added with generated keys/address: %+v", list)
	}
}

func TestWireGuardClientAllocatesUniqueIPsAcrossTwoAdds(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	ib := mkInbound(t, 51901, model.WireGuard, wgServerSettings())

	for _, email := range []string{"one@wg", "two@wg"} {
		add := &model.Inbound{Id: ib.Id, Protocol: model.WireGuard, Settings: clientsSettings(t, []model.Client{
			{Email: email, Enable: true},
		})}
		if _, err := svc.AddInboundClient(inboundSvc, add); err != nil {
			t.Fatalf("AddInboundClient(%s): %v", email, err)
		}
	}

	list, err := svc.ListForInbound(nil, ib.Id)
	if err != nil {
		t.Fatalf("ListForInbound: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(list))
	}
	if list[0].AllowedIPs[0] == list[1].AllowedIPs[0] {
		t.Fatalf("two adds collided on address %q", list[0].AllowedIPs[0])
	}
}
