package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

// setupQuotaRestoreTest wires a temp DB and a local runtime that accepts
// inbound changes, so a reset can be driven end to end.
func setupQuotaRestoreTest(t *testing.T) {
	t.Helper()
	initTrafficTestDB(t)
	resetTrafficWriterForTest(t)

	mgr := runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }})
	mgr.SetLocalRuntimeOverride(&fakeNodeRuntime{})
	runtime.SetManager(mgr)
	t.Cleanup(func() { runtime.SetManager(nil) })
}

// seedQuotaInbound creates an inbound in the state a traffic cap leaves it in:
// switched off, with its counters past the cap.
func seedQuotaInbound(t *testing.T, tag string, port int, total, up int64, expiryTime int64) *model.Inbound {
	t.Helper()
	ib := &model.Inbound{
		UserId:         1,
		Tag:            tag,
		Enable:         false,
		Listen:         "0.0.0.0",
		Port:           port,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp"}`,
		Settings:       `{}`,
		Total:          total,
		Up:             up,
		ExpiryTime:     expiryTime,
	}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("seed inbound %s: %v", tag, err)
	}
	return ib
}

// A traffic cap switches an inbound off and drops it from the running core.
// Zeroing the counters lifts the cap, so the inbound has to come back — an
// inbound left off takes every client on it down with it.
func TestResetAllTrafficsRestoresQuotaDisabledInbound(t *testing.T) {
	setupQuotaRestoreTest(t)
	const gb = int64(1024 * 1024 * 1024)
	seedQuotaInbound(t, "capped", 45501, gb, gb, 0)

	if _, err := (&InboundService{}).ResetAllTraffics(); err != nil {
		t.Fatalf("ResetAllTraffics: %v", err)
	}

	after := loadInboundByTag(t, "capped")
	if !after.Enable {
		t.Fatal("an inbound whose traffic cap was lifted must be enabled again")
	}
	if after.Up != 0 || after.Down != 0 {
		t.Fatalf("counters = up %d / down %d, want both zero", after.Up, after.Down)
	}
}

func TestResetInboundTrafficRestoresQuotaDisabledInbound(t *testing.T) {
	setupQuotaRestoreTest(t)
	const gb = int64(1024 * 1024 * 1024)
	ib := seedQuotaInbound(t, "capped", 45502, gb, gb, 0)

	if _, err := (&InboundService{}).ResetInboundTraffic(ib.Id); err != nil {
		t.Fatalf("ResetInboundTraffic: %v", err)
	}

	after := loadInboundByTag(t, "capped")
	if !after.Enable {
		t.Fatal("an inbound whose traffic cap was lifted must be enabled again")
	}
	if after.Up != 0 || after.Down != 0 {
		t.Fatalf("counters = up %d / down %d, want both zero", after.Up, after.Down)
	}
}

// Only a cap may bring an inbound back. One an operator switched off by hand,
// and one that expired, stay off — the reset has nothing to do with either.
func TestResetAllTrafficsLeavesHandDisabledAndExpiredInboundsOff(t *testing.T) {
	setupQuotaRestoreTest(t)
	const gb = int64(1024 * 1024 * 1024)
	seedQuotaInbound(t, "hand-disabled", 45503, 0, 0, 0)
	seedQuotaInbound(t, "expired", 45504, gb, gb, time.Now().Add(-time.Hour).UnixMilli())

	if _, err := (&InboundService{}).ResetAllTraffics(); err != nil {
		t.Fatalf("ResetAllTraffics: %v", err)
	}

	for _, tag := range []string{"hand-disabled", "expired"} {
		if after := loadInboundByTag(t, tag); after.Enable {
			t.Fatalf("inbound %q must stay disabled", tag)
		}
	}
}
