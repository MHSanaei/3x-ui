package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

const clientPlansSettingKey = "tgbot.clientPlans.v1"

// This also serializes first inserts when the traffic writer has not started.
var botSettingsMu sync.Mutex

type ClientPlan struct {
	ID          string `json:"id"`
	Revision    int64  `json:"revision"`
	Name        string `json:"name"`
	QuotaBytes  int64  `json:"quotaBytes"`
	Days        int    `json:"days"`
	FirstUse    bool   `json:"firstUse"`
	InboundIDs  []int  `json:"inboundIds"`
	LimitIP     int    `json:"limitIp"`
	Enabled     bool   `json:"enabled"`
	RenewalMode string `json:"renewalMode"`
}

type ClientPlanService struct{}

func readBotSetting(tx *gorm.DB, key string, value any) error {
	var rows []model.Setting
	if err := tx.Where("key = ?", key).Limit(2).Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return gorm.ErrRecordNotFound
	}
	if len(rows) != 1 {
		return errors.New("ambiguous Telegram workflow settings")
	}
	return json.Unmarshal([]byte(rows[0].Value), value)
}

func writeBotSetting(tx *gorm.DB, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	result := tx.Model(&model.Setting{}).Where("key = ?", key).Update("value", string(data))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 1 {
		return errors.New("ambiguous Telegram workflow settings")
	}
	if result.RowsAffected == 0 {
		return tx.Create(&model.Setting{Key: key, Value: string(data)}).Error
	}
	return nil
}

func loadClientPlans(tx *gorm.DB) ([]ClientPlan, error) {
	rows := []ClientPlan{}
	err := readBotSetting(tx, clientPlansSettingKey, &rows)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return rows, nil
	}
	return rows, err
}

func (s *ClientPlanService) List() ([]ClientPlan, error) { return loadClientPlans(database.GetDB()) }

func (s *ClientPlanService) Resolve(name string) (*ClientPlan, error) {
	rows, err := s.List()
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if strings.EqualFold(row.Name, name) || row.ID == name {
			return &row, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (p ClientPlan) Payload(email, subID string, now int64) *ClientCreatePayload {
	expiry := int64(p.Days) * 86400000
	if p.FirstUse {
		expiry = -expiry
	} else if expiry != 0 {
		expiry += now
	}
	return &ClientCreatePayload{Client: model.Client{Email: email, SubID: subID, Enable: p.Enabled, TotalGB: p.QuotaBytes, ExpiryTime: expiry, LimitIP: p.LimitIP, TrafficReset: "never", TrafficResetDay: 1}, InboundIds: slices.Clone(p.InboundIDs)}
}

func validateClientPlan(tx *gorm.DB, plan ClientPlan, email string) error {
	if plan.Name == "" || utf8.RuneCountInString(plan.Name) > 32 || strings.ContainsFunc(plan.Name, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_' && r != '-' }) {
		return errors.New("plan name must contain 1–32 letters, numbers, hyphens or underscores")
	}
	if plan.QuotaBytes < 0 || plan.Days < 0 || plan.Days > 36500 || plan.LimitIP < 0 || plan.LimitIP > 9999 || (plan.RenewalMode != "replace" && plan.RenewalMode != "extend") {
		return errors.New("invalid client plan limits or renewal mode")
	}
	if email == "" {
		return errors.New("invalid client email")
	}
	if err := validateClientEmail(email); err != nil {
		return err
	}
	ids := slices.Clone(plan.InboundIDs)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if len(ids) == 0 || ids[0] <= 0 {
		return errors.New("at least one valid inbound is required")
	}
	var inbounds []model.Inbound
	for _, batch := range chunkInts(ids, sqlInChunk) {
		var page []model.Inbound
		if err := tx.Where("id IN ?", batch).Order("id ASC").Find(&page).Error; err != nil {
			return err
		}
		inbounds = append(inbounds, page...)
	}
	if len(inbounds) != len(ids) {
		return errors.New("a selected inbound no longer exists")
	}
	client := plan.Payload(email, "", 0).Client
	svc := &ClientService{}
	checkedNodes := map[struct {
		ID       int
		Protocol model.Protocol
	}]bool{}
	for i := range inbounds {
		if !inbounds[i].Protocol.SupportsClients() {
			return errors.New("selected inbound does not support individual clients")
		}
		if inbounds[i].NodeID != nil {
			key := struct {
				ID       int
				Protocol model.Protocol
			}{*inbounds[i].NodeID, inbounds[i].Protocol}
			if !checkedNodes[key] {
				if err := checkNodeCanHostProtocol(tx, key.ID, key.Protocol); err != nil {
					return err
				}
				checkedNodes[key] = true
			}
		}
		previousPassword := client.Password
		if _, err := svc.prepareClientAdd(&client, &inbounds[i]); err != nil {
			return fmt.Errorf("inbound %d: %w", inbounds[i].Id, err)
		}
		if previousPassword != "" && previousPassword != client.Password {
			return errors.New("selected inbounds require incompatible shared passwords")
		}
	}
	return nil
}

func (s *ClientPlanService) Validate(plan ClientPlan, email string) error {
	return validateClientPlan(database.GetDB(), plan, email)
}

func (s *ClientPlanService) Save(plan ClientPlan) (*ClientPlan, error) {
	botSettingsMu.Lock()
	defer botSettingsMu.Unlock()
	plan.InboundIDs = slices.Clone(plan.InboundIDs)
	slices.Sort(plan.InboundIDs)
	plan.InboundIDs = slices.Compact(plan.InboundIDs)
	err := runSerializedTx(func(tx *gorm.DB) error {
		if err := validateClientPlan(tx, plan, "plan-validation"); err != nil {
			return err
		}
		rows, err := loadClientPlans(tx)
		if err != nil {
			return err
		}
		index := -1
		for i := range rows {
			if rows[i].ID == plan.ID {
				index = i
			}
			if strings.EqualFold(rows[i].Name, plan.Name) && rows[i].ID != plan.ID {
				return errors.New("plan name already exists")
			}
		}
		if plan.ID == "" {
			if plan.Revision != 0 || len(rows) >= 100 {
				return errors.New("invalid new plan or plan limit reached")
			}
			plan.ID = uuid.NewString()
			plan.Revision = 1
			rows = append(rows, plan)
		} else {
			if index < 0 || rows[index].Revision != plan.Revision {
				return errors.New("plan changed; reopen it before editing")
			}
			plan.Revision++
			rows[index] = plan
		}
		return writeBotSetting(tx, clientPlansSettingKey, rows)
	})
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func (s *ClientPlanService) Delete(id string, revision int64) error {
	botSettingsMu.Lock()
	defer botSettingsMu.Unlock()
	return runSerializedTx(func(tx *gorm.DB) error {
		rows, err := loadClientPlans(tx)
		if err != nil {
			return err
		}
		for i := range rows {
			if rows[i].ID != id {
				continue
			}
			if rows[i].Revision != revision {
				return errors.New("plan changed; reopen it before deleting")
			}
			return writeBotSetting(tx, clientPlansSettingKey, append(rows[:i], rows[i+1:]...))
		}
		return gorm.ErrRecordNotFound
	})
}
