package service

import (
	"context"
	"errors"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"

	"gorm.io/gorm"
)

// wgRecordsForInbound fetches all ClientRecord rows attached to a WireGuard inbound.
func wgRecordsForInbound(tx *gorm.DB, inboundId int) ([]*model.ClientRecord, error) {
	if tx == nil {
		tx = database.GetDB()
	}
	var recs []*model.ClientRecord
	err := tx.Table("clients").
		Joins("JOIN client_inbounds ON client_inbounds.client_id = clients.id").
		Where("client_inbounds.inbound_id = ?", inboundId).
		Order("clients.id ASC").
		Find(&recs).Error
	return recs, err
}

// rebuildAndSaveWgPeers rebuilds settings.peers[] from the current clients list
// and persists the inbound to the database. Must be called inside a transaction.
func rebuildAndSaveWgPeers(tx *gorm.DB, inbound *model.Inbound) error {
	recs, err := wgRecordsForInbound(tx, inbound.Id)
	if err != nil {
		return err
	}
	newSettings, err := syncWgPeersFromClients(inbound.Settings, recs)
	if err != nil {
		return err
	}
	inbound.Settings = newSettings
	return tx.Save(inbound).Error
}

// AddWgClient creates a WireGuard peer as a client record, attaches it to the
// inbound, rebuilds settings.peers[], and triggers an xray restart.
func (s *ClientService) AddWgClient(inboundSvc *InboundService, inboundId int, rec *model.ClientRecord) (bool, error) {
	defer lockInbound(inboundId).Unlock()

	if rec.Email == "" {
		return false, common.NewError("peer name (email) is required")
	}
	if rec.WgSettings == "" {
		return false, common.NewError("WireGuard peer settings are required")
	}

	inbound, err := inboundSvc.GetInbound(inboundId)
	if err != nil {
		return false, err
	}
	if inbound.Protocol != model.WireGuard {
		return false, common.NewError("inbound is not WireGuard")
	}

	// Check for duplicate email.
	var existing model.ClientRecord
	checkErr := database.GetDB().Where("email = ?", rec.Email).First(&existing).Error
	if checkErr == nil {
		return false, common.NewError("peer name already in use:", rec.Email)
	}
	if !errors.Is(checkErr, gorm.ErrRecordNotFound) {
		return false, checkErr
	}

	now := time.Now().UnixMilli()
	rec.Enable = true
	rec.CreatedAt = now
	rec.UpdatedAt = now

	needRestart := false
	txErr := runSerializedTx(func(tx *gorm.DB) error {
		if err := tx.Create(rec).Error; err != nil {
			return err
		}
		link := model.ClientInbound{ClientId: rec.Id, InboundId: inboundId}
		if err := tx.Create(&link).Error; err != nil {
			return err
		}
		return rebuildAndSaveWgPeers(tx, inbound)
	})
	if txErr != nil {
		return false, txErr
	}

	rt, push, _, perr := inboundSvc.nodePushPlan(inbound)
	if perr != nil {
		logger.Warning("nodePushPlan failed after WG peer add:", perr)
		needRestart = true
	} else if push {
		if err := rt.UpdateInbound(context.Background(), inbound, inbound); err != nil {
			logger.Warning("WG inbound update on runtime failed:", err)
			needRestart = true
		}
	} else {
		needRestart = true
	}
	return needRestart, nil
}

// UpdateWgClient finds the peer by email, updates its record and wg_settings,
// rebuilds settings.peers[], and triggers an xray restart.
func (s *ClientService) UpdateWgClient(inboundSvc *InboundService, inboundId int, email string, rec *model.ClientRecord) (bool, error) {
	defer lockInbound(inboundId).Unlock()

	inbound, err := inboundSvc.GetInbound(inboundId)
	if err != nil {
		return false, err
	}
	if inbound.Protocol != model.WireGuard {
		return false, common.NewError("inbound is not WireGuard")
	}

	var existing model.ClientRecord
	if err := database.GetDB().Where("email = ?", email).First(&existing).Error; err != nil {
		return false, common.NewError("peer not found:", email)
	}

	now := time.Now().UnixMilli()
	existing.Email = rec.Email
	existing.Password = rec.Password
	existing.WgSettings = rec.WgSettings
	existing.Enable = rec.Enable
	existing.Comment = rec.Comment
	existing.UpdatedAt = now

	needRestart := false
	txErr := runSerializedTx(func(tx *gorm.DB) error {
		if err := tx.Save(&existing).Error; err != nil {
			return err
		}
		return rebuildAndSaveWgPeers(tx, inbound)
	})
	if txErr != nil {
		return false, txErr
	}

	rt, push, _, perr := inboundSvc.nodePushPlan(inbound)
	if perr != nil {
		logger.Warning("nodePushPlan failed after WG peer update:", perr)
		needRestart = true
	} else if push {
		if err := rt.UpdateInbound(context.Background(), inbound, inbound); err != nil {
			logger.Warning("WG inbound update on runtime failed:", err)
			needRestart = true
		}
	} else {
		needRestart = true
	}
	return needRestart, nil
}

// DelWgClient removes the peer by email, rebuilds settings.peers[], and
// triggers an xray restart.
func (s *ClientService) DelWgClient(inboundSvc *InboundService, inboundId int, email string) (bool, error) {
	defer lockInbound(inboundId).Unlock()

	inbound, err := inboundSvc.GetInbound(inboundId)
	if err != nil {
		return false, err
	}
	if inbound.Protocol != model.WireGuard {
		return false, common.NewError("inbound is not WireGuard")
	}

	var rec model.ClientRecord
	if err := database.GetDB().Where("email = ?", email).First(&rec).Error; err != nil {
		return false, common.NewError("peer not found:", email)
	}

	needRestart := false
	txErr := runSerializedTx(func(tx *gorm.DB) error {
		if err := tx.Where("client_id = ? AND inbound_id = ?", rec.Id, inboundId).
			Delete(&model.ClientInbound{}).Error; err != nil {
			return err
		}
		// Check if this client is attached to other inbounds before deleting the record.
		var otherLinks int64
		tx.Model(&model.ClientInbound{}).Where("client_id = ?", rec.Id).Count(&otherLinks)
		if otherLinks == 0 {
			if err := tx.Delete(&rec).Error; err != nil {
				return err
			}
		}
		return rebuildAndSaveWgPeers(tx, inbound)
	})
	if txErr != nil {
		return false, txErr
	}

	rt, push, _, perr := inboundSvc.nodePushPlan(inbound)
	if perr != nil {
		logger.Warning("nodePushPlan failed after WG peer del:", perr)
		needRestart = true
	} else if push {
		if err := rt.UpdateInbound(context.Background(), inbound, inbound); err != nil {
			logger.Warning("WG inbound update on runtime failed:", err)
			needRestart = true
		}
	} else {
		needRestart = true
	}
	return needRestart, nil
}
