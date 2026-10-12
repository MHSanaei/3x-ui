package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func audit3CreateTuicClient(t *testing.T, svc *ClientService, inboundSvc *InboundService, ib *model.Inbound, email, id string) model.ClientRecord {
	t.Helper()
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{Client: model.Client{Email: email, ID: id, Password: "pw", Enable: true}, InboundIds: []int{ib.Id}}); err != nil {
		t.Fatalf("public Create %s: %v", email, err)
	}
	return lookupClientRecord(t, email)
}

func audit3Traffic(t *testing.T, email string) xray.ClientTraffic {
	t.Helper()
	var out xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", email).First(&out).Error; err != nil {
		t.Fatal(err)
	}
	return out
}

func TestAudit3DeletedIdentityMustNotChargeDuplicateUUID(t *testing.T) {
	setupBulkDB(t)
	svc, inboundSvc := &ClientService{}, &InboundService{}
	ibB := mkInbound(t, 28001, model.TUIC, `{"clients":[]}`)
	ibA := mkInbound(t, 28002, model.TUIC, `{"clients":[]}`)
	const id = "a0000000-0000-0000-0000-000000000031"
	audit3CreateTuicClient(t, svc, inboundSvc, ibB, "b@audit3", id)
	a := audit3CreateTuicClient(t, svc, inboundSvc, ibA, "a@audit3", id)
	trafficID := audit3RuntimeTrafficID(t, inboundSvc, ibA, "a@audit3")
	if _, err := svc.Delete(inboundSvc, a.Id, true); err != nil {
		t.Fatalf("public Delete keepTraffic: %v", err)
	}
	if _, _, err := inboundSvc.AddTraffic(nil, []*xray.ClientTraffic{{Email: "a@audit3", TuicTrafficID: trafficID, TuicUUID: id, TuicInboundId: ibA.Id, Up: 123, Down: 456}}); err != nil {
		t.Fatal(err)
	}
	old, other := audit3Traffic(t, "a@audit3"), audit3Traffic(t, "b@audit3")
	if old.Up != 123 || old.Down != 456 || other.Up != 0 || other.Down != 0 {
		t.Fatalf("retired A=%d/%d; unrelated B=%d/%d, want A=123/456 and B=0/0", old.Up, old.Down, other.Up, other.Down)
	}
}

func TestAudit3RenameAndRotateCredentialsMustPreserveRetiredUsage(t *testing.T) {
	setupBulkDB(t)
	svc, inboundSvc := &ClientService{}, &InboundService{}
	ib := mkInbound(t, 28003, model.TUIC, `{"clients":[]}`)
	const oldID = "a0000000-0000-0000-0000-000000000032"
	a := audit3CreateTuicClient(t, svc, inboundSvc, ib, "old@audit3", oldID)
	trafficID := audit3RuntimeTrafficID(t, inboundSvc, ib, "old@audit3")
	if _, err := svc.Update(inboundSvc, a.Id, model.Client{Email: "new@audit3", ID: "a0000000-0000-0000-0000-000000000033", Password: "newpw", Enable: true}, 0); err != nil {
		t.Fatalf("public rename + rotate: %v", err)
	}
	if _, _, err := inboundSvc.AddTraffic(nil, []*xray.ClientTraffic{{Email: "old@audit3", TuicTrafficID: trafficID, TuicUUID: oldID, TuicInboundId: ib.Id, Up: 123, Down: 456}}); err != nil {
		t.Fatal(err)
	}
	got := audit3Traffic(t, "new@audit3")
	if got.Up != 123 || got.Down != 456 {
		t.Fatalf("new account usage=%d/%d, want retired 123/456", got.Up, got.Down)
	}
}

func TestAudit3PublicRenamePreservesRetiredUsage(t *testing.T) {
	setupBulkDB(t)
	svc, inboundSvc := &ClientService{}, &InboundService{}
	ib := mkInbound(t, 28004, model.TUIC, `{"clients":[]}`)
	const id = "a0000000-0000-0000-0000-000000000034"
	a := audit3CreateTuicClient(t, svc, inboundSvc, ib, "old@audit3", id)
	trafficID := audit3RuntimeTrafficID(t, inboundSvc, ib, "old@audit3")
	if _, err := svc.Update(inboundSvc, a.Id, model.Client{Email: "new@audit3", ID: id, Password: "pw", Enable: true}, 0); err != nil {
		t.Fatalf("public rename: %v", err)
	}
	if _, _, err := inboundSvc.AddTraffic(nil, []*xray.ClientTraffic{{Email: "old@audit3", TuicTrafficID: trafficID, TuicUUID: id, TuicInboundId: ib.Id, Up: 123, Down: 456}}); err != nil {
		t.Fatal(err)
	}
	got := audit3Traffic(t, "new@audit3")
	if got.Up != 123 || got.Down != 456 {
		t.Fatalf("new account usage=%d/%d, want retired 123/456", got.Up, got.Down)
	}
}

func TestAudit3SameInboundDuplicateUUIDMustBeRejectedOrBillMatchedPrincipal(t *testing.T) {
	setupBulkDB(t)
	svc, inboundSvc := &ClientService{}, &InboundService{}
	ib := mkInbound(t, 28005, model.TUIC, `{"clients":[]}`)
	const id = "a0000000-0000-0000-0000-000000000035"
	audit3CreateTuicClient(t, svc, inboundSvc, ib, "a@audit3", id)
	if _, err := svc.Create(inboundSvc, &ClientCreatePayload{Client: model.Client{Email: "b@audit3", ID: id, Password: "different-pw", Enable: true}, InboundIds: []int{ib.Id}}); err != nil {
		t.Logf("duplicate correctly rejected: %v", err)
		return
	}
	t.Fatal("public Create accepted duplicate UUID")
}

func audit3RuntimeTrafficID(t *testing.T, service *InboundService, inbound *model.Inbound, email string) int {
	t.Helper()
	fresh, err := service.GetInbound(inbound.Id)
	if err != nil {
		t.Fatal(err)
	}
	built, err := service.buildInboundForLocalRuntime(database.GetDB(), fresh)
	if err != nil {
		t.Fatal(err)
	}
	instance, ok := tuic.InstanceFromInbound(built)
	if !ok {
		t.Fatal("invalid runtime settings")
	}
	for _, client := range instance.Clients {
		if client.Email == email {
			if client.TrafficID == 0 || client.TrafficID != audit3Traffic(t, email).Id {
				t.Fatal("runtime snapshot lacks immutable accounting identity")
			}
			return client.TrafficID
		}
	}
	t.Fatal("runtime client missing")
	return 0
}

func TestTuicDeletedTrafficRowCannotBillReusedEmail(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	old := xray.ClientTraffic{Email: "reused@audit3", Enable: true}
	if err := db.Create(&old).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&old).Error; err != nil {
		t.Fatal(err)
	}
	replacement := xray.ClientTraffic{Email: old.Email, Enable: true}
	if err := db.Create(&replacement).Error; err != nil {
		t.Fatal(err)
	}
	if old.Id == replacement.Id {
		t.Fatal("accounting primary key reused")
	}
	if _, _, err := (&InboundService{}).AddTraffic(nil, []*xray.ClientTraffic{{Email: old.Email, TuicTrafficID: old.Id, Up: 123, Down: 456}}); err != nil {
		t.Fatal(err)
	}
	got := audit3Traffic(t, old.Email)
	if got.Up != 0 || got.Down != 0 {
		t.Fatalf("retired client billed replacement: %+v", got)
	}
}
