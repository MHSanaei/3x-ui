package service

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const portReservationsGateEnv = "XUI_ENFORCE_PORT_RESERVATIONS"

// 0x5855 is the fixed "XU" advisory-lock namespace. The remaining bits hold
// node_scope (31 bits) and port (16 bits), so valid keys never hash-collide.
const portReservationLockNamespace uint64 = 0x5855

func portReservationsEnabled() bool {
	return strings.TrimSpace(os.Getenv(portReservationsGateEnv)) == "1"
}

func canonicalReservationListen(listen string) string {
	listen = strings.TrimSpace(listen)
	if isAnyListen(listen) {
		return "*"
	}
	return listen
}

func inboundNodeScope(inbound *model.Inbound) int {
	if inbound.NodeID == nil {
		return 0
	}
	return *inbound.NodeID
}

func reservationRows(inbound *model.Inbound) []model.InboundPortReservation {
	bits := inboundTransports(inbound.Protocol, inbound.StreamSettings, inbound.Settings)
	rows := make([]model.InboundPortReservation, 0, 2)
	for _, transport := range []transportBits{transportTCP, transportUDP} {
		if bits&transport != 0 {
			rows = append(rows, model.InboundPortReservation{
				InboundID: inbound.Id, NodeScope: inboundNodeScope(inbound),
				Listen: canonicalReservationListen(inbound.Listen), Port: inbound.Port,
				Transport: uint8(transport),
			})
		}
	}
	return rows
}

// PrepareInboundPortReservations backfills the migrated reservation schema
// in one fail-closed transaction when enforcement is enabled.
func PrepareInboundPortReservations() error {
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("database is not initialized")
	}
	if !portReservationsEnabled() {
		return nil
	}
	if err := validatePortReservationIndexes(db); err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.InboundPortReservation{}).Error; err != nil {
			return err
		}
		var inbounds []*model.Inbound
		if err := tx.Order("COALESCE(node_id, 0), port, id").Find(&inbounds).Error; err != nil {
			return err
		}
		for _, inbound := range inbounds {
			if err := reserveInboundPortsTx(tx, inbound, inbound.Id); err != nil {
				return fmt.Errorf("backfill inbound %d: %w", inbound.Id, err)
			}
		}
		var got int64
		if err := tx.Model(&model.InboundPortReservation{}).Count(&got).Error; err != nil {
			return err
		}
		want := int64(0)
		for _, inbound := range inbounds {
			want += int64(len(reservationRows(inbound)))
		}
		if got != want {
			return fmt.Errorf("port-reservation verification failed: got %d rows, want %d", got, want)
		}
		return nil
	})
}

func validatePortReservationIndexes(db *gorm.DB) error {
	expected := map[string][]string{
		"ux_inbound_port_reservation_exact":           {"node_scope", "listen", "port", "transport"},
		"ux_inbound_port_reservation_owner_transport": {"inbound_id", "transport"},
	}
	indexes, err := db.Migrator().GetIndexes(&model.InboundPortReservation{})
	if err != nil {
		return fmt.Errorf("inspect port-reservation indexes: %w", err)
	}
	for name, columns := range expected {
		valid := false
		for _, index := range indexes {
			if index.Name() != name || !sameStringSlice(index.Columns(), columns) {
				continue
			}
			unique, ok := index.Unique()
			valid = ok && unique
			break
		}
		if !valid {
			return fmt.Errorf("port-reservation index %s is missing or has the wrong shape", name)
		}
	}
	return nil
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func reserveInboundPortsTx(tx *gorm.DB, inbound *model.Inbound, ignoreID int) error {
	if !portReservationsEnabled() {
		return nil
	}
	conflict, err := checkPortConflictTx(tx, inbound, ignoreID)
	if err != nil {
		return err
	}
	if conflict != nil {
		return fmt.Errorf("%s", conflict.String())
	}
	for _, row := range reservationRows(inbound) {
		if err := tx.Create(&row).Error; err != nil {
			return fmt.Errorf("reserve port %d/%s: %w", inbound.Port, transportTagSuffix(transportBits(row.Transport)), err)
		}
	}
	return nil
}

func replaceInboundPortReservationsTx(tx *gorm.DB, inbound *model.Inbound) error {
	if !portReservationsEnabled() {
		return nil
	}
	if err := tx.Where("inbound_id = ?", inbound.Id).Delete(&model.InboundPortReservation{}).Error; err != nil {
		return err
	}
	return reserveInboundPortsTx(tx, inbound, inbound.Id)
}

func deleteInboundPortReservationsTx(tx *gorm.DB, inboundID int) error {
	if !portReservationsEnabled() {
		return nil
	}
	return tx.Where("inbound_id = ?", inboundID).Delete(&model.InboundPortReservation{}).Error
}

type portLockKey struct{ nodeScope, port int }

type keyedLockEntry struct {
	mu   sync.Mutex
	refs int
}

var portReservationProcessLocks = struct {
	sync.Mutex
	locks map[portLockKey]*keyedLockEntry
}{locks: make(map[portLockKey]*keyedLockEntry)}

func lockPortReservationKeys(keys ...portLockKey) func() {
	if !portReservationsEnabled() || database.IsPostgres() {
		return func() {}
	}
	keys = uniqueSortedPortLockKeys(keys)
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].nodeScope != keys[j].nodeScope {
			return keys[i].nodeScope < keys[j].nodeScope
		}
		return keys[i].port < keys[j].port
	})
	entries := make([]*keyedLockEntry, 0, len(keys))
	for _, key := range keys {
		portReservationProcessLocks.Lock()
		entry := portReservationProcessLocks.locks[key]
		if entry == nil {
			entry = &keyedLockEntry{}
			portReservationProcessLocks.locks[key] = entry
		}
		entry.refs++
		portReservationProcessLocks.Unlock()
		entry.mu.Lock()
		entries = append(entries, entry)
	}
	return func() {
		for i := len(entries) - 1; i >= 0; i-- {
			entries[i].mu.Unlock()
		}
		for i, key := range keys {
			portReservationProcessLocks.Lock()
			entry := entries[i]
			entry.refs--
			if entry.refs == 0 && portReservationProcessLocks.locks[key] == entry {
				delete(portReservationProcessLocks.locks, key)
			}
			portReservationProcessLocks.Unlock()
		}
	}
}

func lockPortReservationKeysTx(tx *gorm.DB, keys ...portLockKey) error {
	if !portReservationsEnabled() || !database.IsPostgres() {
		return nil
	}
	keys = uniqueSortedPortLockKeys(keys)
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].nodeScope != keys[j].nodeScope {
			return keys[i].nodeScope < keys[j].nodeScope
		}
		return keys[i].port < keys[j].port
	})
	for _, key := range keys {
		if key.nodeScope < 0 || key.nodeScope > math.MaxInt32 || key.port < 0 || key.port > math.MaxUint16 {
			return fmt.Errorf("port-reservation lock key out of range: node=%d port=%d", key.nodeScope, key.port)
		}
		advisoryKey := int64((portReservationLockNamespace << 47) | (uint64(key.nodeScope) << 16) | uint64(key.port))
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", advisoryKey).Error; err != nil {
			return err
		}
	}
	return nil
}

func uniqueSortedPortLockKeys(keys []portLockKey) []portLockKey {
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].nodeScope != keys[j].nodeScope {
			return keys[i].nodeScope < keys[j].nodeScope
		}
		return keys[i].port < keys[j].port
	})
	out := keys[:0]
	for _, key := range keys {
		if len(out) == 0 || out[len(out)-1] != key {
			out = append(out, key)
		}
	}
	return out
}
