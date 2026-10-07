package job

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestTuicShutdownJournalSurvivesDatabaseFailureAndReplaysOnce(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dir)
	dbtest.InitDB(t, filepath.Join(dir, "x-ui.db"))
	db := database.GetDB()
	row := xray.ClientTraffic{Email: "journal@x", Enable: true}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	batch := tuicTrafficBatch{ID: "shutdown-batch", Deltas: []tuic.ClientTrafficDelta{{Email: row.Email, TrafficID: row.Id, Up: 123, Down: 456}}}
	path, err := storeTuicBatch(batch)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TRIGGER fail_tuic_write BEFORE UPDATE ON client_traffics BEGIN SELECT RAISE(FAIL, 'disk fault'); END`).Error; err != nil {
		t.Fatal(err)
	}
	job := NewTuicJob()
	if err := job.FlushStoppedTraffic(); err == nil {
		t.Fatal("failed traffic update must return an error")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("durable recovery file missing: %v", err)
	}
	var receipts int64
	if err := db.Table("tuic_traffic_receipts").Count(&receipts).Error; err == nil && receipts != 0 {
		t.Fatal("failed transaction committed a receipt")
	}
	if err := db.Exec("DROP TRIGGER fail_tuic_write").Error; err != nil {
		t.Fatal(err)
	}
	// A new job has no knowledge of the previous process's in-memory deltas.
	recovered := NewTuicJob()
	if err := recovered.replayTuicJournal(); err != nil {
		t.Fatal(err)
	}
	// Recreate a stale file, as if file removal was lost after DB commit.
	if _, err := storeTuicBatch(batch); err != nil {
		t.Fatal(err)
	}
	if err := recovered.replayTuicJournal(); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("id = ?", row.Id).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Up != 123 || row.Down != 456 {
		t.Fatalf("replay billed %d/%d", row.Up, row.Down)
	}
	entries, err := os.ReadDir(tuicJournalDir())
	if err != nil || len(entries) != 0 {
		t.Fatalf("journal not drained: %v %v", entries, err)
	}
}
