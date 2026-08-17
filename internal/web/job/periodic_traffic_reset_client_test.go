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

type seededClient struct {
	email        string
	cycle        string
	day          int
	recordEnable bool
	quotaEnable  bool
	total        int64
}

// seedClientOnCycle creates an inbound that never resets on its own, a client
// carrying its own cycle, and the client_inbounds link the reset path resolves
// through — without it every reset falls into the orphaned-client branch.
func seedClientOnCycle(t *testing.T, port int, c seededClient) {
	t.Helper()
	db := database.GetDB()

	client := model.Client{
		Email: c.email, ID: uuidFor(port), Enable: c.recordEnable,
		TrafficReset: c.cycle, TrafficResetDay: c.day,
	}
	settings, err := json.Marshal(map[string]any{"clients": []model.Client{client}})
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	ib := model.Inbound{
		UserId: 1, Enable: true, Port: port, Protocol: model.VLESS,
		Tag: "inbound-" + c.email, TrafficReset: "never", Settings: string(settings),
	}
	if err := db.Create(&ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	rec := model.ClientRecord{
		Email: c.email, UUID: client.ID, Enable: c.recordEnable,
		TrafficReset: c.cycle, TrafficResetDay: c.day,
	}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatalf("create client record: %v", err)
	}
	// gorm skips a false bool on insert, so the column default:true wins; the
	// disabled case has to be written back explicitly.
	if err := db.Model(&model.ClientRecord{}).Where("id = ?", rec.Id).
		Update("enable", c.recordEnable).Error; err != nil {
		t.Fatalf("set record enable: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("link client to inbound: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{
		InboundId: ib.Id, Email: c.email, Enable: c.quotaEnable, Up: 500, Down: 700, Total: c.total,
	}).Error; err != nil {
		t.Fatalf("create traffic: %v", err)
	}
}

func uuidFor(port int) string {
	return "00000000-0000-0000-0000-0000000" + string(rune('0'+port/10000%10)) +
		string(rune('0'+port/1000%10)) + string(rune('0'+port/100%10)) +
		string(rune('0'+port/10%10)) + string(rune('0'+port%10))
}

func trafficFor(t *testing.T, email string) xray.ClientTraffic {
	t.Helper()
	var row xray.ClientTraffic
	if err := database.GetDB().Where("email = ?", email).First(&row).Error; err != nil {
		t.Fatalf("read traffic for %s: %v", email, err)
	}
	return row
}

func recordFor(t *testing.T, email string) model.ClientRecord {
	t.Helper()
	var rec model.ClientRecord
	if err := database.GetDB().Where("email = ?", email).First(&rec).Error; err != nil {
		t.Fatalf("read record for %s: %v", email, err)
	}
	return rec
}

func TestPeriodicTrafficResetClients(t *testing.T) {
	t.Run("resets a client on its own cycle inside a never-reset inbound", func(t *testing.T) {
		initResetJobDB(t)
		seedClientOnCycle(t, 41001, seededClient{email: "weekly@example.com", cycle: "weekly", day: 1, recordEnable: true, quotaEnable: true})
		seedClientOnCycle(t, 41002, seededClient{email: "monthly@example.com", cycle: "monthly", day: 1, recordEnable: true, quotaEnable: true})

		NewPeriodicTrafficResetJob("weekly", time.UTC).Run()

		if row := trafficFor(t, "weekly@example.com"); row.Up != 0 || row.Down != 0 {
			t.Fatalf("weekly client not reset by the weekly run: up=%d down=%d", row.Up, row.Down)
		}
		if row := trafficFor(t, "monthly@example.com"); row.Up != 500 || row.Down != 700 {
			t.Fatalf("monthly client reset by the weekly run: up=%d down=%d", row.Up, row.Down)
		}
	})

	t.Run("leaves a client with no cycle of its own alone", func(t *testing.T) {
		initResetJobDB(t)
		seedClientOnCycle(t, 41003, seededClient{email: "none@example.com", cycle: "never", day: 1, recordEnable: true, quotaEnable: true})

		for _, period := range []Period{"hourly", "daily", "weekly", "monthly"} {
			NewPeriodicTrafficResetJob(period, time.UTC).Run()
		}

		if row := trafficFor(t, "none@example.com"); row.Up != 500 || row.Down != 700 {
			t.Fatalf("client with trafficReset=never was reset: up=%d down=%d", row.Up, row.Down)
		}
	})

	t.Run("monthly client waits for its own day", func(t *testing.T) {
		initResetJobDB(t)
		today := time.Now().In(time.UTC).Day()
		otherDay := today%28 + 1
		seedClientOnCycle(t, 41004, seededClient{email: "due@example.com", cycle: "monthly", day: today, recordEnable: true, quotaEnable: true})
		seedClientOnCycle(t, 41005, seededClient{email: "notdue@example.com", cycle: "monthly", day: otherDay, recordEnable: true, quotaEnable: true})

		NewPeriodicTrafficResetJob("monthly", time.UTC).Run()

		if row := trafficFor(t, "due@example.com"); row.Up != 0 || row.Down != 0 {
			t.Fatalf("client due today was not reset: up=%d down=%d", row.Up, row.Down)
		}
		if row := trafficFor(t, "notdue@example.com"); row.Up != 500 || row.Down != 700 {
			t.Fatalf("client due on another day was reset: up=%d down=%d", row.Up, row.Down)
		}
	})

	t.Run("restores a client the quota switched off", func(t *testing.T) {
		initResetJobDB(t)
		// Depletion disables all three of client_traffics.enable, clients.enable
		// and the settings JSON, so a reset that lifts only the first leaves the
		// client out of the running core with nothing left to revisit it.
		seedClientOnCycle(t, 41006, seededClient{
			email: "depleted@example.com", cycle: "daily", day: 1,
			recordEnable: false, quotaEnable: false, total: 1000,
		})

		NewPeriodicTrafficResetJob("daily", time.UTC).Run()

		if row := trafficFor(t, "depleted@example.com"); !row.Enable {
			t.Fatal("quota gate not lifted: the client cannot use its new allowance")
		}
		if rec := recordFor(t, "depleted@example.com"); !rec.Enable {
			t.Fatal("clients.enable still false: GetXrayConfig skips the client, so it stays locked out for good")
		}
		if enabled := settingsEnableOf(t, 41006); !enabled {
			t.Fatal("the inbound settings JSON still has the client disabled")
		}
	})

	t.Run("leaves a client the operator switched off", func(t *testing.T) {
		initResetJobDB(t)
		// Disabled with usage below quota: nothing but a human did that.
		seedClientOnCycle(t, 41007, seededClient{
			email: "banned@example.com", cycle: "daily", day: 1,
			recordEnable: false, quotaEnable: true, total: 100000,
		})

		NewPeriodicTrafficResetJob("daily", time.UTC).Run()

		if rec := recordFor(t, "banned@example.com"); rec.Enable {
			t.Fatal("an operator-disabled client was switched back on by a cron job")
		}
		if row := trafficFor(t, "banned@example.com"); row.Up != 500 || row.Down != 700 {
			t.Fatalf("an operator-disabled client was reset anyway: up=%d down=%d", row.Up, row.Down)
		}
	})
}

func settingsEnableOf(t *testing.T, port int) bool {
	t.Helper()
	var stored model.Inbound
	if err := database.GetDB().Where("port = ?", port).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Clients []model.Client `json:"clients"`
	}
	if err := json.Unmarshal([]byte(stored.Settings), &settings); err != nil {
		t.Fatalf("parse inbound settings: %v", err)
	}
	if len(settings.Clients) != 1 {
		t.Fatalf("inbound holds %d clients, want 1", len(settings.Clients))
	}
	return settings.Clients[0].Enable
}
