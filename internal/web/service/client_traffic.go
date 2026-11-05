package service

import (
	"slices"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
)

func (s *ClientService) ResetTrafficByEmail(inboundSvc *InboundService, email string) (bool, error) {
	if email == "" {
		return false, common.NewError("client email is required")
	}
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return false, err
	}
	inboundIds, err := s.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		return false, err
	}
	// Decided before the resets below zero the counters the rule reads.
	quotaDisabled, err := quotaDisabledEmails(database.GetDB(), []string{email})
	if err != nil {
		return false, err
	}

	needRestart := false
	if len(inboundIds) == 0 {
		if rErr := inboundSvc.ResetClientTrafficByEmail(email); rErr != nil {
			return false, rErr
		}
	} else {
		applies := make([]inboundApply, 0, len(inboundIds))
		for _, ibId := range inboundIds {
			applies = append(applies, inboundApply{id: ibId, run: func() (bool, error) {
				return inboundSvc.ResetClientTraffic(ibId, email)
			}})
		}
		nr, applyErr := fanoutInboundApplies(applies)
		if applyErr != nil {
			return nr, applyErr
		}
		needRestart = nr
	}

	// Restore quota-disabled clients after zeroing; preserve an operator
	// disable, including when no inbound remains.
	if !rec.Enable && len(quotaDisabled) > 0 {
		// Inbound resets already restored hosted clients. Only an unhosted
		// record still needs the full update.
		fresh, fErr := s.GetRecordByEmail(nil, email)
		if fErr == nil && !fresh.Enable {
			updated := fresh.ToClient()
			updated.Enable = true
			nr, uErr := s.Update(inboundSvc, fresh.Id, *updated, fresh.LimitHwid)
			if uErr != nil {
				logger.Warning("Failed to auto-enable client during traffic reset:", uErr)
			}
			if nr {
				needRestart = true
			}
		}
	}
	return needRestart, nil
}

func (s *ClientService) BulkResetTraffic(inboundSvc *InboundService, emails []string) (int, error) {
	if len(emails) == 0 {
		return 0, nil
	}
	cleanEmails := trimmedUniqueEmails(emails)
	if len(cleanEmails) == 0 {
		return 0, nil
	}

	recordsByEmail, err := clientRecordsByEmail(nil, cleanEmails)
	if err != nil {
		return 0, err
	}
	var affected int64
	var restore []string
	var resetNodes []int
	err = submitTrafficWrite(func() error {
		db := database.GetDB()
		return db.Transaction(func(tx *gorm.DB) error {
			var err error
			if restore, err = quotaDisabledEmails(tx, cleanEmails); err != nil {
				return err
			}
			keepOff, err := handDisabledEmails(tx, cleanEmails)
			if err != nil {
				return err
			}
			if err := adjustGroupBaselinesForRemovedTraffic(tx, cleanEmails); err != nil {
				return err
			}
			if affected, err = zeroClientTraffics(tx, cleanEmails, keepOff); err != nil {
				return err
			}
			if err := clearGlobalTraffic(tx, cleanEmails...); err != nil {
				return err
			}
			for _, batch := range chunkStrings(cleanEmails, sqlInChunk) {
				if err := tx.Where("email IN ?", batch).Delete(&model.NodeClientTraffic{}).Error; err != nil {
					return err
				}
			}
			var qErr error
			resetNodes, qErr = queueNodeResets(tx, cleanEmails)
			return qErr
		})
	})
	if err != nil {
		return 0, err
	}
	inboundSvc.resetMtprotoClientQuotas(cleanEmails)
	inboundSvc.deliverNodeResetsNow(resetNodes)
	// After the zeroing, as in ResetTrafficByEmail: enabling a still-depleted
	// client first lets the next traffic tick switch it off again.
	for _, e := range restore {
		rec := recordsByEmail[e]
		if rec == nil || rec.Enable {
			continue
		}
		updated := rec.ToClient()
		updated.Enable = true
		if _, uErr := s.Update(inboundSvc, rec.Id, *updated, rec.LimitHwid); uErr != nil {
			logger.Warning("Failed to auto-enable client during bulk traffic reset:", uErr)
		}
	}
	return int(affected), nil
}

func (s *ClientService) ResetAllClientTraffics(inboundSvc *InboundService, id int) (bool, error) {
	var resetNodes []int
	var resetEmails, restore []string
	err := submitTrafficWrite(func() error {
		var inner error
		resetEmails, restore, resetNodes, inner = s.resetAllClientTrafficsLocked(id)
		return inner
	})
	if err != nil {
		return false, err
	}
	inboundSvc.resetMtprotoClientQuotas(resetEmails)
	inboundSvc.deliverNodeResetsNow(resetNodes)
	needRestart := false
	if len(restore) > 0 {
		var enableErr error
		if _, needRestart, enableErr = s.BulkSetEnable(inboundSvc, restore, true); enableErr != nil {
			logger.Warning("Failed to re-enable depleted clients after resetting inbound traffic:", enableErr)
		}
	}
	return needRestart, nil
}

func (s *ClientService) resetAllClientTrafficsLocked(id int) ([]string, []string, []int, error) {
	db := database.GetDB()
	now := time.Now().Unix() * 1000
	var resetNodes []int
	var reset, restore []string

	if err := db.Transaction(func(tx *gorm.DB) error {
		// client_traffics.inbound_id is stale: it reflects the inbound the row was
		// first inserted under and is never refreshed. Use the client_inbounds join
		// as the authoritative source for which emails belong to a given inbound.
		var resetEmails []string
		if id == -1 {
			if err := tx.Model(xray.ClientTraffic{}).Pluck("email", &resetEmails).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Table("client_inbounds ci").
				Select("c.email").
				Joins("JOIN clients c ON c.id = ci.client_id").
				Where("ci.inbound_id = ?", id).
				Pluck("c.email", &resetEmails).Error; err != nil {
				return err
			}
		}
		if len(resetEmails) == 0 {
			return nil
		}
		reset = resetEmails

		var qErr error
		if restore, qErr = quotaDisabledEmails(tx, resetEmails); qErr != nil {
			return qErr
		}
		keepOff, qErr := handDisabledEmails(tx, resetEmails)
		if qErr != nil {
			return qErr
		}

		if err := adjustGroupBaselinesForRemovedTraffic(tx, resetEmails); err != nil {
			return err
		}

		if _, err := zeroClientTraffics(tx, resetEmails, keepOff); err != nil {
			return err
		}

		if err := clearGlobalTraffic(tx, resetEmails...); err != nil {
			return err
		}

		for _, batch := range chunkStrings(resetEmails, sqlInChunk) {
			if err := tx.Where("email IN ?", batch).Delete(&model.NodeClientTraffic{}).Error; err != nil {
				return err
			}
		}
		if resetNodes, qErr = queueNodeResets(tx, resetEmails); qErr != nil {
			return qErr
		}

		inboundWhereText := "id "
		if id == -1 {
			inboundWhereText += " > ?"
		} else {
			inboundWhereText += " = ?"
		}

		result := tx.Model(model.Inbound{}).
			Where(inboundWhereText, id).
			Update("last_traffic_reset_time", now)

		return result.Error
	}); err != nil {
		return nil, nil, nil, err
	}
	return reset, restore, resetNodes, nil
}

func (s *ClientService) ResetAllTraffics() (bool, error) {
	var affected int64
	var resetNodes []int
	var restore []string
	err := submitTrafficWrite(func() error {
		return database.GetDB().Transaction(func(tx *gorm.DB) error {
			var qErr error
			if restore, qErr = quotaDisabledEmails(tx, nil); qErr != nil {
				return qErr
			}
			keepOff, qErr := handDisabledEmails(tx, nil)
			if qErr != nil {
				return qErr
			}
			if affected, qErr = zeroClientTraffics(tx, nil, keepOff); qErr != nil {
				return qErr
			}
			if err := tx.Where("1 = 1").Delete(&model.ClientGlobalTraffic{}).Error; err != nil {
				return err
			}
			if err := tx.Where("1 = 1").Delete(&model.NodeClientTraffic{}).Error; err != nil {
				return err
			}
			resetNodes, qErr = queueNodeResets(tx, nil)
			return qErr
		})
	})
	if err != nil {
		return false, err
	}
	inbounds := &InboundService{}
	inbounds.resetAllMtprotoQuotas()
	inbounds.deliverNodeResetsNow(resetNodes)
	needRestart := false
	if len(restore) > 0 {
		var enableErr error
		if _, needRestart, enableErr = s.BulkSetEnable(inbounds, restore, true); enableErr != nil {
			logger.Warning("Failed to re-enable depleted clients after resetting all traffic:", enableErr)
		}
	}
	return affected > 0 || needRestart, nil
}

// handDisabledEmails lists, among emails (every client when nil), the clients switched off by
// hand: a disabled record whose quota is not spent. Decide it before a reset zeroes the counters.
func handDisabledEmails(tx *gorm.DB, emails []string) ([]string, error) {
	var disabled []string
	q := func() *gorm.DB { return tx.Model(&model.ClientRecord{}).Where("enable = ?", false) }
	if emails == nil {
		if err := q().Pluck("email", &disabled).Error; err != nil {
			return nil, err
		}
	} else {
		for _, batch := range chunkStrings(emails, sqlInChunk) {
			var page []string
			if err := q().Where("email IN ?", batch).Pluck("email", &page).Error; err != nil {
				return nil, err
			}
			disabled = append(disabled, page...)
		}
	}
	if len(disabled) == 0 {
		return nil, nil
	}
	quota, err := quotaDisabledEmails(tx, disabled)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(disabled, func(e string) bool { return slices.Contains(quota, e) }), nil
}

// zeroClientTraffics zeroes the counters of emails (every row when nil) and lifts their traffic
// gate, except for keepOff: a master pushes that flag to its nodes as the client's state.
func zeroClientTraffics(tx *gorm.DB, emails, keepOff []string) (int64, error) {
	var affected int64
	reset := map[string]any{"enable": true, "up": 0, "down": 0}
	if emails == nil {
		res := tx.Model(&xray.ClientTraffic{}).Where("1 = 1").Updates(reset)
		if res.Error != nil {
			return 0, res.Error
		}
		affected = res.RowsAffected
	} else {
		for _, batch := range chunkStrings(emails, sqlInChunk) {
			res := tx.Model(&xray.ClientTraffic{}).Where("email IN ?", batch).Updates(reset)
			if res.Error != nil {
				return 0, res.Error
			}
			affected += res.RowsAffected
		}
	}
	for _, batch := range chunkStrings(keepOff, sqlInChunk) {
		if err := tx.Model(&xray.ClientTraffic{}).Where("email IN ?", batch).Update("enable", false).Error; err != nil {
			return 0, err
		}
	}
	return affected, nil
}
