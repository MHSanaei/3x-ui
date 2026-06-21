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

// AddWgClient creates a WireGuard peer as a client record, attaches it to the
// inbound, appends the peer to settings.peers[], and triggers an xray hot-reload.
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

	var existing model.ClientRecord
	checkErr := database.GetDB().Where("email = ?", rec.Email).First(&existing).Error
	if checkErr == nil {
		return false, common.NewError("peer name already in use:", rec.Email)
	}
	if !errors.Is(checkErr, gorm.ErrRecordNotFound) {
		return false, checkErr
	}

	peer, err := buildPeerMap(rec)
	if err != nil {
		return false, err
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
		newSettings, sErr := addPeerToSettings(inbound.Settings, peer)
		if sErr != nil {
			return sErr
		}
		inbound.Settings = newSettings
		return tx.Save(inbound).Error
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
// patches settings.peers[] in-place, and triggers an xray hot-reload.
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

	peer, err := buildPeerMap(&existing)
	if err != nil {
		return false, err
	}

	needRestart := false
	txErr := runSerializedTx(func(tx *gorm.DB) error {
		if err := tx.Save(&existing).Error; err != nil {
			return err
		}
		// peer == nil when WgSettings is empty: treat as disabled in peers[].
		newSettings, sErr := updatePeerInSettings(inbound.Settings, email, peer, existing.Enable && peer != nil)
		if sErr != nil {
			return sErr
		}
		inbound.Settings = newSettings
		return tx.Save(inbound).Error
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

// DelWgClient removes the peer by email, removes it from settings.peers[], and
// triggers an xray hot-reload.
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
		var otherLinks int64
		tx.Model(&model.ClientInbound{}).Where("client_id = ?", rec.Id).Count(&otherLinks)
		if otherLinks == 0 {
			if err := tx.Delete(&rec).Error; err != nil {
				return err
			}
		}
		newSettings, sErr := removePeerFromSettings(inbound.Settings, email)
		if sErr != nil {
			return sErr
		}
		inbound.Settings = newSettings
		return tx.Save(inbound).Error
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
