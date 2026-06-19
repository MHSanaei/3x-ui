package service

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	xuilogger "github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"github.com/op/go-logging"
)

func seedClientTraffics(t *testing.T, inboundId int, clients []model.Client) {
	t.Helper()
	db := database.GetDB()
	rows := make([]xray.ClientTraffic, len(clients))
	for i := range clients {
		rows[i] = xray.ClientTraffic{
			InboundId:  inboundId,
			Email:      clients[i].Email,
			Enable:     true,
			Total:      clients[i].TotalGB,
			ExpiryTime: clients[i].ExpiryTime,
		}
	}
	if err := db.CreateInBatches(rows, 1000).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}
}

// TestAllAPIsPostgresScale exercises every client/inbound/group service method
// reachable from the REST API at 100k/200k clients, asserting none crash on the
// PostgreSQL bind-parameter ceiling and logging the wall-clock cost of each.
func TestAllAPIsPostgresScale(t *testing.T) {
	if strings.TrimSpace(os.Getenv("XUI_DB_DSN")) == "" || os.Getenv("XUI_DB_TYPE") != "postgres" {
		t.Skip("set XUI_DB_TYPE=postgres and XUI_DB_DSN to run the postgres scale benchmark")
	}
	xuilogger.InitLogger(logging.ERROR)
	if err := database.InitDB(""); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	svc := &ClientService{}
	inboundSvc := &InboundService{}
	settingSvc := &SettingService{}
	const userId = 1
	const m = 2000
	sizes := []int{50000, 100000, 200000}

	for _, n := range sizes {
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			db := database.GetDB()
			if err := db.Exec("TRUNCATE TABLE inbounds, clients, client_inbounds, client_traffics, client_groups RESTART IDENTITY CASCADE").Error; err != nil {
				t.Fatalf("truncate: %v", err)
			}

			clients := makeScaleClients(n)
			exp := time.Now().AddDate(1, 0, 0).UnixMilli()
			for i := range clients {
				clients[i].ExpiryTime = exp
				clients[i].TotalGB = 100 << 30
			}
			ib := &model.Inbound{UserId: userId, Tag: fmt.Sprintf("all-%d", n), Enable: true, Port: 40000, Protocol: model.VLESS, Settings: clientsSettings(t, clients)}
			if err := db.Create(ib).Error; err != nil {
				t.Fatalf("create inbound: %v", err)
			}
			ib2 := &model.Inbound{UserId: userId, Tag: fmt.Sprintf("all2-%d", n), Enable: true, Port: 40001, Protocol: model.VLESS, Settings: `{"clients":[]}`}
			if err := db.Create(ib2).Error; err != nil {
				t.Fatalf("create inbound2: %v", err)
			}
			if err := svc.SyncInbound(nil, ib.Id, clients); err != nil {
				t.Fatalf("seed SyncInbound: %v", err)
			}

			run := func(name string, fn func() error) {
				start := time.Now()
				if err := fn(); err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				t.Logf("N=%-7d %-26s %v", n, name, time.Since(start).Round(time.Millisecond))
			}

			run("GetInboundDetail(noTraffic)", func() error { _, err := inboundSvc.GetInboundDetail(ib.Id); return err })

			seedClientTraffics(t, ib.Id, clients)
			db.Exec("ANALYZE")

			emails := make([]string, n)
			for i := range n {
				emails[i] = clients[i].Email
			}
			emailsM := emails[:m]

			run("GetInbounds", func() error { _, err := inboundSvc.GetInbounds(userId); return err })
			run("GetInboundsSlim", func() error { _, err := inboundSvc.GetInboundsSlim(userId); return err })
			run("GetInboundDetail", func() error { _, err := inboundSvc.GetInboundDetail(ib.Id); return err })
			run("GetInboundOptions", func() error { _, err := inboundSvc.GetInboundOptions(userId); return err })
			run("ListPaged", func() error {
				_, err := svc.ListPaged(inboundSvc, settingSvc, ClientPageParams{Page: 1, PageSize: 25})
				return err
			})
			run("ListPaged+search", func() error {
				_, err := svc.ListPaged(inboundSvc, settingSvc, ClientPageParams{Page: 1, PageSize: 25, Search: "user-0012345"})
				return err
			})
			run("GetClientsLastOnline", func() error { _, err := inboundSvc.GetClientsLastOnline(); return err })
			run("GetClientTrafficByEmail", func() error { _, err := inboundSvc.GetClientTrafficByEmail(emails[n/2]); return err })
			run("GetRecordByEmail", func() error { _, err := svc.GetRecordByEmail(nil, emails[n/2]); return err })

			run("ListGroups", func() error { _, err := svc.ListGroups(); return err })
			run("AddToGroup(M)", func() error { _, err := svc.AddToGroup(emailsM, "g1"); return err })
			run("EmailsByGroup", func() error { _, err := svc.EmailsByGroup("g1"); return err })
			run("RenameGroup", func() error { _, err := svc.RenameGroup("g1", "g2"); return err })
			run("DeleteGroup", func() error { _, err := svc.DeleteGroup("g2"); return err })

			run("ResetInboundTraffic", func() error { return inboundSvc.ResetInboundTraffic(ib.Id) })
			run("Inbound.ResetAllTraffics", func() error { return inboundSvc.ResetAllTraffics() })
			run("Client.ResetAllTraffics", func() error { _, err := svc.ResetAllTraffics(); return err })
			run("BulkResetTraffic(M)", func() error { _, err := svc.BulkResetTraffic(inboundSvc, emailsM); return err })

			run("UpdateByEmail", func() error {
				upd := clients[n/3]
				upd.Comment = "touched"
				_, err := svc.UpdateByEmail(inboundSvc, upd.Email, upd)
				return err
			})
			run("AttachByEmail", func() error { _, err := svc.AttachByEmail(inboundSvc, emails[n/3], []int{ib2.Id}); return err })
			run("DetachByEmailMany", func() error { _, err := svc.DetachByEmailMany(inboundSvc, emails[n/3], []int{ib2.Id}); return err })

			depEmails := emails[:1000]
			for _, batch := range chunkStrings(depEmails, sqlInChunk) {
				if err := db.Model(xray.ClientTraffic{}).Where("email IN ?", batch).Update("down", int64(200)<<30).Error; err != nil {
					t.Fatalf("mark depleted: %v", err)
				}
			}
			run("DelDepleted(1k)", func() error { _, _, err := svc.DelDepleted(inboundSvc); return err })

			run("DelInbound(full)", func() error { _, err := inboundSvc.DelInbound(ib.Id); return err })
		})
	}
}

// TestGetClientTrafficByEmailABScale measures the GetClientTrafficByEmail change:
// old path (GetClientByEmail, which parses the inbound's entire settings JSON to
// find one client) vs new path (UUID/subId read from the indexed clients table).
func TestGetClientTrafficByEmailABScale(t *testing.T) {
	if strings.TrimSpace(os.Getenv("XUI_DB_DSN")) == "" || os.Getenv("XUI_DB_TYPE") != "postgres" {
		t.Skip("set XUI_DB_TYPE=postgres and XUI_DB_DSN to run the postgres scale benchmark")
	}
	xuilogger.InitLogger(logging.ERROR)
	if err := database.InitDB(""); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	svc := &ClientService{}
	inboundSvc := &InboundService{}
	const reps = 10
	sizes := []int{50000, 100000, 200000}

	oldImpl := func(email string) error {
		tr, client, err := inboundSvc.GetClientByEmail(email)
		if err != nil {
			return err
		}
		if tr != nil && client != nil {
			tr.UUID = client.ID
			tr.SubId = client.SubID
		}
		return nil
	}

	for _, n := range sizes {
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			db := database.GetDB()
			if err := db.Exec("TRUNCATE TABLE inbounds, clients, client_inbounds, client_traffics RESTART IDENTITY CASCADE").Error; err != nil {
				t.Fatalf("truncate: %v", err)
			}
			clients := makeScaleClients(n)
			ib := &model.Inbound{UserId: 1, Tag: fmt.Sprintf("ctbe-%d", n), Enable: true, Port: 40000, Protocol: model.VLESS, Settings: clientsSettings(t, clients)}
			if err := db.Create(ib).Error; err != nil {
				t.Fatalf("create inbound: %v", err)
			}
			if err := svc.SyncInbound(nil, ib.Id, clients); err != nil {
				t.Fatalf("seed SyncInbound: %v", err)
			}
			seedClientTraffics(t, ib.Id, clients)
			db.Exec("ANALYZE")

			targets := []string{clients[0].Email, clients[n/2].Email, clients[n-1].Email}

			start := time.Now()
			for i := range reps {
				if _, err := inboundSvc.GetClientTrafficByEmail(targets[i%len(targets)]); err != nil {
					t.Fatalf("new GetClientTrafficByEmail: %v", err)
				}
			}
			newDur := time.Since(start) / reps

			start = time.Now()
			for i := range reps {
				if err := oldImpl(targets[i%len(targets)]); err != nil {
					t.Fatalf("old GetClientTrafficByEmail: %v", err)
				}
			}
			oldDur := time.Since(start) / reps

			t.Logf("N=%-7d new=%-9v old=%-9v speedup=%.0fx", n,
				newDur.Round(time.Microsecond), oldDur.Round(time.Millisecond),
				float64(oldDur)/float64(maxDur(newDur, time.Microsecond)))
		})
	}
}
