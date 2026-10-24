package job

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestMonthlyResetDue(t *testing.T) {
	cases := []struct {
		name     string
		resetDay int
		now      time.Time
		want     bool
	}{
		{"legacy default on first", 0, time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC), true},
		{"configured day", 15, time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC), true},
		{"before configured day", 15, time.Date(2026, time.July, 14, 0, 0, 0, 0, time.UTC), false},
		{"month end", 31, time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC), true},
		{"short month fallback", 31, time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC), true},
		{"leap year fallback", 31, time.Date(2028, time.February, 29, 0, 0, 0, 0, time.UTC), true},
		{"not before short month end", 31, time.Date(2028, time.February, 28, 0, 0, 0, 0, time.UTC), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := monthlyResetDue(tc.resetDay, tc.now); got != tc.want {
				t.Fatalf("monthlyResetDue(%d, %s) = %v, want %v", tc.resetDay, tc.now.Format(time.DateOnly), got, tc.want)
			}
		})
	}
}

func TestPeriodicTrafficResetReenablesQuotaDisabledInbound(t *testing.T) {
	cases := []struct {
		name       string
		total      int64
		expiryTime int64
		want       bool
	}{
		{"switched off by its own quota", 1000, 0, true},
		{"switched off by hand below its quota", 100000, 0, false},
		{"over quota but expired", 1000, time.Now().Add(-time.Hour).UnixMilli(), false},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			initResetJobDB(t)
			db := database.GetDB()
			ib := model.Inbound{
				UserId: 1, Enable: true, Port: 41201 + i, Protocol: model.VLESS, Tag: "inbound-quota",
				TrafficReset: "daily", Total: tc.total, ExpiryTime: tc.expiryTime, Up: 600, Down: 500,
				Settings: `{"clients":[]}`,
			}
			if err := db.Create(&ib).Error; err != nil {
				t.Fatalf("create inbound: %v", err)
			}
			if err := db.Model(&model.Inbound{}).Where("id = ?", ib.Id).Update("enable", false).Error; err != nil {
				t.Fatalf("disable inbound: %v", err)
			}

			NewPeriodicTrafficResetJob("daily", time.UTC).Run()

			var got model.Inbound
			if err := db.First(&got, ib.Id).Error; err != nil {
				t.Fatalf("reload inbound: %v", err)
			}
			if got.Up != 0 || got.Down != 0 {
				t.Fatalf("inbound traffic not reset: up=%d down=%d", got.Up, got.Down)
			}
			if got.Enable != tc.want {
				t.Fatalf("inbound enable = %v after its reset, want %v", got.Enable, tc.want)
			}
		})
	}
}
