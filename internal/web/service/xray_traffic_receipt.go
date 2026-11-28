package service

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/outbound"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// AddXrayTrafficBatch accounts for all parts of one immutable Xray sample.
// The poller retries only its outstanding sample and advances after confirmation.
func (s *InboundService) AddXrayTrafficBatch(batchID string, traffics []*xray.Traffic, clients []*xray.ClientTraffic) (bool, bool, error) {
	if batchID == "" {
		return false, false, errors.New("Xray traffic batch has no ID")
	}
	return s.addTraffic(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"batch_id"}),
			Where: clause.Where{Exprs: []clause.Expression{
				clause.Neq{Column: clause.Column{Table: clause.CurrentTable, Name: "batch_id"}, Value: batchID},
			}},
		}).Create(&model.XrayTrafficReceipt{ID: 1, BatchID: batchID})
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		if err := s.accountInboundTraffic(tx, traffics, clients); err != nil {
			return err
		}
		return (&outbound.OutboundService{}).AddTrafficTx(tx, traffics)
	})
}
