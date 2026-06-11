package service

import (
	"strings"
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

	needRestart := false
	if !rec.Enable {
		updated := rec.ToClient()
		updated.Enable = true
		nr, uErr := s.Update(inboundSvc, rec.Id, *updated)
		if uErr != nil {
			logger.Warning("Failed to auto-enable client during traffic reset:", uErr)
		}
		if nr {
			needRestart = true
		}
	}

	if len(inboundIds) == 0 {
		if rErr := inboundSvc.ResetClientTrafficByEmail(email); rErr != nil {
			return false, rErr
		}
		return needRestart, nil
	}

	for _, ibId := range inboundIds {
		nr, rErr := inboundSvc.ResetClientTraffic(ibId, email)
		if rErr != nil {
			return needRestart, rErr
		}
		if nr {
			needRestart = true
		}
	}
	return needRestart, nil
}

func (s *ClientService) BulkResetTraffic(inboundSvc *InboundService, emails []string) (int, error) {
	if len(emails) == 0 {
		return 0, nil
	}
	seen := map[string]struct{}{}
	cleanEmails := make([]string, 0, len(emails))
	for _, e := range emails {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if _, ok := seen[e]; ok {
			continue
		}
		seen[e] = struct{}{}
		cleanEmails = append(cleanEmails, e)
	}
	if len(cleanEmails) == 0 {
		return 0, nil
	}

	for _, e := range cleanEmails {
		rec, err := s.GetRecordByEmail(nil, e)
		if err == nil && !rec.Enable {
			updated := rec.ToClient()
			updated.Enable = true
			s.Update(inboundSvc, rec.Id, *updated)
		}
	}

	affected := 0
	err := submitTrafficWrite(func() error {
		db := database.GetDB()
		return db.Transaction(func(tx *gorm.DB) error {
			for _, batch := range chunkStrings(cleanEmails, sqlInChunk) {
				res := tx.Model(xray.ClientTraffic{}).
					Where("email IN ?", batch).
					Updates(map[string]any{"enable": true, "up": 0, "down": 0})
				if res.Error != nil {
					return res.Error
				}
				affected += int(res.RowsAffected)
			}
			return clearGlobalTraffic(tx, cleanEmails...)
		})
	})
	if err != nil {
		return 0, err
	}
	return affected, nil
}

func (s *ClientService) ResetAllClientTraffics(inboundSvc *InboundService, id int) error {
	return submitTrafficWrite(func() error {
		return s.resetAllClientTrafficsLocked(id)
	})
}

func (s *ClientService) resetAllClientTrafficsLocked(id int) error {
	db := database.GetDB()
	now := time.Now().Unix() * 1000

	if err := db.Transaction(func(tx *gorm.DB) error {
		whereText := "inbound_id "
		if id == -1 {
			whereText += " > ?"
		} else {
			whereText += " = ?"
		}

		var resetEmails []string
		if err := tx.Model(xray.ClientTraffic{}).
			Where(whereText, id).
			Pluck("email", &resetEmails).Error; err != nil {
			return err
		}

		result := tx.Model(xray.ClientTraffic{}).
			Where(whereText, id).
			Updates(map[string]any{"enable": true, "up": 0, "down": 0})

		if result.Error != nil {
			return result.Error
		}

		if err := clearGlobalTraffic(tx, resetEmails...); err != nil {
			return err
		}

		inboundWhereText := "id "
		if id == -1 {
			inboundWhereText += " > ?"
		} else {
			inboundWhereText += " = ?"
		}

		result = tx.Model(model.Inbound{}).
			Where(inboundWhereText, id).
			Update("last_traffic_reset_time", now)

		return result.Error
	}); err != nil {
		return err
	}
	return nil
}

func (s *ClientService) ResetAllTraffics() (bool, error) {
	db := database.GetDB()
	res := db.Model(&xray.ClientTraffic{}).
		Where("1 = 1").
		Updates(map[string]any{"up": 0, "down": 0})
	if res.Error != nil {
		return false, res.Error
	}
	if err := db.Where("1 = 1").Delete(&model.ClientGlobalTraffic{}).Error; err != nil {
		return false, err
	}
	return res.RowsAffected > 0, nil
}
