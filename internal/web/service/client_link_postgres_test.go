package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// PostgreSQL before 16 rejects a FROM subquery without an alias, so the TUIC
// duplicate-UUID guard must not break client sync for every protocol there.
func TestSyncInboundClientsOnPostgres(t *testing.T) {
	db := durablePostgresDB(t)
	suffix := time.Now().UnixNano()
	inbound := durableTestInbound(nil, fmt.Sprintf("pg-sync-%d", suffix), 20000+int(suffix%20000))
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	email := fmt.Sprintf("pg-sync-%d@x", suffix)
	t.Cleanup(func() {
		_ = db.Where("email = ?", email).Delete(&xray.ClientTraffic{}).Error
		_ = db.Where("email = ?", email).Delete(&model.ClientRecord{}).Error
		_ = db.Delete(&model.Inbound{}, inbound.Id).Error
	})

	clients := []model.Client{{ID: "8a9c1b2e-1111-4c3d-9e8f-000000000001", Email: email, Enable: true}}
	if err := (&ClientService{}).SyncInbound(nil, inbound.Id, clients); err != nil {
		t.Fatalf("SyncInbound on PostgreSQL: %v", err)
	}
}
