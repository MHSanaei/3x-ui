package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash/fnv"
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

const minHwidLength = 6

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

func hwidAdvisoryLockKey(subID string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("client_hwid:" + subID))
	return int64(h.Sum64())
}

func lockSubHwidRegistration(tx *gorm.DB, subID string) error {
	if !database.IsPostgres() {
		return nil
	}
	return tx.Exec("SELECT pg_advisory_xact_lock(?)", hwidAdvisoryLockKey(subID)).Error
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

func effectiveHwidLimitForSubID(tx *gorm.DB, subID string) (int, error) {
	var limit int
	err := tx.Model(&model.ClientRecord{}).
		Where("sub_id = ? AND enable = ?", subID, true).
		Select("COALESCE(MAX(limit_hwid), 0)").
		Scan(&limit).Error
	return limit, err
}

func (s *ClientService) EnforceHwidForSubID(subID string, req HwidRequest) (HwidGateResult, error) {
	var res HwidGateResult
	subID = strings.TrimSpace(subID)
	if subID == "" {
		res.Allowed = true
		return res, nil
	}

	db := database.GetDB()
	limit, err := effectiveHwidLimitForSubID(db, subID)
	if err != nil {
		return res, err
	}
	if limit <= 0 {
		res.Allowed = true
		return res, nil
	}

	req = normalizeHwidRequest(req)
	res.Active = true
	res.Limit = limit
	if len(req.Hwid) < minHwidLength {
		res.NotSupported = true
		return res, nil
	}
	hwidHash := hashHwid(req.Hwid)

	err = db.Transaction(func(tx *gorm.DB) error {
		if err := lockSubHwidRegistration(tx, subID); err != nil {
			return err
		}
		limit, err := effectiveHwidLimitForSubID(tx, subID)
		if err != nil {
			return err
		}
		if limit <= 0 {
			res = HwidGateResult{Allowed: true}
			return nil
		}
		res.Active = true
		res.Limit = limit
		now := time.Now().UnixMilli()
		var existing model.ClientHwid
		err = tx.Where("sub_id = ? AND hwid_hash = ?", subID, hwidHash).First(&existing).Error
		if err == nil {
			if err := tx.Model(&model.ClientHwid{}).Where("id = ?", existing.Id).Updates(map[string]any{
				"last_seen": now, "user_agent": req.UserAgent, "device_os": req.DeviceOS, "os_version": req.OsVersion, "device_model": req.DeviceModel,
			}).Error; err != nil {
				return err
			}
			var count int64
			if err := tx.Model(&model.ClientHwid{}).Where("sub_id = ?", subID).Count(&count).Error; err != nil {
				return err
			}
			res.Allowed = true
			res.Registered = int(count)
			res.LimitReached = count >= int64(limit)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var count int64
		if err := tx.Model(&model.ClientHwid{}).Where("sub_id = ?", subID).Count(&count).Error; err != nil {
			return err
		}
		res.Registered = int(count)
		if count >= int64(limit) {
			res.MaxDevicesReached = true
			res.LimitReached = true
			return nil
		}
		if err := tx.Create(&model.ClientHwid{SubID: subID, HwidHash: hwidHash, FirstSeen: now, LastSeen: now, UserAgent: req.UserAgent, DeviceOS: req.DeviceOS, OsVersion: req.OsVersion, DeviceModel: req.DeviceModel}).Error; err != nil {
			return err
		}
		res.Allowed = true
		res.Registered = int(count) + 1
		res.LimitReached = res.Registered >= limit
		return nil
	})
	return res, err
}

func (s *ClientService) ListClientHwids(email string) ([]ClientHwidInfo, error) {
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return nil, err
	}
	subID := strings.TrimSpace(rec.SubID)
	if subID == "" {
		return nil, nil
	}
	var rows []model.ClientHwid
	if err := database.GetDB().
		Where("sub_id = ?", subID).
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
	subID := strings.TrimSpace(rec.SubID)
	if subID == "" {
		return nil
	}
	return database.GetDB().Where("sub_id = ?", subID).Delete(&model.ClientHwid{}).Error
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
	subID := strings.TrimSpace(rec.SubID)
	if subID == "" {
		return nil
	}
	effective, err := effectiveHwidLimitForSubID(tx, subID)
	if err != nil {
		return err
	}
	return trimClientHwidsForSubID(tx, subID, effective)
}

func trimClientHwidsForSubID(tx *gorm.DB, subID string, limit int) error {
	subID = strings.TrimSpace(subID)
	if subID == "" || limit <= 0 {
		return nil
	}
	var keep []int
	if err := tx.Model(&model.ClientHwid{}).
		Where("sub_id = ?", subID).
		Order("last_seen DESC").
		Order("id DESC").
		Limit(limit).
		Pluck("id", &keep).Error; err != nil {
		return err
	}
	if len(keep) == 0 {
		return tx.Where("sub_id = ?", subID).Delete(&model.ClientHwid{}).Error
	}
	return tx.Where("sub_id = ? AND id NOT IN ?", subID, keep).Delete(&model.ClientHwid{}).Error
}

func clearClientHwidsBySubIDTx(tx *gorm.DB, subIDs ...string) error {
	if tx == nil {
		tx = database.GetDB()
	}
	clean := make([]string, 0, len(subIDs))
	seen := map[string]struct{}{}
	for _, subID := range subIDs {
		subID = strings.TrimSpace(subID)
		if subID == "" {
			continue
		}
		if _, ok := seen[subID]; ok {
			continue
		}
		seen[subID] = struct{}{}
		clean = append(clean, subID)
	}
	for _, batch := range chunkStrings(clean, sqlInChunk) {
		if err := tx.Where("sub_id IN ?", batch).Delete(&model.ClientHwid{}).Error; err != nil {
			return err
		}
	}
	return nil
}
