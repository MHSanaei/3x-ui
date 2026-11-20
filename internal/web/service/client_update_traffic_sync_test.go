package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// A client whose last inbound was detached keeps its enforcement row
// (client_traffics) for the auto-disable loop (#7160): editing it must sync
// the row's lifecycle fields (expiry/total/reset), not just clients, or the
// expiry mechanism keeps acting on the stale date.
func TestUpdate_NoInbound_SyncsClientTrafficLifecycle(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}

	email := "noib-trafficsync@x"
	rec := &model.ClientRecord{
		Email:      email,
		UUID:       "11111111-1111-1111-1111-111111111111",
		SubID:      email,
		TotalGB:    5,
		ExpiryTime: 1000,
		Reset:      1,
	}
	if err := database.GetDB().Create(rec).Error; err != nil {
		t.Fatalf("create record: %v", err)
	}
	// The enforcement row the traffic poll created while the client was still
	// attached, with real usage counters that must survive the edit.
	if err := database.GetDB().Create(&xray.ClientTraffic{
		InboundId:  999,
		Email:      email,
		Enable:     true,
		Up:         123,
		Down:       456,
		ExpiryTime: 1000,
		Total:      5,
		Reset:      1,
	}).Error; err != nil {
		t.Fatalf("create client_traffics row: %v", err)
	}

	updated := rec.ToClient()
	updated.ExpiryTime = 1700000000
	updated.TotalGB = 10
	updated.Reset = 30
	if _, err := svc.Update(nil, rec.Id, *updated, rec.LimitHwid); err != nil {
		t.Fatalf("Update: %v", err)
	}

	var stat xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", email).First(&stat).Error; err != nil {
		t.Fatalf("read client_traffics: %v", err)
	}
	if stat.ExpiryTime != 1700000000 {
		t.Fatalf("client_traffics.expiry_time not synced, got %d, want 1700000000", stat.ExpiryTime)
	}
	if stat.Total != 10 {
		t.Fatalf("client_traffics.total not synced, got %d, want 10", stat.Total)
	}
	if stat.Reset != 30 {
		t.Fatalf("client_traffics.reset not synced, got %d, want 30", stat.Reset)
	}
	if stat.Up != 123 || stat.Down != 456 {
		t.Fatalf("usage counters clobbered, got up=%d down=%d, want 123/456", stat.Up, stat.Down)
	}
}

// A client the quota loop switched off stays off when the edit does not clear
// the depletion; a re-enable must still reach the enforcement row (existing
// branch) and a disable must switch it off there too.
func TestUpdate_NoInbound_SyncsClientTrafficEnable(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}

	email := "noib-enablesync@x"
	rec := &model.ClientRecord{
		Email:      email,
		UUID:       "11111111-1111-1111-1111-111111111111",
		SubID:      email,
		ExpiryTime: 1700000000,
		Enable:     true,
	}
	if err := database.GetDB().Create(rec).Error; err != nil {
		t.Fatalf("create record: %v", err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{
		InboundId:  999,
		Email:      email,
		Enable:     true,
		ExpiryTime: 1700000000,
	}).Error; err != nil {
		t.Fatalf("create client_traffics row: %v", err)
	}

	updated := rec.ToClient()
	updated.Enable = false
	if _, err := svc.Update(nil, rec.Id, *updated, rec.LimitHwid); err != nil {
		t.Fatalf("Update: %v", err)
	}

	var stat xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", email).First(&stat).Error; err != nil {
		t.Fatalf("read client_traffics: %v", err)
	}
	if stat.Enable {
		t.Fatalf("client_traffics.enable not synced to false on disable")
	}
	if stat.ExpiryTime != 1700000000 {
		t.Fatalf("client_traffics.expiry_time clobbered, got %d", stat.ExpiryTime)
	}
}
