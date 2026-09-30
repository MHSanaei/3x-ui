package service

import (
	"errors"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// AddTuicTrafficBatch commits a journal batch and its receipt atomically. A
// retained journal can therefore be replayed after an ambiguous commit result.
func (s *InboundService) AddTuicTrafficBatch(id string, traffic []*xray.ClientTraffic) error {
	if id == "" {
		return errors.New("TUIC traffic batch has no ID")
	}
	return submitTrafficWrite(func() error {
		return database.GetDB().Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("CREATE TABLE IF NOT EXISTS tuic_traffic_receipts (id TEXT PRIMARY KEY)").Error; err != nil {
				return err
			}
			result := tx.Exec("INSERT INTO tuic_traffic_receipts (id) VALUES (?) ON CONFLICT(id) DO NOTHING", id)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return nil
			}
			return s.addClientTraffic(tx, traffic)
		})
	})
}
