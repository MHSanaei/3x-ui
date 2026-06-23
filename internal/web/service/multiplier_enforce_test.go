package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// TestBilledQuotaEnforcement verifies depletion is decided on Billed, not Real:
// on a 2x inbound a 100 GB quota is exhausted by 50 GB of Real traffic.
func TestBilledQuotaEnforcement(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	db := database.GetDB()
	const gb = int64(1024 * 1024 * 1024)

	in := &model.Inbound{UserId: 1, Tag: "in-bill", Protocol: model.VLESS, Port: 1003, Multiplier: 2, Enable: true}
	if err := db.Create(in).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&xray.ClientTraffic{Email: "bob", Enable: true, Total: 100 * gb}).Error; err != nil {
		t.Fatal(err)
	}

	svc := &InboundService{}
	enabled := func() bool {
		var ct xray.ClientTraffic
		if err := db.Where("email = ?", "bob").First(&ct).Error; err != nil {
			t.Fatal(err)
		}
		return ct.Enable
	}

	// 40 GB Real -> 80 GB Billed < 100 GB quota: still active.
	_, disabled, err := svc.AddTraffic(nil, []*xray.ClientTraffic{{InboundId: in.Id, Email: "bob", Up: 40 * gb}})
	if err != nil {
		t.Fatalf("AddTraffic: %v", err)
	}
	if disabled {
		t.Error("disabled at 80 GB Billed (quota 100), want active")
	}
	if !enabled() {
		t.Error("enable=false at 80 GB Billed, want true")
	}

	// +10 GB Real -> 50 GB Real, 100 GB Billed >= 100 GB quota: depleted on Billed.
	_, disabled, err = svc.AddTraffic(nil, []*xray.ClientTraffic{{InboundId: in.Id, Email: "bob", Down: 10 * gb}})
	if err != nil {
		t.Fatalf("AddTraffic 2: %v", err)
	}
	if !disabled {
		t.Error("not disabled at 100 GB Billed (quota 100), want disabled")
	}
	if enabled() {
		t.Error("enable=true after Billed depletion, want false")
	}
}

// TestResetClearsBilledAndAttachments verifies a traffic reset zeroes Billed (so
// the client starts a fresh cycle) and drops the per-attachment breakdown rows.
func TestResetClearsBilledAndAttachments(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	db := database.GetDB()
	const gb = int64(1024 * 1024 * 1024)

	in := &model.Inbound{UserId: 1, Tag: "in-reset", Protocol: model.VLESS, Port: 1004, Multiplier: 2, Enable: true}
	if err := db.Create(in).Error; err != nil {
		t.Fatal(err)
	}
	// A clients record so ResetClientTraffic can resolve the client.
	if err := db.Create(&model.ClientRecord{Email: "carol", Enable: true, TotalGB: 1000 * gb}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&xray.ClientTraffic{Email: "carol", Enable: true, Total: 1000 * gb}).Error; err != nil {
		t.Fatal(err)
	}

	svc := &InboundService{}
	if _, _, err := svc.AddTraffic(nil, []*xray.ClientTraffic{{InboundId: in.Id, Email: "carol", Up: 30 * gb}}); err != nil {
		t.Fatalf("AddTraffic: %v", err)
	}

	var ct xray.ClientTraffic
	if err := db.Where("email = ?", "carol").First(&ct).Error; err != nil {
		t.Fatal(err)
	}
	if ct.BilledUp+ct.BilledDown == 0 {
		t.Fatal("precondition: expected Billed > 0 before reset")
	}
	var n int64
	db.Model(&model.ClientInboundTraffic{}).Where("email = ?", "carol").Count(&n)
	if n == 0 {
		t.Fatal("precondition: expected an attachment row before reset")
	}

	if _, err := svc.ResetClientTraffic(in.Id, "carol"); err != nil {
		t.Fatalf("ResetClientTraffic: %v", err)
	}

	if err := db.Where("email = ?", "carol").First(&ct).Error; err != nil {
		t.Fatal(err)
	}
	if ct.Up+ct.Down != 0 {
		t.Errorf("Real not zeroed after reset: %d", ct.Up+ct.Down)
	}
	if ct.BilledUp+ct.BilledDown != 0 {
		t.Errorf("Billed not zeroed after reset: %d", ct.BilledUp+ct.BilledDown)
	}
	db.Model(&model.ClientInboundTraffic{}).Where("email = ?", "carol").Count(&n)
	if n != 0 {
		t.Errorf("attachment rows not cleared after reset: %d", n)
	}
}
