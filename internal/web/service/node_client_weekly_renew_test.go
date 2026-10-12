package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestNodeWeeklyRenew_AdoptsNewPeriod(t *testing.T) {
	db := initTrafficTestDB(t)
	createNodeInbound(t, db, 1, "n1-weekly", 41011)
	svc := &InboundService{}
	first := time.Date(2030, time.January, 7, 0, 0, 0, 0, time.UTC).UnixMilli()
	second := first + 7*86400000
	client := model.Client{Email: "node-weekly", ID: "11111111-1111-1111-1111-111111111111", Enable: true, ResetWeekday: 1, ResetMax: 4, ExpiryTime: first}
	stats := xray.ClientTraffic{Email: client.Email, Enable: true, ResetWeekday: 1, ResetMax: 4, ResetCount: 2, ExpiryTime: first}
	syncNodeWithSettings(t, svc, 1, "n1-weekly", clientsSettings(t, []model.Client{client}), stats)
	seeded := readTraffic(t, db, client.Email)
	if seeded.ResetMax != 4 || seeded.ResetCount != 2 || seeded.ResetWeekday != 1 {
		t.Fatalf("node adoption lost renewal policy: %+v", seeded)
	}
	stats.Up, stats.Down = 500, 100
	syncNodeWithSettings(t, svc, 1, "n1-weekly", clientsSettings(t, []model.Client{client}), stats)
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", client.Email).Update("enable", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientGlobalTraffic{MasterGuid: "other", Email: client.Email, Up: 90}).Error; err != nil {
		t.Fatal(err)
	}
	client.ExpiryTime, stats.ExpiryTime = second, second
	stats.Up, stats.Down, stats.ResetCount = 0, 0, 3
	syncNodeWithSettings(t, svc, 1, "n1-weekly", clientsSettings(t, []model.Client{client}), stats)
	got := readTraffic(t, db, client.Email)
	if got.ExpiryTime != second || got.ResetCount != 3 || got.ResetMax != 4 || got.ResetWeekday != 1 || !got.Enable || got.Up != 0 || got.Down != 0 {
		t.Fatalf("node weekly renewal was not adopted: %+v", got)
	}
	var count int64
	if err := db.Model(&model.ClientGlobalTraffic{}).Where("email = ?", client.Email).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("stale global traffic count/error = %d/%v, want 0/nil", count, err)
	}
	var record model.ClientRecord
	if err := db.Where("email = ?", client.Email).First(&record).Error; err != nil {
		t.Fatal(err)
	}
	if record.ResetWeekday != 1 || record.ExpiryTime != second {
		t.Fatalf("node weekly client record lost schedule: %+v", record)
	}
	stats.Up, stats.Down = 20, 8
	syncNodeWithSettings(t, svc, 1, "n1-weekly", clientsSettings(t, []model.Client{client}), stats)
	assertUpDown(t, readTraffic(t, db, client.Email), 20, 8, "new weekly period")
}
