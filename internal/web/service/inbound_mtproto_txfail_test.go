package service

import (
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

func TestUpdateInboundLocalMtprotoDefersPushUntilCommit(t *testing.T) {
	setupConflictDB(t)

	mgr := runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }})
	fake := &fakeNodeRuntime{}
	mgr.SetLocalRuntimeOverride(fake)
	runtime.SetManager(mgr)
	t.Cleanup(func() { runtime.SetManager(nil) })

	seedInboundConflict(t, "mt-txfail", "", 46150, model.MTProto, "",
		`{"clients":[{"email":"mtx","secret":"`+mtprotoTestSecretA+`","enable":true}]}`)
	seeded := loadInboundByTag(t, "mt-txfail")
	seedClientTraffic(t, seeded.Id, "mtx", true)

	db := database.GetDB()
	const cbName = "b1-05:fail-inbound-update"
	if err := db.Callback().Update().After("gorm:update").Register(cbName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "inbounds" {
			tx.AddError(errors.New("injected transaction failure"))
		}
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}
	t.Cleanup(func() { _ = db.Callback().Update().Remove(cbName) })

	update := *loadInboundByTag(t, "mt-txfail")
	update.Remark = "edited"
	if _, _, err := (&InboundService{}).UpdateInbound(&update); err == nil {
		t.Fatal("UpdateInbound: expected the injected transaction failure")
	}

	if n := fake.updateInbound.Load(); n != 0 {
		t.Fatalf("the MTProto sidecar push ran %d time(s) inside the failed transaction; it must be deferred until the commit succeeds", n)
	}
}

func TestSetInboundEnableRoutedMtprotoRequestsRestart(t *testing.T) {
	setupConflictDB(t)

	mgr := runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }})
	mgr.SetLocalRuntimeOverride(&fakeNodeRuntime{})
	runtime.SetManager(mgr)
	t.Cleanup(func() { runtime.SetManager(nil) })

	seedInboundConflict(t, "mt-route", "", 46160, model.MTProto, "",
		`{"clients":[{"email":"mtr","secret":"`+mtprotoTestSecretA+`","enable":true}],"routeThroughXray":true,"routeXrayPort":12345}`)
	seeded := loadInboundByTag(t, "mt-route")
	if err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", seeded.Id).Update("enable", false).Error; err != nil {
		t.Fatalf("force disable: %v", err)
	}

	needRestart, err := (&InboundService{}).SetInboundEnable(seeded.Id, true)
	if err != nil {
		t.Fatalf("SetInboundEnable: %v", err)
	}
	if !needRestart {
		t.Fatal("re-enabling a routed MTProto inbound must request an xray restart to re-inject the egress bridge")
	}
}
