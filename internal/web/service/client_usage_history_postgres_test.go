package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

// The fold, the change-log cap and the reap each lean on SQL the two dialects spell
// differently: OFFSET without LIMIT, HAVING over a group, a recursive index walk.
func TestClientUsageHistoryPostgres(t *testing.T) {
	db := durablePostgresDB(t)
	suffix := time.Now().UnixNano()
	kept, gone := fmt.Sprintf("pg-hist-kept-%d@x", suffix), fmt.Sprintf("pg-hist-gone-%d@x", suffix)
	inbound := durableTestInbound(nil, fmt.Sprintf("pg-hist-%d", suffix), 20000+int(suffix%20000))
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	t.Cleanup(func() {
		for _, table := range []any{&model.ClientTrafficWindow{}, &model.ClientChangeLog{}, &xray.ClientTraffic{}, &model.ClientRecord{}} {
			_ = db.Where("email IN ?", []string{kept, gone}).Delete(table).Error
		}
		_ = db.Delete(&model.Inbound{}, inbound.Id).Error
	})
	if err := db.Create(&model.ClientRecord{Email: kept, SubID: kept, Enable: true}).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{InboundId: inbound.Id, Email: kept, Enable: true, Up: 7, Down: 9}).Error; err != nil {
		t.Fatalf("create client traffic: %v", err)
	}
	windows := make([]model.ClientTrafficWindow, 0, clientTrafficWindowKeep+2)
	for i := range clientTrafficWindowKeep + 1 {
		windows = append(windows, model.ClientTrafficWindow{Email: kept, At: int64(1000 + i), Up: 1, Down: 2, Reason: trafficWindowRenew})
	}
	windows = append(windows, model.ClientTrafficWindow{Email: gone, At: 1, Up: 5, Reason: trafficWindowReset})
	changes := make([]model.ClientChangeLog, 0, clientChangeLogKeep+1)
	for i := range clientChangeLogKeep {
		changes = append(changes, model.ClientChangeLog{Email: kept, At: int64(1000 + i), Field: "enable", NewValue: fmt.Sprint(i)})
	}
	changes = append(changes, model.ClientChangeLog{Email: gone, At: 1, Field: "enable"})
	if err := db.Create(&windows).Error; err != nil {
		t.Fatalf("seed windows: %v", err)
	}
	if err := db.Create(&changes).Error; err != nil {
		t.Fatalf("seed changes: %v", err)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := archiveClosedWindowsTx(tx, []string{kept}, trafficWindowReset); err != nil {
			return err
		}
		return logClientChangesTx(tx, []model.ClientChangeLog{{Email: kept, Field: "dataLimit", NewValue: "1"}})
	}); err != nil {
		t.Fatalf("archive and log on PostgreSQL: %v", err)
	}

	var folded []model.ClientTrafficWindow
	if err := db.Where("email = ? AND reason = ?", kept, trafficWindowEarlier).Find(&folded).Error; err != nil {
		t.Fatal(err)
	}
	var windowCount, changeCount, oldestChange int64
	db.Model(&model.ClientTrafficWindow{}).Where("email = ?", kept).Count(&windowCount)
	db.Model(&model.ClientChangeLog{}).Where("email = ?", kept).Count(&changeCount)
	db.Model(&model.ClientChangeLog{}).Where("email = ?", kept).Select("MIN(at)").Scan(&oldestChange)
	if windowCount != clientTrafficWindowKeep+1 || len(folded) != 1 || folded[0].Up != 2 || folded[0].Down != 4 {
		t.Fatalf("%d windows, folded %+v; want %d with one earlier row of 2/4", windowCount, folded, clientTrafficWindowKeep+1)
	}
	if changeCount != clientChangeLogKeep || oldestChange != 1001 {
		t.Fatalf("%d change rows, oldest at %d; want %d with the row at 1000 dropped", changeCount, oldestChange, clientChangeLogKeep)
	}

	if _, err := (&ClientService{}).ReapOrphanClientHistory(); err != nil {
		t.Fatalf("reap on PostgreSQL: %v", err)
	}
	var goneWindows, goneChanges, keptWindows int64
	db.Model(&model.ClientTrafficWindow{}).Where("email = ?", gone).Count(&goneWindows)
	db.Model(&model.ClientChangeLog{}).Where("email = ?", gone).Count(&goneChanges)
	db.Model(&model.ClientTrafficWindow{}).Where("email = ?", kept).Count(&keptWindows)
	if goneWindows != 0 || goneChanges != 0 || keptWindows != clientTrafficWindowKeep+1 {
		t.Fatalf("after the reap: orphan has %d windows and %d changes, the client %d windows; want 0, 0 and %d",
			goneWindows, goneChanges, keptWindows, clientTrafficWindowKeep+1)
	}
}
