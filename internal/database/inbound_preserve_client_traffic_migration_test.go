package database

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestMigrateInboundPreserveClientTrafficColumn(t *testing.T) {
	initMigrateDB(t)
	tx := GetDB().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })

	// Recreate the previous schema in this transaction on SQLite or PostgreSQL.
	if err := tx.Exec("ALTER TABLE inbounds DROP COLUMN preserve_client_traffic").Error; err != nil {
		t.Fatal(err)
	}
	const tag = "legacy-preserve-client-traffic"
	if err := tx.Exec("INSERT INTO inbounds (tag, up, down, traffic_reset, traffic_reset_day) VALUES (?, 30, 50, 'monthly', 20)", tag).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateInboundPreserveClientTrafficColumn(tx); err != nil {
		t.Fatal(err)
	}
	var stored model.Inbound
	if err := tx.Where("tag = ?", tag).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.PreserveClientTraffic == nil || *stored.PreserveClientTraffic {
		t.Fatal("legacy inbound must have a non-null, false preserve_client_traffic")
	}
	if stored.Up != 30 || stored.Down != 50 || stored.TrafficReset != "monthly" || stored.TrafficResetDay != 20 {
		t.Fatalf("legacy inbound after migration: up=%d down=%d schedule=%s day=%d", stored.Up, stored.Down, stored.TrafficReset, stored.TrafficResetDay)
	}
	if err := tx.Model(&stored).Update("preserve_client_traffic", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateInboundPreserveClientTrafficColumn(tx); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	if err := tx.First(&stored, stored.Id).Error; err != nil || !stored.PreservesClientTraffic() {
		t.Fatalf("repeat migration lost the opt-in: preserve=%t, err=%v", stored.PreservesClientTraffic(), err)
	}
}
