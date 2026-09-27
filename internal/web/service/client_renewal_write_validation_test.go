package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestClientRenewalWriteValidation(t *testing.T) {
	for _, operation := range []string{"add inbound", "update inbound", "add inbound client", "update inbound client", "sync inbound", "client delta", "add stat", "update stat", "import stat"} {
		for _, schedule := range []struct {
			name                string
			reset, day, weekday int
			errorMessage        string
		}{
			{"weekly and interval", 7, 0, 3, "client weekly renewal cannot be combined with reset or resetDay"},
			{"weekly and monthly", 0, 1, 3, "client weekly renewal cannot be combined with reset or resetDay"},
			{"invalid weekday", 0, 0, 8, "client resetWeekday must be between 0 and 7, got: 8"},
		} {
			t.Run(operation+"/"+schedule.name, func(t *testing.T) {
				setupConflictDB(t)
				nodeID, fake := setupNodeRuntime(t)
				svc, inboundSvc := &ClientService{}, &InboundService{}
				client := model.Client{Email: "renewal-boundary", ID: "11111111-1111-1111-1111-111111111111", SubID: "renewal-boundary-sub", Enable: true, ResetWeekday: 3, ResetMax: 4, ExpiryTime: time.Now().Add(time.Hour).UnixMilli()}
				inbound := nodeInbound(t, nodeID, 41759, []model.Client{client})
				if err := inboundSvc.AddClientStat(database.GetDB(), inbound.Id, &client); err != nil {
					t.Fatal(err)
				}
				record, err := svc.GetRecordByEmail(nil, client.Email)
				if err != nil {
					t.Fatal(err)
				}
				beforeRecord := *record
				beforeTraffic := readTraffic(t, database.GetDB(), client.Email)
				beforeSettings := inbound.Settings
				client.Reset, client.ResetDay, client.ResetWeekday = schedule.reset, schedule.day, schedule.weekday
				update := *inbound
				update.Settings = clientsSettings(t, []model.Client{client})
				switch operation {
				case "add inbound", "import stat":
					update.Id, update.Port, update.Tag = 0, 41760, "renewal-boundary-new"
					if operation == "import stat" {
						update.Settings = beforeSettings
						update.ClientStats = []xray.ClientTraffic{{Email: "invalid-import-stat", Reset: schedule.reset, ResetDay: schedule.day, ResetWeekday: schedule.weekday}}
					}
					_, _, err = inboundSvc.AddInbound(&update)
				case "update inbound":
					_, _, err = inboundSvc.UpdateInbound(&update)
				case "add inbound client":
					client.Email = "invalid-new-client"
					update.Settings = clientsSettings(t, []model.Client{client})
					_, err = svc.AddInboundClient(inboundSvc, &update)
				case "update inbound client":
					_, err = svc.UpdateInboundClient(inboundSvc, &update, client.Email)
				case "sync inbound":
					err = svc.SyncInbound(nil, inbound.Id, []model.Client{client})
				case "client delta":
					err = svc.ApplyInboundClientDelta(nil, inbound.Id, []model.Client{client}, nil)
				case "add stat":
					err = inboundSvc.AddClientStat(database.GetDB(), inbound.Id, &client)
				case "update stat":
					err = inboundSvc.UpdateClientStat(database.GetDB(), client.Email, &client)
				}
				if err == nil || err.Error() != schedule.errorMessage+"\n" {
					t.Fatalf("write error = %v, want %q", err, schedule.errorMessage)
				}
				var persisted model.Inbound
				if err := database.GetDB().First(&persisted, inbound.Id).Error; err != nil {
					t.Fatal(err)
				}
				if persisted.Settings != beforeSettings {
					t.Fatal("rejected write changed inbound settings")
				}
				record, err = svc.GetRecordByEmail(nil, beforeRecord.Email)
				if err != nil || *record != beforeRecord || readTraffic(t, database.GetDB(), beforeRecord.Email) != beforeTraffic {
					t.Fatalf("rejected write changed client/traffic: record=%+v error=%v", record, err)
				}
				for _, table := range []string{"inbounds", "clients", "client_traffics"} {
					var count int64
					if err := database.GetDB().Table(table).Count(&count).Error; err != nil || count != 1 {
						t.Fatalf("%s count/error = %d/%v, want 1/nil", table, count, err)
					}
				}
				if fake.addInbound.Load() != 0 || fake.updateInbound.Load() != 0 || fake.delInbound.Load() != 0 || fake.addClient.Load() != 0 || fake.updateUser.Load() != 0 {
					t.Fatal("rejected write dispatched to the runtime")
				}
			})
		}
	}
}

func TestInboundRenewalModesRemainEditable(t *testing.T) {
	for _, weekly := range []bool{false, true} {
		name := "legacy monthly with interval"
		if weekly {
			name = "weekly"
		}
		t.Run(name, func(t *testing.T) {
			setupConflictDB(t)
			nodeID, _ := setupNodeRuntime(t)
			svc, inboundSvc := &ClientService{}, &InboundService{}
			client := model.Client{Email: "renewal-editable", ID: "11111111-1111-1111-1111-111111111111", Enable: true, Reset: 7, ResetDay: 1, ResetMax: 4, ExpiryTime: time.Now().Add(time.Hour).UnixMilli()}
			if weekly {
				client.Reset, client.ResetDay, client.ResetWeekday = 0, 0, 3
			}
			inbound := &model.Inbound{Tag: "renewal-editable", NodeID: &nodeID, Port: 41761, Protocol: model.VLESS, Enable: true, Settings: clientsSettings(t, []model.Client{client})}
			if _, _, err := inboundSvc.AddInbound(inbound); err != nil {
				t.Fatal(err)
			}
			if _, _, err := inboundSvc.UpdateInbound(inbound); err != nil {
				t.Fatal(err)
			}
			record, err := svc.GetRecordByEmail(nil, client.Email)
			if err != nil {
				t.Fatal(err)
			}
			toggle := *record.ToClient()
			toggle.Enable = false
			if _, err := svc.Update(inboundSvc, record.Id, toggle, 0); err != nil {
				t.Fatalf("valid inbound client could not be toggled: %v", err)
			}
			record, err = svc.GetRecordByEmail(nil, client.Email)
			if err != nil || record.Enable || record.Reset != client.Reset || record.ResetDay != client.ResetDay || record.ResetWeekday != client.ResetWeekday || record.ResetMax != 4 {
				t.Fatalf("valid schedule/toggle not preserved: %+v/%v", record, err)
			}
		})
	}
}
