package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestAutoRenewClients_WeeklyMode(t *testing.T) {
	for _, tt := range []struct {
		name, zone          string
		weekday, max, count int
		inclusive, manual   bool
	}{
		{name: "UTC Monday catch-up across shared inbounds", zone: "UTC", weekday: 1},
		{name: "New York Sunday across daylight saving", zone: "America/New_York", weekday: 7},
		{name: "capped catch-up stays expired", zone: "UTC", weekday: 3, max: 3, count: 2},
		{name: "inclusive last second spends one allowance", zone: "Asia/Taipei", weekday: 1, max: 1, inclusive: true},
		{name: "operator-disabled settings stay disabled", zone: "UTC", weekday: 5, manual: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setupBulkDB(t)
			db := database.GetDB()
			zone := pinPanelZone(t, tt.zone)
			now := time.Now().In(zone)
			boundary := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, zone)
			for int(boundary.Weekday()+6)%7+1 != tt.weekday || !boundary.Before(now) {
				boundary = boundary.AddDate(0, 0, -1)
			}
			past := boundary.AddDate(0, 0, -21)
			if tt.zone == "America/New_York" {
				past = time.Date(2026, time.March, 1, 0, 0, 0, 0, zone)
			}
			if tt.inclusive {
				past = boundary.Add(-time.Second)
			}
			client := model.Client{
				Email: "weekly@x", ID: "11111111-1111-1111-1111-111111111111",
				ResetWeekday: tt.weekday, ResetMax: tt.max, ExpiryTime: past.UnixMilli(),
			}
			svc := &InboundService{}
			for _, port := range []int{30241, 30242} {
				ib := mkInbound(t, port, model.VLESS, clientsSettings(t, []model.Client{client}))
				if err := svc.clientService.SyncInbound(nil, ib.Id, []model.Client{client}); err != nil {
					t.Fatal(err)
				}
			}
			traffic := xray.ClientTraffic{
				Email: client.Email, ResetWeekday: tt.weekday, ResetMax: tt.max, ResetCount: tt.count,
				ExpiryTime: past.UnixMilli(), Up: 111, Down: 222, Enable: tt.manual,
			}
			if err := db.Create(&traffic).Error; err != nil {
				t.Fatal(err)
			}
			want, steps := past, 0
			if tt.inclusive {
				want = boundary
			}
			for !want.After(now) && (tt.max == 0 || tt.count+steps < tt.max) {
				want = want.AddDate(0, 0, 7)
				steps++
			}
			if _, _, err := svc.autoRenewClients(db, newTrafficMutationBatch()); err != nil {
				t.Fatal(err)
			}
			var got xray.ClientTraffic
			if err := db.Where("email = ?", client.Email).First(&got).Error; err != nil {
				t.Fatal(err)
			}
			if got.ExpiryTime != want.UnixMilli() || got.ResetCount != tt.count+steps || got.ResetWeekday != tt.weekday {
				t.Fatalf("expiry/count/weekday = %d/%d/%d, want %d/%d/%d", got.ExpiryTime, got.ResetCount, got.ResetWeekday, want.UnixMilli(), tt.count+steps, tt.weekday)
			}
			if want.After(now) {
				if !got.Enable || got.Up != 0 || got.Down != 0 {
					t.Fatalf("renewed enable/up/down = %v/%d/%d, want true/0/0", got.Enable, got.Up, got.Down)
				}
			} else if got.Enable || got.Up != 111 || got.Down != 222 {
				t.Fatalf("capped enable/up/down = %v/%d/%d, want false/111/222", got.Enable, got.Up, got.Down)
			}
			var record model.ClientRecord
			if err := db.Where("email = ?", client.Email).First(&record).Error; err != nil {
				t.Fatal(err)
			}
			if record.ResetWeekday != tt.weekday || record.ExpiryTime != got.ExpiryTime || record.Enable != (want.After(now) && !tt.manual) {
				t.Fatalf("client record lost weekly schedule or operator enable state: %+v", record)
			}
			if _, count, err := svc.autoRenewClients(db, newTrafficMutationBatch()); err != nil || count != 0 {
				t.Fatalf("repeat tick count/error = %d/%v, want 0/nil", count, err)
			}
		})
	}
}
