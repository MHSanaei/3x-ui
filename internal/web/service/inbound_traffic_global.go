package service

import (
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *InboundService) AcceptGlobalTraffic(traffics []*xray.ClientTraffic) error {
	db := database.GetDB()
	return submitTrafficWrite(func() error {
		tx := db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		for _, t := range traffics {
			if t == nil || t.Email == "" {
				continue
			}

			if err := tx.Model(xray.ClientTraffic{}).Where("email = ?", t.Email).
				Updates(map[string]any{
					"up":   gorm.Expr(database.GreatestExpr("up", "?"), t.Up),
					"down": gorm.Expr(database.GreatestExpr("down", "?"), t.Down),
				}).Error; err != nil {
				tx.Rollback()
				return err
			}

			err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "node_id"}, {Name: "email"}},
				DoUpdates: clause.AssignmentColumns([]string{"up", "down"}),
			}).Create(&model.NodeClientTraffic{NodeId: 0, Email: t.Email, Up: t.Up, Down: t.Down}).Error
			if err != nil {
				tx.Rollback()
				return err
			}
		}
		return tx.Commit().Error
	})
}

func (s *InboundService) GetPushedBaselines() (map[string]runtime.NodeTrafficCounter, error) {
	db := database.GetDB()
	var rows []model.NodeClientTraffic
	if err := db.Where("node_id = ?", 0).Find(&rows).Error; err != nil {
		return nil, err
	}
	res := make(map[string]runtime.NodeTrafficCounter, len(rows))
	for _, r := range rows {
		res[r.Email] = runtime.NodeTrafficCounter{Up: r.Up, Down: r.Down}
	}
	return res, nil
}
