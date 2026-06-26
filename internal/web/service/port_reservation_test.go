package service

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"gorm.io/gorm"
)

// setupPortReservationDB makes every H-04 test hermetic on the shared scratch
// PostgreSQL DSN. SQLite already gets a fresh file from setupConflictDB. The PG
// suite intentionally reuses one database, so both authoritative rows and the
// additive schema must be reset before and after each test.
func setupPortReservationDB(t *testing.T) {
	t.Helper()
	setupConflictDB(t)
	if !database.IsPostgres() {
		return
	}
	reset := func() {
		db := database.GetDB()
		if db == nil {
			return
		}
		if err := db.Exec("DROP TABLE IF EXISTS inbound_port_reservations").Error; err != nil {
			t.Fatalf("drop port reservations: %v", err)
		}
		if err := db.Exec("TRUNCATE TABLE inbounds RESTART IDENTITY CASCADE").Error; err != nil {
			t.Fatalf("truncate inbounds: %v", err)
		}
	}
	reset()
	t.Cleanup(reset)
}

func TestPortReservationsGateOffDoesNotCreateSchema(t *testing.T) {
	t.Setenv(portReservationsGateEnv, "")
	setupPortReservationDB(t)
	if err := PrepareInboundPortReservations(); err != nil {
		t.Fatalf("prepare gate off: %v", err)
	}
	if database.GetDB().Migrator().HasTable(portReservation{}) {
		t.Fatal("gate-off boot must not create the reservation table")
	}
}

func TestPortReservationsPostgresConcurrentSamePortSingleWinner(t *testing.T) {
	if os.Getenv("XUI_DB_TYPE") != "postgres" || strings.TrimSpace(os.Getenv("XUI_DB_DSN")) == "" {
		t.Skip("set XUI_DB_TYPE=postgres and XUI_DB_DSN to run port-reservation concurrency test")
	}
	t.Setenv(portReservationsGateEnv, "1")
	setupPortReservationDB(t)
	if err := PrepareInboundPortReservations(); err != nil {
		t.Fatal(err)
	}

	basePort := 30000 + int(time.Now().UnixNano()%19000)
	cases := []struct {
		name       string
		first      model.Inbound
		second     model.Inbound
		wantCommit int
	}{
		{
			name:       "wildcard conflicts with specific",
			first:      model.Inbound{Listen: "", Protocol: model.VLESS, StreamSettings: `{"network":"tcp"}`},
			second:     model.Inbound{Listen: "127.0.0.1", Protocol: model.Trojan, StreamSettings: `{"network":"tcp"}`},
			wantCommit: 1,
		},
		{
			name:       "distinct specific listeners coexist",
			first:      model.Inbound{Listen: "127.0.0.1", Protocol: model.VLESS, StreamSettings: `{"network":"tcp"}`},
			second:     model.Inbound{Listen: "127.0.0.2", Protocol: model.Trojan, StreamSettings: `{"network":"tcp"}`},
			wantCommit: 2,
		},
		{
			name:       "tcp and udp coexist",
			first:      model.Inbound{Listen: "", Protocol: model.VLESS, StreamSettings: `{"network":"tcp"}`},
			second:     model.Inbound{Listen: "", Protocol: model.Hysteria},
			wantCommit: 2,
		},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			port := basePort + i
			inbounds := []model.Inbound{tc.first, tc.second}
			for j := range inbounds {
				inbounds[j].Port = port
				inbounds[j].Tag = fmt.Sprintf("h04-race-%d-%d-%d", time.Now().UnixNano(), i, j)
			}
			t.Cleanup(func() {
				_ = database.GetDB().Where("port = ?", port).Delete(&portReservation{}).Error
				_ = database.GetDB().Where("port = ? AND tag LIKE ?", port, "h04-race-%").Delete(&model.Inbound{}).Error
			})
			start := make(chan struct{})
			errs := make(chan error, 2)
			var wg sync.WaitGroup
			for j := range inbounds {
				wg.Add(1)
				go func(inbound model.Inbound) {
					defer wg.Done()
					<-start
					err := database.GetDB().Transaction(func(tx *gorm.DB) error {
						key := portLockKey{nodeScope: 0, port: port}
						if err := lockPortReservationKeysTx(tx, key); err != nil {
							return err
						}
						if err := tx.Create(&inbound).Error; err != nil {
							return err
						}
						return reserveInboundPortsTx(tx, &inbound, inbound.Id)
					})
					errs <- err
				}(inbounds[j])
			}
			close(start)
			wg.Wait()
			close(errs)
			commits := 0
			for err := range errs {
				if err == nil {
					commits++
				}
			}
			if commits != tc.wantCommit {
				t.Fatalf("successful concurrent commits=%d, want %d", commits, tc.wantCommit)
			}
		})
	}
}

func TestPortReservationsBackfillAndIdempotentRerun(t *testing.T) {
	t.Setenv(portReservationsGateEnv, "1")
	setupPortReservationDB(t)
	seedInboundConflict(t, "tcp", "127.0.0.1", 443, model.VLESS, `{"network":"tcp"}`, `{}`)
	seedInboundConflict(t, "udp", "127.0.0.1", 443, model.Hysteria, ``, `{}`)

	for i := 0; i < 2; i++ {
		if err := PrepareInboundPortReservations(); err != nil {
			t.Fatalf("prepare pass %d: %v", i+1, err)
		}
	}
	var count int64
	if err := database.GetDB().Model(&portReservation{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("reservation count=%d, want 2", count)
	}
}

func TestPortReservationsBackfillRejectsSemanticConflict(t *testing.T) {
	t.Setenv(portReservationsGateEnv, "1")
	setupPortReservationDB(t)
	seedInboundConflict(t, "wildcard", "", 8443, model.VLESS, `{"network":"tcp"}`, `{}`)
	seedInboundConflict(t, "specific", "127.0.0.1", 8443, model.Trojan, `{"network":"tcp"}`, `{}`)

	if err := PrepareInboundPortReservations(); err == nil {
		t.Fatal("conflicting backfill must fail closed")
	}
	var count int64
	if err := database.GetDB().Model(&portReservation{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("failed backfill left %d reservations", count)
	}
}

func TestPortReservationMutationMatrix(t *testing.T) {
	t.Setenv(portReservationsGateEnv, "1")
	setupPortReservationDB(t)
	if err := PrepareInboundPortReservations(); err != nil {
		t.Fatal(err)
	}

	db := database.GetDB()
	wildcard := &model.Inbound{Tag: "wild", Listen: "", Port: 9443, Protocol: model.VLESS, StreamSettings: `{"network":"tcp"}`}
	if err := db.Transaction(func(tx *gorm.DB) error { return reserveInboundPortsTx(tx, wildcard, 0) }); err != nil {
		t.Fatalf("reserve wildcard: %v", err)
	}
	specificTCP := &model.Inbound{Tag: "specific", Listen: "127.0.0.1", Port: 9443, Protocol: model.Trojan, StreamSettings: `{"network":"tcp"}`}
	if err := db.Transaction(func(tx *gorm.DB) error { return reserveInboundPortsTx(tx, specificTCP, 0) }); err == nil {
		t.Fatal("wildcard and specific TCP must conflict")
	}
	specificUDP := &model.Inbound{Tag: "udp", Listen: "127.0.0.1", Port: 9443, Protocol: model.Hysteria}
	if err := db.Transaction(func(tx *gorm.DB) error { return reserveInboundPortsTx(tx, specificUDP, 0) }); err != nil {
		t.Fatalf("TCP and UDP must coexist: %v", err)
	}
}

func TestPortReservationsRejectWrongIndexShape(t *testing.T) {
	t.Setenv(portReservationsGateEnv, "1")
	setupPortReservationDB(t)
	db := database.GetDB()
	if err := db.Exec(`CREATE TABLE inbound_port_reservations (
		inbound_id integer NOT NULL, node_scope integer NOT NULL,
		listen text NOT NULL, port integer NOT NULL, transport integer NOT NULL
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX ux_inbound_port_reservation_exact
		ON inbound_port_reservations (node_scope, port, transport)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX ux_inbound_port_reservation_owner_transport
		ON inbound_port_reservations (inbound_id, transport)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := PrepareInboundPortReservations(); err == nil {
		t.Fatal("wrong named-index shape must fail closed")
	}
}

func TestPortReservationRollsBackWithMutation(t *testing.T) {
	t.Setenv(portReservationsGateEnv, "1")
	setupPortReservationDB(t)
	if err := PrepareInboundPortReservations(); err != nil {
		t.Fatal(err)
	}
	inbound := &model.Inbound{Tag: "rollback", Port: 10443, Protocol: model.VLESS, StreamSettings: `{"network":"tcp"}`}
	wantErr := errors.New("injected commit failure")
	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(inbound).Error; err != nil {
			return err
		}
		if err := reserveInboundPortsTx(tx, inbound, inbound.Id); err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("transaction error=%v, want injected failure", err)
	}
	var inbounds, reservations int64
	if err := database.GetDB().Model(&model.Inbound{}).Where("tag = ?", "rollback").Count(&inbounds).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Model(&portReservation{}).Count(&reservations).Error; err != nil {
		t.Fatal(err)
	}
	if inbounds != 0 || reservations != 0 {
		t.Fatalf("rollback leaked inbounds=%d reservations=%d", inbounds, reservations)
	}
}

func TestPortReservationFollowsInboundAddUpdateDelete(t *testing.T) {
	t.Setenv(portReservationsGateEnv, "1")
	setupPortReservationDB(t)
	if err := PrepareInboundPortReservations(); err != nil {
		t.Fatal(err)
	}

	inbound := &model.Inbound{
		Tag:            "h04-lifecycle",
		Enable:         false, // keep this test DB-only; no runtime side effects
		Port:           11443,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp"}`,
		Settings:       `{"clients":[]}`,
	}
	if _, _, err := (&InboundService{}).AddInbound(inbound); err != nil {
		t.Fatalf("add: %v", err)
	}
	assertReservationPort := func(wantPort int, wantCount int64) {
		t.Helper()
		var rows []portReservation
		if err := database.GetDB().Where("inbound_id = ?", inbound.Id).Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		if int64(len(rows)) != wantCount {
			t.Fatalf("reservation count=%d, want %d", len(rows), wantCount)
		}
		if wantCount > 0 && rows[0].Port != wantPort {
			t.Fatalf("reservation port=%d, want %d", rows[0].Port, wantPort)
		}
	}
	assertReservationPort(11443, 1)

	inbound.Port = 12443
	if _, _, err := (&InboundService{}).UpdateInbound(inbound); err != nil {
		t.Fatalf("update: %v", err)
	}
	assertReservationPort(12443, 1)

	if _, err := (&InboundService{}).DelInbound(inbound.Id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	assertReservationPort(0, 0)
}
