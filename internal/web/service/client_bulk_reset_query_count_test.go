package service

import (
	"fmt"
	"sync/atomic"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// countClientTableQueries runs fn with a callback counting SELECTs against the
// clients table, so a per-email lookup shows up as growth with the batch size.
func countClientTableQueries(t *testing.T, name string, fn func()) int {
	t.Helper()
	db := database.GetDB()
	var n int64
	cb := "test:count_clients_query_" + name
	if err := db.Callback().Query().Before("gorm:query").Register(cb, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "clients" {
			atomic.AddInt64(&n, 1)
		}
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	defer func() {
		if err := db.Callback().Query().Remove(cb); err != nil {
			t.Errorf("remove query callback: %v", err)
		}
	}()
	fn()
	return int(atomic.LoadInt64(&n))
}

func seedEnabledClientsForReset(t *testing.T, svc *ClientService, port int, n int, prefix string) []string {
	t.Helper()
	clients := make([]model.Client, 0, n)
	for i := range n {
		email := fmt.Sprintf("%s-%d@x", prefix, i)
		clients = append(clients, model.Client{
			Email:  email,
			ID:     fmt.Sprintf("%08d-1111-1111-1111-111111111111", i),
			SubID:  email,
			Enable: true,
		})
	}
	ib := mkInbound(t, port, model.VLESS, clientsSettings(t, clients))
	if err := svc.SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("seed linkage: %v", err)
	}
	emails := make([]string, 0, n)
	for _, c := range clients {
		mkTraffic(t, ib.Id, c.Email, 100, 200, 0, 0, true)
		emails = append(emails, c.Email)
	}
	return emails
}

// TestBulkResetTraffic_DoesNotQueryPerEmail pins BulkResetTraffic's client
// lookup to a batched read: the number of SELECTs on clients must not grow
// with the number of emails reset.
func TestBulkResetTraffic_DoesNotQueryPerEmail(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	few := seedEnabledClientsForReset(t, svc, 53010, 3, "few")
	many := seedEnabledClientsForReset(t, svc, 53011, 30, "many")

	fewCount := countClientTableQueries(t, "few", func() {
		if _, err := svc.BulkResetTraffic(inboundSvc, few); err != nil {
			t.Fatalf("BulkResetTraffic(few): %v", err)
		}
	})
	manyCount := countClientTableQueries(t, "many", func() {
		if _, err := svc.BulkResetTraffic(inboundSvc, many); err != nil {
			t.Fatalf("BulkResetTraffic(many): %v", err)
		}
	})

	if manyCount != fewCount {
		t.Fatalf("clients SELECTs: %d emails -> %d, %d emails -> %d; want the same batched count",
			len(few), fewCount, len(many), manyCount)
	}
}
