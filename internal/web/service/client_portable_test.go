package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// TestExportImportPreservesDisabledEnable covers #6478: ExportAll keeps the
// real enable flag; ImportClients must not force enable=true.
func TestExportImportPreservesDisabledEnable(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	ib := mkInbound(t, 26001, model.VLESS, `{"clients":[]}`)
	const email = "portable@disabled"
	const subID = "sub-portable-disabled"
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
		Client: model.Client{
			Email: email, SubID: subID, Enable: true,
			ID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		},
		InboundIds: []int{ib.Id},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	rec := lookupClientRecord(t, email)
	updated := rec.ToClient()
	updated.Enable = false
	if _, err := svc.Update(inboundSvc, rec.Id, *updated, 0); err != nil {
		t.Fatalf("Update disable: %v", err)
	}
	assertEnableEverywhere(t, svc, inboundSvc, ib.Id, email, false)

	exported, err := svc.ExportAll()
	if err != nil {
		t.Fatalf("ExportAll: %v", err)
	}
	if len(exported) != 1 {
		t.Fatalf("ExportAll len=%d, want 1", len(exported))
	}
	if exported[0].Client.Enable {
		t.Fatal("ExportAll should carry enable=false for a disabled client")
	}

	raw, err := json.Marshal(exported)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}
	var roundTrip []ClientCreatePayload
	if err := json.Unmarshal(raw, &roundTrip); err != nil {
		t.Fatalf("unmarshal export: %v", err)
	}
	if roundTrip[0].Client.Enable {
		t.Fatal("JSON round-trip lost enable=false")
	}

	if _, err := svc.Delete(inboundSvc, rec.Id, false); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	res, _, err := svc.ImportClients(inboundSvc, roundTrip)
	if err != nil {
		t.Fatalf("ImportClients: %v", err)
	}
	if res.Created != 1 || len(res.Skipped) != 0 {
		t.Fatalf("ImportClients result=%+v", res)
	}
	assertEnableEverywhere(t, svc, inboundSvc, ib.Id, email, false)
}

func TestImportClientsPreservesOrphanDisabledEnable(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}

	items := []ClientCreatePayload{{
		Client: model.Client{
			Email: "orphan@disabled", SubID: "sub-orphan-disabled", Enable: false,
			ID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		},
		InboundIds: nil,
	}}
	res, _, err := svc.ImportClients(&InboundService{}, items)
	if err != nil {
		t.Fatalf("ImportClients orphan: %v", err)
	}
	if res.Created != 1 {
		t.Fatalf("created=%d, want 1; skipped=%v", res.Created, res.Skipped)
	}
	if got := recordEnableOf(t, svc, "orphan@disabled"); got {
		t.Fatal("orphan import forced enable=true; want false")
	}
}

func TestBulkCreatePreservesExplicitDisable(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	ib := mkInbound(t, 26002, model.VLESS, `{"clients":[]}`)
	const email = "bulk@disabled"
	res, _, err := svc.BulkCreate(inboundSvc, []ClientCreatePayload{{
		Client: model.Client{
			Email: email, SubID: "sub-bulk-disabled", Enable: false,
			ID: "cccccccc-cccc-cccc-cccc-cccccccccccc",
		},
		InboundIds: []int{ib.Id},
	}})
	if err != nil {
		t.Fatalf("BulkCreate: %v", err)
	}
	if res.Created != 1 {
		t.Fatalf("BulkCreate result=%+v", res)
	}
	assertEnableEverywhere(t, svc, inboundSvc, ib.Id, email, false)
}

func TestClientCreatePayload_OmitEnableDefaultsTrue(t *testing.T) {
	raw := []byte(`{"client":{"email":"omit@x","id":"dddddddd-dddd-dddd-dddd-dddddddddddd","subId":"sub-omit"},"inboundIds":[1]}`)
	var p ClientCreatePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !p.Client.Enable {
		t.Fatal("omitted enable must default to true")
	}

	rawFalse := []byte(`{"client":{"email":"off@x","id":"eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee","subId":"sub-off","enable":false},"inboundIds":[1]}`)
	var pFalse ClientCreatePayload
	if err := json.Unmarshal(rawFalse, &pFalse); err != nil {
		t.Fatalf("unmarshal false: %v", err)
	}
	if pFalse.Client.Enable {
		t.Fatal("explicit enable:false must stay false")
	}
}

func TestBulkCreate_DisabledOnNodeSkipsAddClient(t *testing.T) {
	setupBulkDB(t)
	nodeID, fake := setupNodeRuntime(t)
	ib := nodeInbound(t, nodeID, 26003, nil)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	const email = "node@disabled"
	res, _, err := svc.BulkCreate(inboundSvc, []ClientCreatePayload{{
		Client: model.Client{
			Email: email, SubID: "sub-node-disabled", Enable: false,
			ID: "ffffffff-ffff-ffff-ffff-ffffffffffff",
		},
		InboundIds: []int{ib.Id},
	}})
	if err != nil {
		t.Fatalf("BulkCreate: %v", err)
	}
	if res.Created != 1 {
		t.Fatalf("BulkCreate result=%+v", res)
	}
	if got := fake.addClient.Load(); got != 0 {
		t.Fatalf("AddClient RPCs = %d, want 0 for enable=false", got)
	}
	assertEnableEverywhere(t, svc, inboundSvc, ib.Id, email, false)
	if _, _, dirty, _, err := (&NodeService{}).NodeSyncState(nodeID); err != nil {
		t.Fatalf("NodeSyncState: %v", err)
	} else if !dirty {
		t.Fatal("disabled node create must leave node dirty for reconcile")
	}
}

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
			Up: 7, Down: 8, ResetCount: 1,
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

// An orphan's restored row must carry its weekly schedule, or depletedClientsClause
// treats the over-quota renewing client as depleted and DelDepleted deletes it.
func TestImportClientsOrphanTrafficKeepsWeeklyRenewal(t *testing.T) {
	const email = "weekly@orphan"
	cases := []struct {
		name string
		seed func(t *testing.T, svc *ClientService, inboundSvc *InboundService)
	}{
		{name: "no prior row", seed: func(*testing.T, *ClientService, *InboundService) {}},
		{
			name: "row kept by keepTraffic delete",
			seed: func(t *testing.T, svc *ClientService, inboundSvc *InboundService) {
				ib := mkInbound(t, 25003, model.VLESS, `{"clients":[]}`)
				if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
					Client:     model.Client{Email: email, SubID: "sub-weekly-old", Enable: true, TotalGB: 1 << 30},
					InboundIds: []int{ib.Id},
				}); err != nil {
					t.Fatalf("Create: %v", err)
				}
				if _, err := svc.Delete(inboundSvc, lookupClientRecord(t, email).Id, true); err != nil {
					t.Fatalf("Delete keepTraffic: %v", err)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupBulkDB(t)
			svc := &ClientService{}
			inboundSvc := &InboundService{}
			tc.seed(t, svc, inboundSvc)

			items := []ClientCreatePayload{{
				Client: model.Client{
					Email: email, SubID: "sub-weekly-orphan", Enable: true,
					TotalGB: 1 << 30, ResetWeekday: 3,
				},
				Traffic: &ClientPortableTraffic{Up: 1 << 30, Down: 1},
			}}
			if res, _, err := svc.ImportClients(inboundSvc, items); err != nil || res.Created != 1 {
				t.Fatalf("ImportClients result=%+v err=%v, want 1 created", res, err)
			}
			deleted, _, err := svc.DelDepleted(inboundSvc)
			if err != nil {
				t.Fatalf("DelDepleted: %v", err)
			}
			if deleted != 0 {
				t.Fatalf("DelDepleted deleted %d weekly-renewing client(s), want 0", deleted)
			}
			lookupClientRecord(t, email)
		})
	}
}

// Restored counters are usage from before the import, so they must not move the
// group total at import time: a delete+re-import would otherwise count them twice.
func TestImportClientsTrafficLeavesGroupTotalUnchanged(t *testing.T) {
	t.Run("re-import after delete on the same panel", func(t *testing.T) {
		setupBulkDB(t)
		svc := &ClientService{}
		inboundSvc := &InboundService{}
		ib := mkInbound(t, 25004, model.VLESS, `{"clients":[]}`)
		const email = "grouped@traffic"
		if _, err := svc.Create(inboundSvc, &ClientCreatePayload{
			Client:     model.Client{Email: email, SubID: "sub-grouped", Enable: true, Group: "g"},
			InboundIds: []int{ib.Id},
		}); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if err := database.GetDB().Model(&xray.ClientTraffic{}).Where("email = ?", email).
			Updates(map[string]any{"up": 100, "down": 200}).Error; err != nil {
			t.Fatalf("seed traffic: %v", err)
		}
		exported, err := svc.ExportAll()
		if err != nil {
			t.Fatalf("ExportAll: %v", err)
		}
		if _, err := svc.Delete(inboundSvc, lookupClientRecord(t, email).Id, false); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if g := groupByName(t, svc, "g"); g.TrafficUsed != 300 {
			t.Fatalf("group after delete = %d, want the kept 300", g.TrafficUsed)
		}

		if res, _, err := svc.ImportClients(inboundSvc, exported); err != nil || res.Created != 1 {
			t.Fatalf("ImportClients result=%+v err=%v, want 1 created", res, err)
		}
		if g := groupByName(t, svc, "g"); g.Up != 100 || g.Down != 200 {
			t.Fatalf("group after re-import up=%d down=%d, want unchanged 100/200", g.Up, g.Down)
		}
	})

	t.Run("new panel starts the group at zero", func(t *testing.T) {
		setupBulkDB(t)
		svc := &ClientService{}
		ib := mkInbound(t, 25005, model.VLESS, `{"clients":[]}`)
		items := []ClientCreatePayload{
			{Client: model.Client{Email: "attached@g", SubID: "sub-attached-g", Enable: true, Group: "g"}, InboundIds: []int{ib.Id}, Traffic: &ClientPortableTraffic{Up: 100, Down: 200}},
			{Client: model.Client{Email: "orphan@g", SubID: "sub-orphan-g", Enable: true, Group: "g"}, Traffic: &ClientPortableTraffic{Up: 10, Down: 20}},
		}
		if res, _, err := svc.ImportClients(&InboundService{}, items); err != nil || res.Created != 2 {
			t.Fatalf("ImportClients result=%+v err=%v, want 2 created", res, err)
		}
		if g := groupByName(t, svc, "g"); g.TrafficUsed != 0 {
			t.Fatalf("group after import = %d (up=%d down=%d), want 0", g.TrafficUsed, g.Up, g.Down)
		}
	})
}

// A duplicate email in the file is skipped, but the copy that was created must
// still get its own counters rather than none or the skipped copy's.
func TestImportClientsDuplicateEmailRestoresCreatedCopy(t *testing.T) {
	cases := []struct {
		name  string
		items func(ibID int) []ClientCreatePayload
	}{
		{
			name: "second attached copy skipped",
			items: func(ibID int) []ClientCreatePayload {
				return []ClientCreatePayload{
					{Client: model.Client{Email: "dup@traffic", SubID: "sub-dup-a", Enable: true}, InboundIds: []int{ibID}, Traffic: &ClientPortableTraffic{Up: 11, Down: 12}},
					{Client: model.Client{Email: "dup@traffic", SubID: "sub-dup-b", Enable: true}, InboundIds: []int{ibID}, Traffic: &ClientPortableTraffic{Up: 99, Down: 99}},
				}
			},
		},
		{
			name: "earlier orphan copy skipped",
			items: func(ibID int) []ClientCreatePayload {
				return []ClientCreatePayload{
					{Client: model.Client{Email: "dup@traffic", SubID: "sub-dup-b", Enable: true}, Traffic: &ClientPortableTraffic{Up: 99, Down: 99}},
					{Client: model.Client{Email: "dup@traffic", SubID: "sub-dup-a", Enable: true}, InboundIds: []int{ibID}, Traffic: &ClientPortableTraffic{Up: 11, Down: 12}},
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupBulkDB(t)
			svc := &ClientService{}
			ib := mkInbound(t, 25002, model.VLESS, `{"clients":[]}`)

			res, _, err := svc.ImportClients(&InboundService{}, tc.items(ib.Id))
			if err != nil {
				t.Fatalf("ImportClients: %v", err)
			}
			if res.Created != 1 || len(res.Skipped) != 1 {
				t.Fatalf("ImportClients result=%+v, want 1 created and 1 skipped", res)
			}
			var row xray.ClientTraffic
			if err := database.GetDB().Where("email = ?", "dup@traffic").First(&row).Error; err != nil {
				t.Fatalf("lookup traffic: %v", err)
			}
			if row.Up != 11 || row.Down != 12 {
				t.Fatalf("traffic up=%d down=%d, want the created copy's 11/12", row.Up, row.Down)
			}
		})
	}
}
