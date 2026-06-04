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

			var recCount int64
			db.Model(&model.ClientRecord{}).Count(&recCount)
			if int(recCount) != n {
				t.Fatalf("record count after add+del = %d, want %d", recCount, n)
			}

			t.Logf("N=%-7d add=%-10v del=%-10v", n, addDur.Round(time.Millisecond), delDur.Round(time.Millisecond))
		})
	}
}
