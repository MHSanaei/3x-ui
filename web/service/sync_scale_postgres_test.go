package service

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"

	"gorm.io/gorm"
)

func syncInboundOld(tx *gorm.DB, inboundId int, clients []model.Client) error {
	if tx == nil {
		tx = database.GetDB()
	}
	if err := tx.Where("inbound_id = ?", inboundId).Delete(&model.ClientInbound{}).Error; err != nil {
		return err
	}
	for i := range clients {
		c := clients[i]
		email := strings.TrimSpace(c.Email)
		if email == "" {
			continue
		}
		incoming := c.ToRecord()
		row := &model.ClientRecord{}
		err := tx.Where("email = ?", email).First(row).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Create(incoming).Error; err != nil {
				return err
			}
			row = incoming
		} else {
			row.Flow = incoming.Flow
			row.SubID = incoming.SubID
			row.LimitIP = incoming.LimitIP
			row.TotalGB = incoming.TotalGB
			row.ExpiryTime = incoming.ExpiryTime
			row.Enable = incoming.Enable
			row.TgID = incoming.TgID
			row.Comment = incoming.Comment
			row.Reset = incoming.Reset
			preservedUpdatedAt := max(incoming.UpdatedAt, row.UpdatedAt)
			row.UpdatedAt = preservedUpdatedAt
			if err := tx.Save(row).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.ClientRecord{}).
				Where("id = ?", row.Id).
				UpdateColumn("updated_at", preservedUpdatedAt).Error; err != nil {
				return err
			}
		}
		link := model.ClientInbound{ClientId: row.Id, InboundId: inboundId, FlowOverride: c.Flow}
		if err := tx.Create(&link).Error; err != nil {
			return err
		}
	}
	return nil
}

func makeScaleClients(n int) []model.Client {
	out := make([]model.Client, n)
	for i := 0; i < n; i++ {
		out[i] = model.Client{
			ID:     uuid.NewString(),
			Email:  fmt.Sprintf("user-%07d@scale", i),
			SubID:  fmt.Sprintf("sub-%07d", i),
			Enable: true,
		}
	}
	return out
}

func TestSyncInboundPostgresScale(t *testing.T) {
	if strings.TrimSpace(os.Getenv("XUI_DB_DSN")) == "" || os.Getenv("XUI_DB_TYPE") != "postgres" {
		t.Skip("set XUI_DB_TYPE=postgres and XUI_DB_DSN to run the postgres scale benchmark")
	}
	if err := database.InitDB(""); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	svc := &ClientService{}
	sizes := []int{5000, 10000, 20000, 50000, 100000, 200000}

	for _, n := range sizes {
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			db := database.GetDB()
			if err := db.Exec("TRUNCATE TABLE inbounds, clients, client_inbounds RESTART IDENTITY CASCADE").Error; err != nil {
				t.Fatalf("truncate: %v", err)
			}

			clients := makeScaleClients(n)
			ib := &model.Inbound{
				Tag:      fmt.Sprintf("scale-%d", n),
				Enable:   true,
				Port:     40000,
				Protocol: model.VLESS,
				Settings: clientsSettings(t, clients),
			}
			if err := db.Create(ib).Error; err != nil {
				t.Fatalf("create inbound: %v", err)
			}

			start := time.Now()
			if err := svc.SyncInbound(nil, ib.Id, clients); err != nil {
				t.Fatalf("seed SyncInbound: %v", err)
			}
			seed := time.Since(start)

			clients[n/2].Enable = !clients[n/2].Enable
			start = time.Now()
			if err := svc.SyncInbound(nil, ib.Id, clients); err != nil {
				t.Fatalf("toggle SyncInbound (new): %v", err)
			}
			toggleNew := time.Since(start)

			start = time.Now()
			if err := svc.SyncInbound(nil, ib.Id, clients); err != nil {
				t.Fatalf("noop SyncInbound (new): %v", err)
			}
			noopNew := time.Since(start)

			toggleOld := time.Duration(0)
			if n <= 10000 {
				clients[n/2].Enable = !clients[n/2].Enable
				start = time.Now()
				if err := syncInboundOld(db, ib.Id, clients); err != nil {
					t.Fatalf("toggle SyncInbound (old): %v", err)
				}
				toggleOld = time.Since(start)
			}

			var linkCount, recCount int64
			db.Model(&model.ClientInbound{}).Where("inbound_id = ?", ib.Id).Count(&linkCount)
			db.Model(&model.ClientRecord{}).Count(&recCount)
			if int(linkCount) != n || int(recCount) != n {
				t.Fatalf("row mismatch: links=%d records=%d want %d", linkCount, recCount, n)
			}

			oldStr, speedup := "skipped", ""
			if toggleOld > 0 {
				oldStr = toggleOld.Round(time.Millisecond).String()
				speedup = fmt.Sprintf("  speedup=%.0fx", float64(toggleOld)/float64(maxDur(toggleNew, time.Millisecond)))
			}
			t.Logf("N=%-7d seed=%-10v toggle_new=%-10v noop_new=%-10v toggle_old=%-10s%s",
				n, seed.Round(time.Millisecond), toggleNew.Round(time.Millisecond),
				noopNew.Round(time.Millisecond), oldStr, speedup)
		})
	}
}

func maxDur(d, floor time.Duration) time.Duration {
	if d < floor {
		return floor
	}
	return d
}

func TestAddDelClientPostgresScale(t *testing.T) {
	if strings.TrimSpace(os.Getenv("XUI_DB_DSN")) == "" || os.Getenv("XUI_DB_TYPE") != "postgres" {
		t.Skip("set XUI_DB_TYPE=postgres and XUI_DB_DSN to run the postgres scale benchmark")
	}
	if err := database.InitDB(""); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	svc := &ClientService{}
	inboundSvc := &InboundService{}
	sizes := []int{5000, 20000, 50000, 100000, 200000}

	for _, n := range sizes {
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			db := database.GetDB()
			if err := db.Exec("TRUNCATE TABLE inbounds, clients, client_inbounds, client_traffics RESTART IDENTITY CASCADE").Error; err != nil {
				t.Fatalf("truncate: %v", err)
			}

			clients := makeScaleClients(n)
			ib := &model.Inbound{
				Tag:      fmt.Sprintf("adddel-%d", n),
				Enable:   true,
				Port:     40000,
				Protocol: model.VLESS,
				Settings: clientsSettings(t, clients),
			}
			if err := db.Create(ib).Error; err != nil {
				t.Fatalf("create inbound: %v", err)
			}
			if err := svc.SyncInbound(nil, ib.Id, clients); err != nil {
				t.Fatalf("seed SyncInbound: %v", err)
			}

			newC := model.Client{
				ID:     uuid.NewString(),
				Email:  "added-client@scale",
				SubID:  "added-sub",
				Enable: true,
			}
			addData := &model.Inbound{Id: ib.Id, Protocol: model.VLESS, Settings: clientsSettings(t, []model.Client{newC})}
			start := time.Now()
			if _, err := svc.AddInboundClient(inboundSvc, addData); err != nil {
				t.Fatalf("AddInboundClient: %v", err)
			}
			addDur := time.Since(start)

			delId := clients[n/2].ID
			start = time.Now()
			if _, err := svc.DelInboundClient(inboundSvc, ib.Id, delId, false); err != nil {
				t.Fatalf("DelInboundClient: %v", err)
			}
			delDur := time.Since(start)

			var recCount, linkCount int64
			db.Model(&model.ClientRecord{}).Count(&recCount)
			db.Model(&model.ClientInbound{}).Where("inbound_id = ?", ib.Id).Count(&linkCount)

			t.Logf("N=%-7d add=%-10v del=%-10v records=%d links=%d", n,
				addDur.Round(time.Millisecond), delDur.Round(time.Millisecond), recCount, linkCount)
		})
	}
}

func TestGroupAndListPostgresScale(t *testing.T) {
	if strings.TrimSpace(os.Getenv("XUI_DB_DSN")) == "" || os.Getenv("XUI_DB_TYPE") != "postgres" {
		t.Skip("set XUI_DB_TYPE=postgres and XUI_DB_DSN to run the postgres scale benchmark")
	}
	if err := database.InitDB(""); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	svc := &ClientService{}
	sizes := []int{5000, 100000}

	for _, n := range sizes {
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			db := database.GetDB()
			if err := db.Exec("TRUNCATE TABLE inbounds, clients, client_inbounds, client_traffics RESTART IDENTITY CASCADE").Error; err != nil {
				t.Fatalf("truncate: %v", err)
			}
			clients := makeScaleClients(n)
			ib := &model.Inbound{Tag: fmt.Sprintf("grp-%d", n), Enable: true, Port: 40000, Protocol: model.VLESS, Settings: clientsSettings(t, clients)}
			if err := db.Create(ib).Error; err != nil {
				t.Fatalf("create inbound: %v", err)
			}
			if err := svc.SyncInbound(nil, ib.Id, clients); err != nil {
				t.Fatalf("seed SyncInbound: %v", err)
			}
			db.Exec("ANALYZE")
			emails := make([]string, n)
			for i := 0; i < n; i++ {
				emails[i] = clients[i].Email
			}

			start := time.Now()
			if _, err := svc.AddToGroup(emails, "benchgroup"); err != nil {
				t.Fatalf("AddToGroup: %v", err)
			}
			addDur := time.Since(start)

			start = time.Now()
			if _, err := svc.RemoveFromGroup(emails); err != nil {
				t.Fatalf("RemoveFromGroup: %v", err)
			}
			rmDur := time.Since(start)

			start = time.Now()
			list, err := svc.List()
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			listDur := time.Since(start)
			if len(list) != n {
				t.Fatalf("List returned %d, want %d", len(list), n)
			}

			t.Logf("N=%-7d bulkAdd=%-9v bulkRemove=%-9v list=%-9v", n,
				addDur.Round(time.Millisecond), rmDur.Round(time.Millisecond), listDur.Round(time.Millisecond))
		})
	}
}

func TestDelAllClientsPostgresScale(t *testing.T) {
	if strings.TrimSpace(os.Getenv("XUI_DB_DSN")) == "" || os.Getenv("XUI_DB_TYPE") != "postgres" {
		t.Skip("set XUI_DB_TYPE=postgres and XUI_DB_DSN to run the postgres scale benchmark")
	}
	if err := database.InitDB(""); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	svc := &ClientService{}
	inboundSvc := &InboundService{}
	sizes := []int{5000, 50000, 100000}

	for _, n := range sizes {
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			db := database.GetDB()
			if err := db.Exec("TRUNCATE TABLE inbounds, clients, client_inbounds, client_traffics RESTART IDENTITY CASCADE").Error; err != nil {
				t.Fatalf("truncate: %v", err)
			}
			clients := makeScaleClients(n)
			ib := &model.Inbound{Tag: fmt.Sprintf("delall-%d", n), Enable: true, Port: 40000, Protocol: model.VLESS, Settings: clientsSettings(t, clients)}
			if err := db.Create(ib).Error; err != nil {
				t.Fatalf("create inbound: %v", err)
			}
			if err := svc.SyncInbound(nil, ib.Id, clients); err != nil {
				t.Fatalf("seed SyncInbound: %v", err)
			}

			emails, err := inboundSvc.EmailsByInbound(ib.Id)
			if err != nil {
				t.Fatalf("EmailsByInbound: %v", err)
			}
			start := time.Now()
			res, _, err := svc.BulkDelete(inboundSvc, emails, false)
			if err != nil {
				t.Fatalf("BulkDelete: %v", err)
			}
			dur := time.Since(start)

			var recCount, linkCount int64
			db.Model(&model.ClientRecord{}).Count(&recCount)
			db.Model(&model.ClientInbound{}).Where("inbound_id = ?", ib.Id).Count(&linkCount)
			if recCount != 0 || linkCount != 0 {
				t.Fatalf("after delAll: records=%d links=%d want 0/0", recCount, linkCount)
			}
			t.Logf("N=%-7d delAllClients=%-10v deleted=%d", n, dur.Round(time.Millisecond), res.Deleted)
		})
	}
}

func TestBulkOpsPostgresScale(t *testing.T) {
	if strings.TrimSpace(os.Getenv("XUI_DB_DSN")) == "" || os.Getenv("XUI_DB_TYPE") != "postgres" {
		t.Skip("set XUI_DB_TYPE=postgres and XUI_DB_DSN to run the postgres scale benchmark")
	}
	if err := database.InitDB(""); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	svc := &ClientService{}
	inboundSvc := &InboundService{}
	sizes := []int{5000, 20000, 50000, 100000}
	const m = 2000

	for _, n := range sizes {
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			db := database.GetDB()
			if err := db.Exec("TRUNCATE TABLE inbounds, clients, client_inbounds, client_traffics RESTART IDENTITY CASCADE").Error; err != nil {
				t.Fatalf("truncate: %v", err)
			}

			clients := makeScaleClients(n)
			exp := time.Now().AddDate(1, 0, 0).UnixMilli()
			for i := range clients {
				clients[i].ExpiryTime = exp
				clients[i].TotalGB = 100 << 30
			}
			ib := &model.Inbound{Tag: fmt.Sprintf("bulk-%d", n), Enable: true, Port: 40000, Protocol: model.VLESS, Settings: clientsSettings(t, clients)}
			if err := db.Create(ib).Error; err != nil {
				t.Fatalf("create inbound: %v", err)
			}
			if err := svc.SyncInbound(nil, ib.Id, clients); err != nil {
				t.Fatalf("seed SyncInbound: %v", err)
			}
			ib2 := &model.Inbound{Tag: fmt.Sprintf("bulk2-%d", n), Enable: true, Port: 40001, Protocol: model.VLESS, Settings: `{"clients":[]}`}
			if err := db.Create(ib2).Error; err != nil {
				t.Fatalf("create inbound2: %v", err)
			}

			emailsM := make([]string, m)
			for i := 0; i < m; i++ {
				emailsM[i] = clients[i].Email
			}

			t0 := time.Now()
			if _, _, err := svc.BulkAdjust(inboundSvc, emailsM, 7, 1<<30); err != nil {
				t.Fatalf("BulkAdjust: %v", err)
			}
			adjustDur := time.Since(t0)

			t0 = time.Now()
			if _, _, err := svc.BulkAttach(inboundSvc, emailsM, []int{ib2.Id}); err != nil {
				t.Fatalf("BulkAttach: %v", err)
			}
			attachDur := time.Since(t0)

			t0 = time.Now()
			if _, _, err := svc.BulkDetach(inboundSvc, emailsM, []int{ib2.Id}); err != nil {
				t.Fatalf("BulkDetach: %v", err)
			}
			detachDur := time.Since(t0)

			payloads := make([]ClientCreatePayload, m)
			for i := 0; i < m; i++ {
				payloads[i] = ClientCreatePayload{
					Client:     model.Client{ID: uuid.NewString(), Email: fmt.Sprintf("bulknew-%07d@scale", i), SubID: fmt.Sprintf("bnsub-%07d", i), Enable: true},
					InboundIds: []int{ib.Id},
				}
			}
			t0 = time.Now()
			if _, _, err := svc.BulkCreate(inboundSvc, payloads); err != nil {
				t.Fatalf("BulkCreate: %v", err)
			}
			createDur := time.Since(t0)

			t0 = time.Now()
			if _, _, err := svc.BulkDelete(inboundSvc, emailsM, false); err != nil {
				t.Fatalf("BulkDelete: %v", err)
			}
			deleteDur := time.Since(t0)

			t.Logf("N=%-6d M=%d adjust=%-9v attach=%-9v detach=%-9v create=%-9v delete=%-9v", n, m,
				adjustDur.Round(time.Millisecond), attachDur.Round(time.Millisecond), detachDur.Round(time.Millisecond),
				createDur.Round(time.Millisecond), deleteDur.Round(time.Millisecond))
		})
	}
}
