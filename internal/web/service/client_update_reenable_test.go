package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// renewalReenableDay keeps the seeds one day past (or before) their boundary.
const renewalReenableDay = int64(24 * 60 * 60 * 1000)

// seedRenewableDepletedClient seeds a local inbound whose client the quota
// enforcement loop switched off: record off, inbound JSON off, and the
// traffic row off and depleted on the given dimension (0 = unlimited side).
func seedRenewableDepletedClient(t *testing.T, svc *ClientService, port int, email string, total, expiry, up, down int64) *model.Inbound {
	t.Helper()
	ib := seedLocalDisabledClient(t, svc, port, "", email, total, expiry, up, down)
	if got := trafficOf(t, email); got.Enable {
		t.Fatalf("seed %s: want an off traffic row, got %+v", email, got)
	}
	return ib
}

func TestUpdate_ReenablesDepletedClient_WhenExpiryExtended(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	now := time.Now().UnixMilli()
	email := "renew-exp@x"
	ib := seedRenewableDepletedClient(t, svc, 53101, email, 0, now-2*renewalReenableDay, 0, 0)

	rec, err := svc.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	updated := rec.ToClient()
	updated.ExpiryTime = now + 30*renewalReenableDay
	// The editor round-trips the switch as the stored off state; the renewal
	// must turn it back on by itself.
	if _, err := svc.Update(inboundSvc, rec.Id, *updated, 0); err != nil {
		t.Fatalf("Update: %v", err)
	}
	assertEnableEverywhere(t, svc, inboundSvc, ib.Id, email, true)
	if got := trafficOf(t, email).ExpiryTime; got != now+30*renewalReenableDay {
		t.Fatalf("%s: expiry = %d, want %d", email, got, now+30*renewalReenableDay)
	}
}

func TestUpdate_ReenablesDepletedClient_WhenTotalRaised(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	email := "renew-total@x"
	ib := seedRenewableDepletedClient(t, svc, 53102, email, 100, 0, 60, 40)

	rec, err := svc.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	updated := rec.ToClient()
	updated.TotalGB = 300
	if _, err := svc.Update(inboundSvc, rec.Id, *updated, 0); err != nil {
		t.Fatalf("Update: %v", err)
	}
	assertEnableEverywhere(t, svc, inboundSvc, ib.Id, email, true)
	if got := trafficOf(t, email).Total; got != 300 {
		t.Fatalf("%s: total = %d, want 300", email, got)
	}
}

func TestUpdate_KeepsDisabled_StillExpiredAfterUpdate(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	now := time.Now().UnixMilli()
	email := "renew-sml@x"
	ib := seedRenewableDepletedClient(t, svc, 53103, email, 0, now-10*renewalReenableDay, 0, 0)

	rec, err := svc.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	updated := rec.ToClient()
	updated.ExpiryTime = now - 5*renewalReenableDay // an extension that still lands in the past
	if _, err := svc.Update(inboundSvc, rec.Id, *updated, 0); err != nil {
		t.Fatalf("Update: %v", err)
	}
	assertEnableEverywhere(t, svc, inboundSvc, ib.Id, email, false)
}

func TestUpdate_KeepsDisabled_StillOverQuotaAfterUpdate(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	email := "renew-quota@x"
	ib := seedRenewableDepletedClient(t, svc, 53104, email, 100, 0, 60, 40)

	rec, err := svc.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	updated := rec.ToClient()
	updated.TotalGB = 80 // still below the 100 bytes of usage
	if _, err := svc.Update(inboundSvc, rec.Id, *updated, 0); err != nil {
		t.Fatalf("Update: %v", err)
	}
	assertEnableEverywhere(t, svc, inboundSvc, ib.Id, email, false)
}

func TestUpdate_KeepsDisabled_ManuallyDisabledNotDepleted(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	now := time.Now().UnixMilli()
	email := "renew-manual@x"
	ib := seedRenewableDepletedClient(t, svc, 53105, email, 0, now+30*renewalReenableDay, 0, 0)

	rec, err := svc.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	updated := rec.ToClient()
	updated.ExpiryTime = now + 60*renewalReenableDay
	if _, err := svc.Update(inboundSvc, rec.Id, *updated, 0); err != nil {
		t.Fatalf("Update: %v", err)
	}
	// The quota loop never switched this client off — it reads under quota —
	// so a manual switch-off survives a renewal.
	assertEnableEverywhere(t, svc, inboundSvc, ib.Id, email, false)
}

func TestUpdate_ReenablesDepletedClient_SkipsStillDepletedGlobalUsage(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	now := time.Now().UnixMilli()
	email := "renew-global@x"
	ib := seedRenewableDepletedClient(t, svc, 53106, email, 100, now+30*renewalReenableDay, 0, 0)
	// A fresh master push depletes the local limits even though up/down read 0.
	global := model.ClientGlobalTraffic{MasterGuid: "master-a", Email: email, Up: 90, Down: 60}
	if err := database.GetDB().Create(&global).Error; err != nil {
		t.Fatalf("seed global traffic: %v", err)
	}

	rec, err := svc.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	updated := rec.ToClient()
	updated.TotalGB = 100
	if _, err := svc.Update(inboundSvc, rec.Id, *updated, 0); err != nil {
		t.Fatalf("Update: %v", err)
	}
	assertEnableEverywhere(t, svc, inboundSvc, ib.Id, email, false)

	// Raising the quota above the pushed usage re-enables it.
	updated.TotalGB = 1000
	if _, err := svc.Update(inboundSvc, rec.Id, *updated, 0); err != nil {
		t.Fatalf("Update: %v", err)
	}
	assertEnableEverywhere(t, svc, inboundSvc, ib.Id, email, true)
}

func TestUpdate_ReenablesDepletedClient_WithoutInbound(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	now := time.Now().UnixMilli()
	email := "renew-noib@x"
	rec := &model.ClientRecord{
		Email: email,
		UUID:  "44444444-4444-4444-4444-444444444444",
		SubID: email,
	}
	if err := database.GetDB().Create(rec).Error; err != nil {
		t.Fatalf("create record: %v", err)
	}
	forceRecordDisabled(t, svc, email)
	if err := database.GetDB().Create(&xray.ClientTraffic{
		Email: email, Enable: false, ExpiryTime: now - 2*renewalReenableDay,
	}).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}

	updated := rec.ToClient()
	updated.ExpiryTime = now + 30*renewalReenableDay
	if _, err := svc.Update(inboundSvc, rec.Id, *updated, 0); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got := recordEnableOf(t, svc, email); !got {
		t.Fatalf("%s: client_records.enable = false, want true", email)
	}
	if got := trafficOf(t, email).Enable; !got {
		t.Fatalf("%s: client_traffics.enable = false, want true (no-inbound re-enable gap)", email)
	}
}

func TestResetClientExpiryTimeByEmail_ReenablesDepletedClient(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	now := time.Now().UnixMilli()
	email := "renew-setter@x"
	ib := seedRenewableDepletedClient(t, svc, 53107, email, 0, now-2*renewalReenableDay, 0, 0)

	if _, err := svc.ResetClientExpiryTimeByEmail(inboundSvc, email, now+30*renewalReenableDay); err != nil {
		t.Fatalf("ResetClientExpiryTimeByEmail: %v", err)
	}
	assertEnableEverywhere(t, svc, inboundSvc, ib.Id, email, true)
}

func TestResetClientExpiryTimeByEmail_KeepsDisabled_ManuallyDisabledNotDepleted(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	now := time.Now().UnixMilli()
	email := "renew-setter-off@x"
	ib := seedRenewableDepletedClient(t, svc, 53108, email, 0, now+30*renewalReenableDay, 0, 0)

	if _, err := svc.ResetClientExpiryTimeByEmail(inboundSvc, email, now+60*renewalReenableDay); err != nil {
		t.Fatalf("ResetClientExpiryTimeByEmail: %v", err)
	}
	assertEnableEverywhere(t, svc, inboundSvc, ib.Id, email, false)
}

func TestResetClientTrafficLimitByEmail_ReenablesDepletedClient(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	email := "renew-setter-total@x"
	ib := seedRenewableDepletedClient(t, svc, 53109, email, 100, 0, 60, 40)

	if _, err := svc.ResetClientTrafficLimitByEmail(inboundSvc, email, 1); err != nil {
		t.Fatalf("ResetClientTrafficLimitByEmail: %v", err)
	}
	assertEnableEverywhere(t, svc, inboundSvc, ib.Id, email, true)
	if got := trafficOf(t, email).Total; got != 1<<30 {
		t.Fatalf("%s: total = %d, want %d", email, got, 1<<30)
	}
}
