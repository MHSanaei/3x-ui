package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

func readClientTraffic(t *testing.T, email string) xray.ClientTraffic {
	t.Helper()
	var row xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", email).First(&row).Error; err != nil {
		t.Fatalf("read client_traffics %s: %v", email, err)
	}
	return row
}

// The inbound modal posts the clients it loaded on open; a renewal committed
// while it was open must survive the save in settings, record and stats.
func TestInboundFormSaveKeepsClientRenewedWhileOpen(t *testing.T) {
	setupBulkDB(t)
	ib := seedRenewableNeighbour(t, 23201, nil)
	form := *ib

	if err := database.GetDB().Transaction(autoRenewTick); err != nil {
		t.Fatalf("autoRenew: %v", err)
	}
	form.Remark = "edited"
	if _, _, err := (&InboundService{}).UpdateInbound(&form); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}

	requireNeighbourRenewed(t, ib.Id)
	now := time.Now().UnixMilli()
	if row := readClientTraffic(t, "y@stale"); !row.Enable || row.ExpiryTime <= now {
		t.Fatalf("client_traffics rolled back: enable=%v expiryTime=%d", row.Enable, row.ExpiryTime)
	}
	if rec := lookupClientRecord(t, "y@stale"); !rec.Enable || rec.ExpiryTime <= now {
		t.Fatalf("client record rolled back: enable=%v expiryTime=%d", rec.Enable, rec.ExpiryTime)
	}
}

// On a node the master's push is authoritative, lifecycle fields included.
func TestInboundUpdateFromMasterAppliesClientLifecycle(t *testing.T) {
	setupBulkDB(t)
	ib := seedRenewableNeighbour(t, 23202, nil)
	clients, err := (&InboundService{}).GetClients(ib)
	if err != nil {
		t.Fatalf("GetClients: %v", err)
	}
	for i := range clients {
		if clients[i].Email == "x@stale" {
			clients[i].Enable = false
		}
	}
	push := *ib
	push.Settings = clientsSettings(t, clients)
	if _, _, err := (&InboundService{FromNodeSync: true}).UpdateInbound(&push); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}
	if x, _ := settingsClient(t, ib.Id, "x@stale"); x.Enable {
		t.Fatal("master push disabling x@stale was ignored in settings")
	}
	if readClientTraffic(t, "x@stale").Enable {
		t.Fatal("master push disabling x@stale was ignored in client_traffics")
	}
}

// Traffic the poll adds after UpdateInbound read the row must not be written
// back over by the edit.
func TestInboundUpdateKeepsTrafficAddedMidEdit(t *testing.T) {
	setupBulkDB(t)
	ib := seedRenewableNeighbour(t, 23203, nil)
	form := *ib
	form.Remark = "edited"
	commitTickBetweenReadAndWrite(t, func(tx *gorm.DB) error {
		return (&InboundService{}).addInboundTraffic(tx, []*xray.Traffic{{IsInbound: true, Tag: ib.Tag, Up: 100, Down: 50}})
	}, func() {
		if _, _, err := (&InboundService{}).UpdateInbound(&form); err != nil {
			t.Errorf("UpdateInbound: %v", err)
		}
	})
	saved, err := (&InboundService{}).GetInbound(ib.Id)
	if err != nil {
		t.Fatalf("GetInbound: %v", err)
	}
	if saved.Up != 100 || saved.Down != 50 || saved.Remark != "edited" {
		t.Fatalf("inbound after edit: up=%d down=%d remark=%q, want 100/50/edited", saved.Up, saved.Down, saved.Remark)
	}
}

func inboundLinksEmail(t *testing.T, inboundId int, email string) bool {
	t.Helper()
	var n int64
	if err := database.GetDB().Table("client_inbounds").
		Joins("JOIN clients ON clients.id = client_inbounds.client_id").
		Where("client_inbounds.inbound_id = ? AND clients.email = ?", inboundId, email).
		Count(&n).Error; err != nil {
		t.Fatalf("count links: %v", err)
	}
	return n > 0
}

// A client added while the modal was open is not in the list it posts back;
// saving the inbound must not detach it.
func TestInboundFormSaveKeepsClientAddedWhileOpen(t *testing.T) {
	setupBulkDB(t)
	ib := seedRenewableNeighbour(t, 23204, nil)
	form := *ib

	if _, err := (&ClientService{}).AddInboundClient(&InboundService{}, &model.Inbound{
		Id: ib.Id, Settings: clientsSettings(t, []model.Client{{Email: "z@stale", ID: "aaaaaaaa-0000-0000-0000-00000000000c", Enable: true}}),
	}); err != nil {
		t.Fatalf("AddInboundClient: %v", err)
	}
	form.Remark = "edited"
	if _, _, err := (&InboundService{}).UpdateInbound(&form); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}
	if _, ok := settingsClient(t, ib.Id, "z@stale"); !ok || !inboundLinksEmail(t, ib.Id, "z@stale") {
		t.Fatalf("client added while the form was open was dropped: in settings=%v linked=%v", ok, inboundLinksEmail(t, ib.Id, "z@stale"))
	}
}

// A client deleted while the modal was open is still in the list it posts
// back; saving must not restore its access.
func TestInboundFormSaveDoesNotResurrectDeletedClient(t *testing.T) {
	setupBulkDB(t)
	ib := seedRenewableNeighbour(t, 23205, nil)
	form := *ib

	if _, err := (&ClientService{}).DelInboundClientByEmail(&InboundService{}, ib.Id, "x@stale", false, true); err != nil {
		t.Fatalf("DelInboundClientByEmail: %v", err)
	}
	form.Remark = "edited"
	if _, _, err := (&InboundService{}).UpdateInbound(&form); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}
	if _, ok := settingsClient(t, ib.Id, "x@stale"); ok || inboundLinksEmail(t, ib.Id, "x@stale") {
		t.Fatalf("deleted client came back: in settings=%v linked=%v", ok, inboundLinksEmail(t, ib.Id, "x@stale"))
	}
}

// An inbound switched off while the modal was open stays off when the form,
// which still holds enable=true, is saved.
func TestInboundFormSaveKeepsEnableToggledWhileOpen(t *testing.T) {
	setupBulkDB(t)
	ib := seedRenewableNeighbour(t, 23206, nil)
	form := *ib

	if _, err := (&InboundService{}).SetInboundEnable(ib.Id, false); err != nil {
		t.Fatalf("SetInboundEnable: %v", err)
	}
	form.Remark = "edited"
	if _, _, err := (&InboundService{}).UpdateInbound(&form); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}
	saved, err := (&InboundService{}).GetInbound(ib.Id)
	if err != nil {
		t.Fatalf("GetInbound: %v", err)
	}
	if saved.Enable || saved.Remark != "edited" {
		t.Fatalf("after save: enable=%v remark=%q, want disabled and edited", saved.Enable, saved.Remark)
	}
}
