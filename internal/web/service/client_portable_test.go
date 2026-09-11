package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestExportImportPreservesTrafficCounters(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	ib := mkInbound(t, 25001, model.VLESS, `{"clients":[]}`)
	const email = "portable@traffic"
	const subID = "sub-portable-traffic"
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client: model.Client{
			Email: email, SubID: subID, Enable: true,
			TotalGB: 10 << 30, ExpiryTime: 1_700_000_000_000,
		},
		InboundIds: []int{ib.Id},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	db := database.GetDB()
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", email).Updates(map[string]any{
		"up": 111, "down": 222, "reset_count": 3, "last_online": 999,
	}).Error; err != nil {
		t.Fatalf("seed traffic: %v", err)
	}

	exported, err := svc.ExportAll()
	if err != nil {
		t.Fatalf("ExportAll: %v", err)
	}
	if len(exported) != 1 {
		t.Fatalf("ExportAll len=%d, want 1", len(exported))
	}
	if exported[0].Traffic == nil {
		t.Fatal("ExportAll missing traffic snapshot")
	}
	if exported[0].Traffic.Up != 111 || exported[0].Traffic.Down != 222 || exported[0].Traffic.ResetCount != 3 {
		t.Fatalf("exported traffic = %+v, want up=111 down=222 resetCount=3", exported[0].Traffic)
	}

	raw, err := json.Marshal(exported)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}
	var roundTrip []ClientCreatePayload
	if err := json.Unmarshal(raw, &roundTrip); err != nil {
		t.Fatalf("unmarshal export: %v", err)
	}
	if roundTrip[0].Traffic == nil || roundTrip[0].Traffic.Up != 111 {
		t.Fatalf("JSON round-trip lost traffic: %+v", roundTrip[0].Traffic)
	}

	rec := lookupClientRecord(t, email)
	if _, err := svc.Delete(inboundSvc, rec.Id, false); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	var gone int64
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", email).Count(&gone).Error; err != nil {
		t.Fatalf("count after delete: %v", err)
	}
	if gone != 0 {
		t.Fatalf("client_traffics still present after delete: %d", gone)
	}

	res, _, err := svc.ImportClients(inboundSvc, roundTrip)
	if err != nil {
		t.Fatalf("ImportClients: %v", err)
	}
	if res.Created != 1 || len(res.Skipped) != 0 {
		t.Fatalf("ImportClients result=%+v", res)
	}
	var restored xray.ClientTraffic
	if err := db.Where("email = ?", email).First(&restored).Error; err != nil {
		t.Fatalf("lookup restored traffic: %v", err)
	}
	if restored.Up != 111 || restored.Down != 222 || restored.ResetCount != 3 || restored.LastOnline != 999 {
		t.Fatalf("restored traffic = %+v, want up=111 down=222 resetCount=3 lastOnline=999", restored)
	}

	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", email).Updates(map[string]any{
		"up": 5000, "down": 6000,
	}).Error; err != nil {
		t.Fatalf("bump live traffic: %v", err)
	}
	// Same email+subId is a BulkCreate reuse (may count as Created), not a hard
	// skip — traffic apply must still refuse to overwrite the live counters.
	if _, _, err := svc.ImportClients(inboundSvc, roundTrip); err != nil {
		t.Fatalf("second ImportClients: %v", err)
	}
	var live xray.ClientTraffic
	if err := db.Where("email = ?", email).First(&live).Error; err != nil {
		t.Fatalf("lookup live traffic: %v", err)
	}
	if live.Up != 5000 || live.Down != 6000 {
		t.Fatalf("re-import of existing email must leave live traffic alone, got up=%d down=%d", live.Up, live.Down)
	}
}

func TestImportClientsAppliesTrafficForOrphans(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}

	items := []ClientCreatePayload{{
		Client: model.Client{
			Email: "orphan@traffic", SubID: "sub-orphan-traffic", Enable: true,
			TotalGB: 1 << 30,
		},
		InboundIds: nil,
		Traffic: &ClientPortableTraffic{
			Up: 7, Down: 8, Total: 1 << 30, ResetCount: 1,
		},
	}}
	res, _, err := svc.ImportClients(&InboundService{}, items)
	if err != nil {
		t.Fatalf("ImportClients orphan: %v", err)
	}
	if res.Created != 1 {
		t.Fatalf("created=%d, want 1", res.Created)
	}
	var traf xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", "orphan@traffic").First(&traf).Error; err != nil {
		t.Fatalf("orphan traffic row missing: %v", err)
	}
	if traf.Up != 7 || traf.Down != 8 || traf.ResetCount != 1 {
		t.Fatalf("orphan traffic = %+v", traf)
	}
}
