package database

import (
	"fmt"
	"log"

	"github.com/mhsanaei/3x-ui/v3/database/model"
	"gorm.io/gorm"
)

// MigrateIPLimit creates the inbound_client_ips table
func MigrateIPLimit(db *gorm.DB) error {
	log.Println("[DB] Migrating IP limit table...")

	type InboundClientIPs struct {
		Id          int   `gorm:"primaryKey;autoIncrement"`
		ClientEmail string `gorm:"unique"`
		IPs         string `gorm:"type:text"`
		CreatedAt   int64
		UpdatedAt   int64
	}

	if !db.Migrator().HasTable(&InboundClientIPs{}) {
		if err := db.Migrator().CreateTable(&InboundClientIPs{}); err != nil {
			return fmt.Errorf("failed to create inbound_client_ips table: %w", err)
		}
		log.Println("[DB] Created inbound_client_ips table")
	}

	return nil
}
