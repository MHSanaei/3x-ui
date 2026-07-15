package service

import (
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// The traffic tick stages inbound and client deltas, then runs three best-effort
// maintenance helpers (renew, disable-depleted-clients, disable-depleted-inbounds)
// that are meant to log and continue. A failure in one of them must not roll back
// the already-staged traffic — xray has already advanced its baseline, so a
// rolled-back tick loses that traffic permanently.
func TestAddTrafficCommitsDespiteDisableHelperError(t *testing.T) {
	db := initTrafficTestDB(t)
	svc := &InboundService{}

	normal := &model.Inbound{UserId: 1, Tag: "in-normal", Enable: true, Port: 43001, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := db.Create(normal).Error; err != nil {
		t.Fatalf("seed normal inbound: %v", err)
	}
	expired := &model.Inbound{UserId: 1, Tag: "in-expired", Enable: true, Port: 43002, Protocol: model.VLESS, ExpiryTime: 1, Settings: `{"clients":[]}`}
	if err := db.Create(expired).Error; err != nil {
		t.Fatalf("seed expired inbound: %v", err)
	}

	const cbName = "b2-03:fail-disable"
	if err := db.Callback().Update().After("gorm:update").Register(cbName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "inbounds" &&
			strings.Contains(tx.Statement.SQL.String(), "expiry_time") {
			tx.AddError(errors.New("injected disableInvalidInbounds failure"))
		}
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}
	t.Cleanup(func() { _ = db.Callback().Update().Remove(cbName) })

	if _, _, err := svc.AddTraffic([]*xray.Traffic{{IsInbound: true, Tag: "in-normal", Up: 500, Down: 700}}, nil); err != nil {
		t.Fatalf("AddTraffic: %v", err)
	}

	var reloaded model.Inbound
	if err := db.Where("tag = ?", "in-normal").First(&reloaded).Error; err != nil {
		t.Fatalf("reload normal inbound: %v", err)
	}
	if reloaded.Up != 500 || reloaded.Down != 700 {
		t.Fatalf("traffic tick was rolled back by a best-effort disable-helper error: up=%d down=%d, want 500/700", reloaded.Up, reloaded.Down)
	}
}
