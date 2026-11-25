package job

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Nothing archives or cleans up when a client is deleted, so without this run the history
// of every deleted client stays in the database for good.
func TestReapSyncOrphansJobDropsTheHistoryOfDeletedClients(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	db := database.GetDB()
	const gone, kept = "gone@example.test", "kept@example.test"
	if err := db.Create(&model.ClientRecord{Email: kept, SubID: kept, Enable: true}).Error; err != nil {
		t.Fatal(err)
	}
	for _, email := range []string{gone, kept} {
		if err := db.Create(&model.ClientTrafficWindow{Email: email, At: 1, Up: 5, Reason: "reset"}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.ClientChangeLog{Email: email, At: 1, Field: "enable"}).Error; err != nil {
			t.Fatal(err)
		}
	}

	NewReapSyncOrphansJob().Run()

	var windows, changes []string
	if err := db.Model(&model.ClientTrafficWindow{}).Pluck("email", &windows).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.ClientChangeLog{}).Pluck("email", &changes).Error; err != nil {
		t.Fatal(err)
	}
	if len(windows) != 1 || windows[0] != kept || len(changes) != 1 || changes[0] != kept {
		t.Fatalf("after the job: windows of %v, changes of %v; want only %s in both", windows, changes, kept)
	}
}
