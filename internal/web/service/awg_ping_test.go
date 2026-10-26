package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func seedAWGPing(t *testing.T) (*model.Inbound, amneziawg.Peer, *model.ClientRecord) {
	t.Helper()
	setupBulkDB(t)
	c := model.Client{Email: "synthetic-ping", SubID: "synthetic-sub", Enable: true, PublicKey: "synthetic-public", AllowedIPs: []string{"192.0.2.2/32"}}
	data, err := json.Marshal(amneziawg.InboundSettings{Server: &amneziawg.ServerSettings{SubnetIP: "192.0.2.0", SubnetCIDR: 24}, Clients: []model.Client{c}})
	if err != nil {
		t.Fatal(err)
	}
	ib := mkInbound(t, 51820, model.AmneziaWG, string(data))
	if err := (&ClientService{}).SyncInbound(nil, ib.Id, []model.Client{c}); err != nil {
		t.Fatal(err)
	}
	policy := &model.AWGPingPolicy{Enabled: true, Allow: []string{"198.51.100.0/24"}, Deny: []string{"198.51.100.8"}}
	if err := saveAWGPing(database.GetDB(), c.Email, policy); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{Email: c.Email, InboundId: ib.Id, Enable: true}).Error; err != nil {
		t.Fatal(err)
	}
	record := lookupClientRecord(t, c.Email)
	return ib, amneziawg.Peer{Email: c.Email, PublicKey: c.PublicKey}, &record
}

func pingGate(s *InboundService, ib *model.Inbound, peer amneziawg.Peer, up, down int64, identity string) (string, error) {
	return s.RecordAWGPing(ib.Id, peer, netip.MustParseAddr("192.0.2.2"), netip.MustParseAddr("198.51.100.7"), up, down, identity)
}

func TestAWGPingUsesAuthoritativePolicyAndExactNativeCounters(t *testing.T) {
	ib, peer, record := seedAWGPing(t)
	svc := &InboundService{}
	token, err := pingGate(svc, ib, peer, 60, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pingGate(svc, ib, peer, 0, 60, token); err != nil {
		t.Fatal(err)
	}
	var traffic xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", peer.Email).Take(&traffic).Error; err != nil {
		t.Fatal(err)
	}
	if traffic.Up != 60 || traffic.Down != 60 || traffic.LastOnline == 0 {
		t.Fatalf("native counters: %#v", traffic)
	}
	var stored model.Inbound
	if err := database.GetDB().First(&stored, ib.Id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Up != 60 || stored.Down != 60 {
		t.Fatalf("inbound counters: %d/%d", stored.Up, stored.Down)
	}
	// Stale inbound JSON never replaces the independent client permission.
	stale := model.Client{Email: peer.Email, SubID: record.SubID, Enable: true, PublicKey: peer.PublicKey, AWGPing: &model.AWGPingPolicy{Enabled: false}}
	if err := (&ClientService{}).SyncInbound(nil, ib.Id, []model.Client{stale}); err != nil {
		t.Fatal(err)
	}
	if !lookupClientRecord(t, peer.Email).AWGPing.Enabled {
		t.Fatal("parent inbound edit cleared ping permission")
	}
	exported, err := (&ClientService{}).ExportAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(exported) != 1 || exported[0].Client.AWGPing == nil || !exported[0].Client.AWGPing.Enabled {
		t.Fatal("export lost permission")
	}
}

func TestAWGPingDeniedAccountStatesDoNotMutateCounters(t *testing.T) {
	for _, name := range []string{"disabled-policy", "empty-allow", "deny", "disabled-client", "expired-client", "expired-traffic", "quota", "record-quota", "disabled-inbound", "inbound-quota", "unknown-peer", "unknown-source", "other-peer-target", "deleted", "missing-traffic", "kernel"} {
		t.Run(name, func(t *testing.T) {
			ib, peer, record := seedAWGPing(t)
			db := database.GetDB()
			src, dst := netip.MustParseAddr("192.0.2.2"), netip.MustParseAddr("198.51.100.7")
			apply := func(result *gorm.DB) {
				t.Helper()
				if result.Error != nil {
					t.Fatal(result.Error)
				}
			}
			switch name {
			case "disabled-policy":
				if err := saveAWGPing(db, peer.Email, &model.AWGPingPolicy{}); err != nil {
					t.Fatal(err)
				}
			case "empty-allow":
				if err := saveAWGPing(db, peer.Email, &model.AWGPingPolicy{Enabled: true}); err != nil {
					t.Fatal(err)
				}
			case "deny":
				dst = netip.MustParseAddr("198.51.100.8")
			case "disabled-client":
				apply(db.Model(record).UpdateColumn("enable", false))
			case "expired-client":
				apply(db.Model(record).UpdateColumn("expiry_time", time.Now().UnixMilli()))
			case "expired-traffic":
				apply(db.Model(&xray.ClientTraffic{}).Where("email = ?", peer.Email).UpdateColumn("expiry_time", time.Now().UnixMilli()))
			case "quota":
				apply(db.Model(&xray.ClientTraffic{}).Where("email = ?", peer.Email).Updates(map[string]any{"total": 59}))
			case "record-quota":
				apply(db.Model(record).UpdateColumn("total_gb", 59))
			case "disabled-inbound":
				apply(db.Model(ib).UpdateColumn("enable", false))
			case "inbound-quota":
				apply(db.Model(ib).UpdateColumn("total", 59))
			case "unknown-peer":
				peer.PublicKey = "unknown"
			case "unknown-source":
				src = netip.MustParseAddr("192.0.2.3")
			case "other-peer-target":
				dst = netip.MustParseAddr("192.0.2.2")
			case "deleted":
				apply(db.Delete(record))
			case "missing-traffic":
				apply(db.Where("email = ?", peer.Email).Delete(&xray.ClientTraffic{}))
			case "kernel":
				var settings amneziawg.InboundSettings
				_ = json.Unmarshal([]byte(ib.Settings), &settings)
				settings.Server.KernelMode = true
				b, _ := json.Marshal(settings)
				apply(db.Model(ib).UpdateColumn("settings", string(b)))
			}
			if _, err := (&InboundService{}).RecordAWGPing(ib.Id, peer, src, dst, 60, 0, ""); err == nil {
				t.Fatal("denied state accepted")
			}
			var stored model.Inbound
			apply(db.First(&stored, ib.Id))
			if stored.Up != 0 || stored.Down != 0 {
				t.Fatal("denied state charged inbound")
			}
			var traffic xray.ClientTraffic
			if db.Where("email = ?", peer.Email).Take(&traffic).Error == nil && (traffic.Up != 0 || traffic.Down != 0) {
				t.Fatal("denied state charged client")
			}
		})
	}
}

func TestAWGPingRevocationAndWriteFailureAreClosed(t *testing.T) {
	ib, peer, _ := seedAWGPing(t)
	svc := &InboundService{}
	db := database.GetDB()
	token, err := pingGate(svc, ib, peer, 60, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := saveAWGPing(db, peer.Email, &model.AWGPingPolicy{Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := pingGate(svc, ib, peer, 0, 60, token); err == nil {
		t.Fatal("reply survived revocation")
	}
	if err := saveAWGPing(db, peer.Email, &model.AWGPingPolicy{Enabled: true, Allow: []string{"198.51.100.0/24"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := pingGate(svc, ib, peer, 0, 60, token); err == nil {
		t.Fatal("reply survived policy revoke and re-enable")
	}
	if err := db.Callback().Update().Before("gorm:update").Register("synthetic-echo-write-failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "client_traffics" {
			tx.AddError(errors.New("synthetic accounting failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Update().Remove("synthetic-echo-write-failure") })
	// The native counter writer uses raw SQL; reject that operation as well.
	if err := db.Callback().Raw().Before("gorm:raw").Register("synthetic-echo-raw-failure", func(tx *gorm.DB) { tx.AddError(errors.New("synthetic accounting failure")) }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Raw().Remove("synthetic-echo-raw-failure") })
	if _, err := pingGate(svc, ib, peer, 60, 0, ""); err == nil {
		t.Fatal("accounting failure accepted")
	}
	var stored model.Inbound
	if err := db.First(&stored, ib.Id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Up != 60 || stored.Down != 0 {
		t.Fatalf("failed transaction changed counters: %d/%d", stored.Up, stored.Down)
	}
}

func TestAWGPingClientUpdatePreservesOmittedAndClearsExplicit(t *testing.T) {
	ib, peer, record := seedAWGPing(t)
	useTestRuntimeManager(t).SetLocalRuntimeOverride(&fakeNodeRuntime{})
	svc := &ClientService{}
	inboundSvc := &InboundService{}
	update := *record.ToClient()
	update.AWGPing = nil
	update.Comment = "metadata only"
	if _, err := svc.Update(inboundSvc, record.Id, update, 0); err != nil {
		t.Fatal(err)
	}
	if !lookupClientRecord(t, peer.Email).AWGPing.Enabled {
		t.Fatal("omitted policy erased")
	}
	update.AWGPing = &model.AWGPingPolicy{Enabled: true, Allow: []string{}, Deny: []string{}}
	if _, err := svc.Update(inboundSvc, record.Id, update, 0); err != nil {
		t.Fatal(err)
	}
	got := lookupClientRecord(t, peer.Email)
	if got.AWGPing == nil || !got.AWGPing.Enabled || len(got.AWGPing.Allow) != 0 {
		t.Fatalf("explicit clear ignored: %#v", got.AWGPing)
	}
	if _, err := pingGate(inboundSvc, ib, peer, 60, 0, ""); err == nil {
		t.Fatal("empty allow retained access")
	}
}

func TestAWGPingKernelChangeCannotHidePolicyInOmittedMirror(t *testing.T) {
	ib, _, _ := seedAWGPing(t)
	var settings amneziawg.InboundSettings
	if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
		t.Fatal(err)
	}
	settings.Server.KernelMode = true
	for i := range settings.Clients {
		settings.Clients[i].AWGPing = nil
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	updated := *ib
	updated.Settings = string(raw)
	if err := (&InboundService{}).validateUpdatedInboundClients(&updated, ib); err == nil {
		t.Fatal("kernel mode bypassed canonical ping policy")
	}
}

func TestAWGPingHonorsFreshCrossPanelQuotaAndIgnoresStaleSnapshot(t *testing.T) {
	for _, stale := range []bool{false, true} {
		t.Run(fmt.Sprint(stale), func(t *testing.T) {
			ib, peer, _ := seedAWGPing(t)
			db := database.GetDB()
			if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", peer.Email).UpdateColumn("total", 100).Error; err != nil {
				t.Fatal(err)
			}
			at := time.Now()
			if stale {
				at = at.Add(-globalTrafficFreshWindow - time.Second)
			}
			if err := db.Create(&model.ClientGlobalTraffic{Email: peer.Email, MasterGuid: "synthetic-master", Up: 80, Down: 20, UpdatedAt: at.UnixMilli()}).Error; err != nil {
				t.Fatal(err)
			}
			_, err := pingGate(&InboundService{}, ib, peer, 60, 0, "")
			if (err == nil) != stale {
				t.Fatalf("stale=%v error=%v", stale, err)
			}
		})
	}
}

func TestAWGPingDelayedExpiryStartsOnFirstAdmittedProbe(t *testing.T) {
	ib, peer, _ := seedAWGPing(t)
	db := database.GetDB()
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", peer.Email).UpdateColumn("expiry_time", -60000).Error; err != nil {
		t.Fatal(err)
	}
	before := time.Now().UnixMilli()
	identity, err := pingGate(&InboundService{}, ib, peer, 60, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pingGate(&InboundService{}, ib, peer, 0, 60, identity); err != nil {
		t.Fatal(err)
	}
	var traffic xray.ClientTraffic
	if err := db.Where("email = ?", peer.Email).Take(&traffic).Error; err != nil {
		t.Fatal(err)
	}
	if traffic.ExpiryTime < before+60000 || traffic.ExpiryTime > time.Now().UnixMilli()+60000 {
		t.Fatalf("delayed expiry not activated: %d", traffic.ExpiryTime)
	}
}

func TestAWGPingDeletedAndReusedEmailCannotReceiveOldReply(t *testing.T) {
	ib, peer, record := seedAWGPing(t)
	svc := &InboundService{}
	db := database.GetDB()
	token, err := pingGate(svc, ib, peer, 60, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Where("client_id = ?", record.Id).Delete(&model.ClientInbound{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(record).Error; err != nil {
		t.Fatal(err)
	}
	newer := &model.ClientRecord{Email: record.Email, PublicKey: record.PublicKey, Enable: true, AWGPing: record.AWGPing.Clone()}
	if err := db.Create(newer).Error; err != nil {
		t.Fatal(err)
	}
	if newer.Id == record.Id {
		t.Fatal("fixture reused primary key")
	}
	if err := db.Create(&model.ClientInbound{ClientId: newer.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := pingGate(svc, ib, peer, 0, 60, token); err == nil {
		t.Fatal("old reply reached reused account")
	}
}

func TestAWGPingParallelAccountingHasNoLostOrDuplicatedBytes(t *testing.T) {
	ib, peer, _ := seedAWGPing(t)
	StartTrafficWriter()
	defer StopTrafficWriter()
	const count = 200
	errors := make(chan error, count)
	var workers sync.WaitGroup
	for i := 0; i < count; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			identity, err := pingGate(&InboundService{}, ib, peer, 60, 0, "")
			if err == nil {
				_, err = pingGate(&InboundService{}, ib, peer, 0, 60, identity)
			}
			errors <- err
		}()
	}
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var traffic xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", peer.Email).Take(&traffic).Error; err != nil {
		t.Fatal(err)
	}
	if traffic.Up != count*60 || traffic.Down != count*60 {
		t.Fatalf("parallel counters %d/%d, want %d/%d", traffic.Up, traffic.Down, count*60, count*60)
	}
	// Reopening the writer must not replay any prior packet's counters.
	StopTrafficWriter()
	StartTrafficWriter()
	var stored model.Inbound
	if err := database.GetDB().First(&stored, ib.Id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Up != count*60 || stored.Down != count*60 {
		t.Fatal("native inbound counters drifted")
	}
}
