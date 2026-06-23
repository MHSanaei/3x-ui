package sub

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestAggregateTrafficByEmails_FallsBackToClientLimits(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	const email = "node-client@example.com"
	const totalBytes = int64(300) * 1024 * 1024 * 1024
	const expiry = int64(1893456000000)

	db := database.GetDB()
	if err := db.Create(&model.ClientRecord{
		Email:      email,
		TotalGB:    totalBytes,
		ExpiryTime: expiry,
		Enable:     true,
	}).Error; err != nil {
		t.Fatalf("seed client record: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{
		Email:      email,
		Up:         111,
		Down:       222,
		Total:      0,
		ExpiryTime: 0,
		Enable:     true,
	}).Error; err != nil {
		t.Fatalf("seed client traffic: %v", err)
	}

	var s SubService
	agg, _ := s.AggregateTrafficByEmails([]string{email})

	if agg.Up != 111 || agg.Down != 222 {
		t.Errorf("usage = up %d/down %d, want 111/222", agg.Up, agg.Down)
	}
	if agg.Total != totalBytes {
		t.Errorf("total = %d, want %d (fallback to clients table)", agg.Total, totalBytes)
	}
	if agg.ExpiryTime != expiry {
		t.Errorf("expiry = %d, want %d (fallback to clients table)", agg.ExpiryTime, expiry)
	}
}

// The aggregate must carry Billed alongside Real so the Subscription-Userinfo
// header, the sub info page and the remark tokens all report the figure quota
// enforcement actually cuts on (Billed), keeping them consistent on non-1x
// inbounds where Billed != Real.
func TestAggregateTrafficByEmails_CarriesBilled(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	const email = "billed@example.com"
	db := database.GetDB()
	if err := db.Create(&xray.ClientTraffic{
		Email:      email,
		Up:         100,
		Down:       200,
		BilledUp:   250,
		BilledDown: 400,
		Enable:     true,
	}).Error; err != nil {
		t.Fatalf("seed client traffic: %v", err)
	}

	var s SubService
	agg, _ := s.AggregateTrafficByEmails([]string{email})
	if agg.Up != 100 || agg.Down != 200 {
		t.Errorf("real = up %d/down %d, want 100/200", agg.Up, agg.Down)
	}
	if agg.BilledUp != 250 || agg.BilledDown != 400 {
		t.Errorf("billed = up %d/down %d, want 250/400", agg.BilledUp, agg.BilledDown)
	}
}
