package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

type HwidRequest struct {
	Hwid        string
	UserAgent   string
	DeviceOS    string
	OsVersion   string
	DeviceModel string
}

type HwidGateResult struct {
	Allowed           bool
	Active            bool
	NotSupported      bool
	MaxDevicesReached bool
	LimitReached      bool
	Limit             int
	Registered        int
}

type ClientHwidInfo struct {
	Id          int    `json:"id"`
	FirstSeen   int64  `json:"firstSeen"`
	LastSeen    int64  `json:"lastSeen"`
	UserAgent   string `json:"userAgent"`
	DeviceOS    string `json:"deviceOs"`
	OsVersion   string `json:"osVersion"`
	DeviceModel string `json:"deviceModel"`
}

func hashHwid(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func trimHwidMeta(s string) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > 512 {
		return string(r[:512])
	}
	return s
}

func normalizeHwidRequest(req HwidRequest) HwidRequest {
	return HwidRequest{
		Hwid:        strings.TrimSpace(req.Hwid),
		UserAgent:   trimHwidMeta(req.UserAgent),
		DeviceOS:    trimHwidMeta(req.DeviceOS),
		OsVersion:   trimHwidMeta(req.OsVersion),
		DeviceModel: trimHwidMeta(req.DeviceModel),
	}
}

func (s *ClientService) EnforceHwidForSubID(subID string, req HwidRequest) (HwidGateResult, error) {
	var res HwidGateResult
	subID = strings.TrimSpace(subID)
	if subID == "" {
		res.Allowed = true
		return res, nil
	}

	db := database.GetDB()
	var rec model.ClientRecord
	err := db.Where("sub_id = ? AND enable = ?", subID, true).Order("id ASC").First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		res.Allowed = true
		return res, nil
	}
	if err != nil {
		return res, err
	}
	if rec.LimitHwid <= 0 {
		res.Allowed = true
		return res, nil
	}

	req = normalizeHwidRequest(req)
	res.Active = true
	res.Limit = rec.LimitHwid
	if len(req.Hwid) < 6 {
		res.NotSupported = true
		return res, nil
	}
	hwidHash := hashHwid(req.Hwid)

	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ClientRecord{}).
			Where("id = ?", rec.Id).
			UpdateColumn("updated_at", gorm.Expr("updated_at")).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ? AND enable = ?", rec.Id, true).First(&rec).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				res = HwidGateResult{Allowed: true}
				return nil
			}
			return err
		}
		if rec.LimitHwid <= 0 {
			res = HwidGateResult{Allowed: true}
			return nil
		}
		res.Active = true
		res.Limit = rec.LimitHwid

		now := time.Now().UnixMilli()
		var existing model.ClientHwid
		err := tx.Where("client_id = ? AND hwid_hash = ?", rec.Id, hwidHash).First(&existing).Error
		if err == nil {
			if err := tx.Model(&model.ClientHwid{}).Where("id = ?", existing.Id).Updates(map[string]any{
				"last_seen":    now,
				"user_agent":   req.UserAgent,
				"device_os":    req.DeviceOS,
				"os_version":   req.OsVersion,
				"device_model": req.DeviceModel,
			}).Error; err != nil {
				return err
			}
			var count int64
			if err := tx.Model(&model.ClientHwid{}).Where("client_id = ?", rec.Id).Count(&count).Error; err != nil {
				return err
			}
			res.Allowed = true
			res.Registered = int(count)
			res.LimitReached = count >= int64(rec.LimitHwid)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var count int64
		if err := tx.Model(&model.ClientHwid{}).Where("client_id = ?", rec.Id).Count(&count).Error; err != nil {
			return err
		}
		res.Registered = int(count)
		if count >= int64(rec.LimitHwid) {
			res.MaxDevicesReached = true
			res.LimitReached = true
			return nil
		}
		if err := tx.Create(&model.ClientHwid{
			ClientId:    rec.Id,
			HwidHash:    hwidHash,
			FirstSeen:   now,
			LastSeen:    now,
			UserAgent:   req.UserAgent,
			DeviceOS:    req.DeviceOS,
			OsVersion:   req.OsVersion,
			DeviceModel: req.DeviceModel,
		}).Error; err != nil {
			return err
		}
		res.Allowed = true
		res.Registered = int(count) + 1
		res.LimitReached = res.Registered >= rec.LimitHwid
		return nil
	})
	return res, err
}

func (s *ClientService) ListClientHwids(email string) ([]ClientHwidInfo, error) {
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return nil, err
	}
	var rows []model.ClientHwid
	if err := database.GetDB().
		Where("client_id = ?", rec.Id).
		Order("last_seen DESC").
		Order("id DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ClientHwidInfo, 0, len(rows))
	for _, r := range rows {
		out = append(out, ClientHwidInfo{
			Id:          r.Id,
			FirstSeen:   r.FirstSeen,
			LastSeen:    r.LastSeen,
			UserAgent:   r.UserAgent,
			DeviceOS:    r.DeviceOS,
			OsVersion:   r.OsVersion,
			DeviceModel: r.DeviceModel,
		})
	}
	return out, nil
}

func (s *ClientService) ClearClientHwids(email string) error {
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return err
	}
	return database.GetDB().Where("client_id = ?", rec.Id).Delete(&model.ClientHwid{}).Error
}

func (s *ClientService) setClientLimitHwidByEmail(tx *gorm.DB, email string, limit int) error {
	if tx == nil {
		tx = database.GetDB()
	}
	if limit < 0 {
		limit = 0
	}
	var rec model.ClientRecord
	if err := tx.Where("email = ?", email).First(&rec).Error; err != nil {
		return err
	}
	if err := tx.Model(&model.ClientRecord{}).Where("id = ?", rec.Id).UpdateColumn("limit_hwid", limit).Error; err != nil {
		return err
	}
	return trimClientHwids(tx, rec.Id, limit)
}

func trimClientHwids(tx *gorm.DB, clientID int, limit int) error {
	if limit <= 0 {
		return nil
	}
	var keep []int
	if err := tx.Model(&model.ClientHwid{}).
		Where("client_id = ?", clientID).
		Order("last_seen DESC").
		Order("id DESC").
		Limit(limit).
		Pluck("id", &keep).Error; err != nil {
		return err
	}
	if len(keep) <= limit {
		var count int64
		if err := tx.Model(&model.ClientHwid{}).Where("client_id = ?", clientID).Count(&count).Error; err != nil {
			return err
		}
		if count <= int64(limit) {
			return nil
		}
	}
	return tx.Where("client_id = ? AND id NOT IN ?", clientID, keep).Delete(&model.ClientHwid{}).Error
}
