package job

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/op/go-logging"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// mtg counts a live connection that moved no bytes as online, so the panel shows
// that client Online; its last_online must keep moving with it instead of going stale.
func TestMtprotoJobBumpsConnectedIdleClients(t *testing.T) {
	xuilogger.InitLogger(logging.ERROR)
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	db := database.GetDB()

	const connected, disconnected = "connected@mtproto", "disconnected@mtproto"
	stale := time.Now().Add(-time.Hour).UnixMilli()
	for _, email := range []string{connected, disconnected} {
		if err := db.Create(&xray.ClientTraffic{Email: email, Enable: true, LastOnline: stale}).Error; err != nil {
			t.Fatalf("seed %s: %v", email, err)
		}
	}

	before := time.Now().UnixMilli()
	NewMtprotoJob().recordTraffic(nil, []string{connected}, nil, nil)

	lastOnline := func(email string) int64 {
		t.Helper()
		var ct xray.ClientTraffic
		if err := db.Where("email = ?", email).First(&ct).Error; err != nil {
			t.Fatalf("read %s: %v", email, err)
		}
		return ct.LastOnline
	}
	if got := lastOnline(connected); got < before {
		t.Errorf("connected idle client last_online = %d, want >= %d: it reads stale while shown Online", got, before)
	}
	if got := lastOnline(disconnected); got != stale {
		t.Errorf("disconnected client last_online = %d, want unchanged %d", got, stale)
	}
}
