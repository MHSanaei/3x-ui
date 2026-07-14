package service

import (
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

// A local MTProto inbound edit must not push to the managed sidecar from inside
// the serialized write transaction: that blocks the single traffic-writer
// goroutine on process/network I/O, and a later step failing the transaction
// would leave the sidecar ahead of the rolled-back database. The push belongs in
// the post-commit hook, exactly as the xray branch already does it.
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
