package job

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func initResetJobDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

// seedClientOnCycle creates an inbound that never resets on its own, holding one
// client that carries its own cycle, plus the client's traffic row.
func seedClientOnCycle(t *testing.T, port int, email, cycle string, day int, up, down int64) {
	t.Helper()
	db := database.GetDB()

	client := model.Client{
		Email: email, ID: "00000000-0000-0000-0000-00000000000" + string(rune('0'+port%10)),
		Enable: true, TrafficReset: cycle, TrafficResetDay: day,
	}
	settings, err := json.Marshal(map[string]any{"clients": []model.Client{client}})
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	ib := model.Inbound{
		UserId: 1, Enable: true, Port: port, Protocol: model.VLESS,
		Tag: "inbound-" + email, TrafficReset: "never", Settings: string(settings),
	}
	if err := db.Create(&ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	if err := db.Create(&model.ClientRecord{
		Email: email, UUID: client.ID, Enable: true,
		TrafficReset: cycle, TrafficResetDay: day,
	}).Error; err != nil {
		t.Fatalf("create client record: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{
		InboundId: ib.Id, Email: email, Enable: true, Up: up, Down: down,
	}).Error; err != nil {
		t.Fatalf("create traffic: %v", err)
	}
}

func trafficFor(t *testing.T, email string) xray.ClientTraffic {
	t.Helper()
	var row xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", email).First(&row).Error; err != nil {
		t.Fatalf("read traffic for %s: %v", email, err)
	}
	return row
}

// The whole point of #5497: a weekly client inside an inbound that never resets
// must still be reset, and a client on a different cycle must be left alone.
func TestPeriodicTrafficReset_ResetsClientsOnTheirOwnCycle(t *testing.T) {
	initResetJobDB(t)

	seedClientOnCycle(t, 41001, "weekly@example.com", "weekly", 1, 500, 700)
	seedClientOnCycle(t, 41002, "monthly@example.com", "monthly", 1, 900, 1100)

	NewPeriodicTrafficResetJob("weekly", time.UTC).Run()

	if row := trafficFor(t, "weekly@example.com"); row.Up != 0 || row.Down != 0 {
		t.Fatalf("weekly client not reset by the weekly run: up=%d down=%d", row.Up, row.Down)
	}
	if row := trafficFor(t, "monthly@example.com"); row.Up != 900 || row.Down != 1100 {
		t.Fatalf("monthly client reset by the weekly run: up=%d down=%d", row.Up, row.Down)
	}
}

// A client with no cycle of its own keeps the old behaviour: only its inbound's
// schedule can reset it.
func TestPeriodicTrafficReset_LeavesClientsWithoutACycle(t *testing.T) {
	initResetJobDB(t)

	seedClientOnCycle(t, 41003, "none@example.com", "never", 1, 300, 400)

	for _, period := range []Period{"hourly", "daily", "weekly", "monthly"} {
		NewPeriodicTrafficResetJob(period, time.UTC).Run()
	}

	if row := trafficFor(t, "none@example.com"); row.Up != 300 || row.Down != 400 {
		t.Fatalf("client with trafficReset=never was reset: up=%d down=%d", row.Up, row.Down)
	}
}

// Monthly clients only come due on their own day of the month, the same rule the
// inbound-level schedule already follows.
func TestPeriodicTrafficReset_MonthlyClientWaitsForItsDay(t *testing.T) {
	initResetJobDB(t)

	today := time.Now().In(time.UTC).Day()
	otherDay := today%28 + 1
	if otherDay == today {
		otherDay = today%28 + 2
	}
	seedClientOnCycle(t, 41004, "due@example.com", "monthly", today, 100, 200)
	seedClientOnCycle(t, 41005, "notdue@example.com", "monthly", otherDay, 100, 200)

	NewPeriodicTrafficResetJob("monthly", time.UTC).Run()

	if row := trafficFor(t, "due@example.com"); row.Up != 0 || row.Down != 0 {
		t.Fatalf("client due today was not reset: up=%d down=%d", row.Up, row.Down)
	}
	if row := trafficFor(t, "notdue@example.com"); row.Up != 100 || row.Down != 200 {
		t.Fatalf("client due on another day was reset: up=%d down=%d", row.Up, row.Down)
	}
}
