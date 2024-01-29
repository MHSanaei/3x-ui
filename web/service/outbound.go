package service

import (
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/xray"

	"gorm.io/gorm"
)

type OutboundService struct {
	xrayApi xray.XrayAPI
}

func (s *OutboundService) AddTraffic(traffics []*xray.Traffic, clientTraffics []*xray.ClientTraffic) (error, bool) {
	var err error
	db := database.GetDB()
	tx := db.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	err = s.addOutboundTraffic(tx, traffics)
	if err != nil {
		return err, false
	}

	return nil, false
}

func (s *OutboundService) addOutboundTraffic(tx *gorm.DB, traffics []*xray.Traffic) error {
	if len(traffics) == 0 {
		return nil
	}

	var err error

	for _, traffic := range traffics {
		if traffic.IsOutbound {

			var outbound model.Outbound

			err = tx.Model(&model.Outbound{}).Where("tag = ?", traffic.Tag).
				FirstOrCreate(&outbound).Error
			if err != nil {
				return err
			}

			outbound.Tag = traffic.Tag
			outbound.Up = outbound.Up + traffic.Up
			outbound.Down = outbound.Down + traffic.Down
			outbound.Total = outbound.Up + outbound.Down

			err = tx.Save(&outbound).Error
			if err != nil {
				return err
			}
		}
	}
	return nil
}
