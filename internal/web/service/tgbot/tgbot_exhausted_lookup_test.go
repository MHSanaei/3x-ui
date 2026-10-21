package tgbot

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

func TestNotifyExhaustedLooksUpEachIDOnce(t *testing.T) {
	for _, outcome := range []string{"empty", "failure", "exhausted", "expiry", "admin", "zero", "disabled-inbound"} {
		t.Run(outcome, func(t *testing.T) {
			tb := initReportDB(t)
			clientInfoLocalizer(t)
			mock, sent := captureReportServer(t)
			swapTestBot(t, mock.URL)
			t.Cleanup(mock.Close)
			clients := []model.Client{
				{Email: "notify-a", ID: "uuid-a", TgID: 6886, Enable: true},
				{Email: "notify-b", ID: "uuid-b", TgID: 6886, Enable: true},
			}
			if outcome == "zero" {
				for i := range clients {
					clients[i].TgID = 0
				}
			}
			if outcome == "admin" {
				tgBotMutex.Lock()
				previous := adminIds
				adminIds = append(append([]int64(nil), adminIds...), 6886)
				tgBotMutex.Unlock()
				t.Cleanup(func() {
					tgBotMutex.Lock()
					adminIds = previous
					tgBotMutex.Unlock()
				})
			}
			db := database.GetDB()
			for i := range 2 {
				inbound := &model.Inbound{UserId: 1, Tag: fmt.Sprintf("notify-%d", i), Port: 41890 + i, Enable: true, Protocol: model.VLESS, Settings: `{"clients":[{"email":"notify-a","id":"uuid-a","enable":true,"tgId":6886},{"email":"notify-b","id":"uuid-b","enable":true,"tgId":6886}]}`}
				if outcome == "disabled-inbound" {
					inbound.Enable = false
				}
				if err := db.Create(inbound).Error; err != nil {
					t.Fatal(err)
				}
				if err := (&service.ClientService{}).SyncInbound(nil, inbound.Id, clients); err != nil {
					t.Fatal(err)
				}
				// Keep the inbound eligible even when the Telegram account has no traffic.
				traffic := &xray.ClientTraffic{Email: fmt.Sprintf("unrelated-%d", i), InboundId: inbound.Id}
				if outcome == "exhausted" || outcome == "expiry" {
					traffic.Email = clients[i].Email
					traffic.Enable = true
					traffic.Total = 1
					traffic.Up = 2
					if outcome == "expiry" {
						traffic.Total = 0
						traffic.ExpiryTime = time.Now().Add(-time.Hour).UnixMilli()
					}
				}
				if err := db.Create(traffic).Error; err != nil {
					t.Fatal(err)
				}
			}
			lookups := 0
			cb := "test:notify_lookup"
			if err := db.Callback().Query().Before("gorm:query").Register(cb, func(tx *gorm.DB) {
				if tx.Statement.Table == "clients" && strings.Contains(fmt.Sprint(tx.Statement.Clauses["WHERE"]), "tg_id") {
					lookups++
					if outcome == "failure" {
						tx.AddError(errors.New("injected Telegram lookup failure"))
					}
				}
			}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Callback().Query().Remove(cb) })
			tb.notifyExhausted()
			wantLookups := 1
			if outcome == "admin" || outcome == "zero" || outcome == "disabled-inbound" {
				wantLookups = 0
			}
			if lookups != wantLookups {
				t.Fatalf("lookups = %d, want %d after %s", lookups, wantLookups, outcome)
			}
			got := sent()
			if outcome == "exhausted" || outcome == "expiry" {
				if len(got) != 1 || !strings.Contains(got[0].Text, "notify-a") || !strings.Contains(got[0].Text, "notify-b") {
					t.Fatalf("want one notification containing both clients, got %+v", got)
				}
			} else if len(got) != 0 {
				t.Fatalf("unexpected notifications: %+v", got)
			}
		})
	}
}
