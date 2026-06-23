package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// TestTrafficMultiplierAccrual pins the worked example from the design: a client
// on a 2x and a 0.5x inbound that moves 50 GB and 30 GB respectively is billed
// 50*2 + 30*0.5 = 115 GB while Real stays 80 GB, and the per-attachment breakdown
// adds up. It also pins non-retroactive billing (ADR 0002): raising a multiplier
// mid-cycle bills only new traffic at the new rate.
func TestTrafficMultiplierAccrual(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	db := database.GetDB()
	const gb = int64(1024 * 1024 * 1024)

	in2x := &model.Inbound{UserId: 1, Tag: "in-2x", Protocol: model.VLESS, Port: 1001, Multiplier: 2, Enable: true}
	inHalf := &model.Inbound{UserId: 1, Tag: "in-half", Protocol: model.VLESS, Port: 1002, Multiplier: 0.5, Enable: true}
	if err := db.Create(in2x).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(inHalf).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&xray.ClientTraffic{Email: "alice", Enable: true, Total: 200 * gb}).Error; err != nil {
		t.Fatal(err)
	}

	svc := &InboundService{}

	assertClient := func(wantReal, wantBilled int64) {
		t.Helper()
		var ct xray.ClientTraffic
		if err := db.Where("email = ?", "alice").First(&ct).Error; err != nil {
			t.Fatal(err)
		}
		if got := ct.Up + ct.Down; got != wantReal {
			t.Errorf("client Real = %d GB, want %d", got/gb, wantReal/gb)
		}
		if got := ct.BilledUp + ct.BilledDown; got != wantBilled {
			t.Errorf("client Billed = %d GB, want %d", got/gb, wantBilled/gb)
		}
	}
	assertAttachment := func(inboundId int, wantReal, wantBilled int64) {
		t.Helper()
		var a model.ClientInboundTraffic
		if err := db.Where("inbound_id = ? AND email = ?", inboundId, "alice").First(&a).Error; err != nil {
			t.Fatalf("attachment %d: %v", inboundId, err)
		}
		if got := a.Up + a.Down; got != wantReal {
			t.Errorf("attachment %d Real = %d GB, want %d", inboundId, got/gb, wantReal/gb)
		}
		if got := a.BilledUp + a.BilledDown; got != wantBilled {
			t.Errorf("attachment %d Billed = %d GB, want %d", inboundId, got/gb, wantBilled/gb)
		}
	}

	// First poll: 50 GB on the 2x inbound, 30 GB on the 0.5x inbound.
	if _, _, err := svc.AddTraffic(nil, []*xray.ClientTraffic{
		{InboundId: in2x.Id, Email: "alice", Up: 50 * gb},
		{InboundId: inHalf.Id, Email: "alice", Up: 30 * gb},
	}); err != nil {
		t.Fatalf("AddTraffic: %v", err)
	}
	assertClient(80*gb, 115*gb)
	assertAttachment(in2x.Id, 50*gb, 100*gb)
	assertAttachment(inHalf.Id, 30*gb, 15*gb)

	// Second poll on the same inbounds accumulates (upsert adds the delta).
	if _, _, err := svc.AddTraffic(nil, []*xray.ClientTraffic{
		{InboundId: in2x.Id, Email: "alice", Down: 10 * gb},
	}); err != nil {
		t.Fatalf("AddTraffic 2: %v", err)
	}
	assertClient(90*gb, 135*gb) // Real 80+10; Billed 115 + 10*2
	assertAttachment(in2x.Id, 60*gb, 120*gb)

	// Non-retroactive: raise the 2x inbound to 3x, then 10 GB more flows on it.
	// Past usage keeps the 2x rate; only the new 10 GB bills at 3x.
	if err := db.Model(&model.Inbound{}).Where("id = ?", in2x.Id).Update("multiplier", float64(3)).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.AddTraffic(nil, []*xray.ClientTraffic{
		{InboundId: in2x.Id, Email: "alice", Up: 10 * gb},
	}); err != nil {
		t.Fatalf("AddTraffic 3: %v", err)
	}
	assertClient(100*gb, 165*gb) // Real 90+10; Billed 135 + 10*3 (not 10*... retroactive)
	assertAttachment(in2x.Id, 70*gb, 150*gb)
}
