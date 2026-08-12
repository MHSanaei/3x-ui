package service

import (
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/naive"
)

type naiveSyncResult struct {
	started   []string
	restarted []string
	stopped   []string
}

type naiveStubSettings struct {
	Proxy               string `json:"proxy"`
	InsecureConcurrency *int   `json:"insecureConcurrency"`
	TunnelTimeout       *int   `json:"tunnelTimeout"`
	IdleTimeout         *int   `json:"idleTimeout"`
	ExtraHeaders        string `json:"extraHeaders"`
	HostResolverRules   string `json:"hostResolverRules"`
	ResolverRange       string `json:"resolverRange"`
	NoPostQuantum       bool   `json:"noPostQuantum"`
}

type naiveStub struct {
	Tag      string            `json:"tag"`
	Protocol string            `json:"protocol"`
	Settings naiveStubSettings `json:"settings"`
}

func saveSettingTx(tx *gorm.DB, key string, value string) error {
	setting := &model.Setting{}
	err := tx.Where("key = ?", key).First(setting).Error
	if database.IsNotFound(err) {
		return tx.Create(&model.Setting{Key: key, Value: value}).Error
	}
	if err != nil {
		return err
	}
	setting.Value = value
	return tx.Save(setting).Error
}

func (s *XraySettingService) saveTemplateAndSyncNaive(newXraySettings string) error {
	db := database.GetDB()
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	result, err := syncNaiveOutboundsTx(tx, newXraySettings)
	if err != nil {
		tx.Rollback()
		return err
	}
	if err := saveSettingTx(tx, "xrayTemplateConfig", newXraySettings); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	if err := applyNaiveSyncResult(result); err != nil {
		return fmt.Errorf("apply naive outbounds: %w", err)
	}
	return nil
}

func syncNaiveOutboundsTx(tx *gorm.DB, raw string) (naiveSyncResult, error) {
	var payload struct {
		Outbounds []json.RawMessage `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return naiveSyncResult{}, err
	}
	var existing []model.NaiveOutbound
	if err := tx.Find(&existing).Error; err != nil {
		return naiveSyncResult{}, err
	}
	byTag := make(map[string]model.NaiveOutbound, len(existing))
	for _, item := range existing {
		byTag[item.Tag] = item
	}
	seen := map[string]struct{}{}
	result := naiveSyncResult{}
	for _, outbound := range payload.Outbounds {
		record, isNaive, changed, err := naiveOutboundFromStubTx(tx, outbound, byTag)
		if err != nil {
			return naiveSyncResult{}, err
		}
		if !isNaive {
			continue
		}
		seen[record.Tag] = struct{}{}
		if err := tx.Save(&record).Error; err != nil {
			return naiveSyncResult{}, err
		}
		if _, exists := byTag[record.Tag]; exists {
			if changed {
				result.restarted = append(result.restarted, record.Tag)
			}
		} else {
			result.started = append(result.started, record.Tag)
		}
	}
	for _, record := range existing {
		if _, ok := seen[record.Tag]; ok {
			continue
		}
		if err := tx.Delete(&model.NaiveOutbound{}, "tag = ?", record.Tag).Error; err != nil {
			return naiveSyncResult{}, err
		}
		result.stopped = append(result.stopped, record.Tag)
	}
	return result, nil
}

func parseNaiveStub(outbound json.RawMessage) (naiveStub, bool, error) {
	var meta struct {
		Protocol string `json:"protocol"`
	}
	if err := json.Unmarshal(outbound, &meta); err != nil {
		return naiveStub{}, false, err
	}
	if meta.Protocol != "naive" {
		return naiveStub{}, false, nil
	}
	var row naiveStub
	if err := json.Unmarshal(outbound, &row); err != nil {
		return naiveStub{}, true, err
	}
	if err := validateNaiveStubValues(row); err != nil {
		return naiveStub{}, true, err
	}
	return row, true, nil
}

func validateNaiveStubValues(row naiveStub) error {
	if err := naive.ValidateTag(row.Tag); err != nil {
		return err
	}
	if err := naive.ValidateProxyURL(row.Settings.Proxy); err != nil {
		return err
	}
	if value := row.Settings.InsecureConcurrency; value != nil && (*value < 1 || *value > 8) {
		return fmt.Errorf("insecureConcurrency must be between 1 and 8")
	}
	if value := row.Settings.TunnelTimeout; value != nil && *value < 0 {
		return fmt.Errorf("tunnelTimeout must be non-negative")
	}
	if value := row.Settings.IdleTimeout; value != nil && *value < 0 {
		return fmt.Errorf("idleTimeout must be non-negative")
	}
	return nil
}

func naiveOutboundFromStubTx(tx *gorm.DB, outbound json.RawMessage, existing map[string]model.NaiveOutbound) (model.NaiveOutbound, bool, bool, error) {
	row, isNaive, err := parseNaiveStub(outbound)
	if err != nil || !isNaive {
		return model.NaiveOutbound{}, isNaive, false, err
	}
	record, exists := existing[row.Tag]
	if !exists {
		port, err := naive.AllocatePort(tx)
		if err != nil {
			return model.NaiveOutbound{}, true, false, err
		}
		record = model.NaiveOutbound{Tag: row.Tag, LocalPort: port, Enabled: true}
	}
	before := record
	record.ProxyURL = row.Settings.Proxy
	record.InsecureConcurrency = valueOrZero(row.Settings.InsecureConcurrency)
	record.TunnelTimeout = valueOrZero(row.Settings.TunnelTimeout)
	record.IdleTimeout = valueOrZero(row.Settings.IdleTimeout)
	record.ExtraHeaders = row.Settings.ExtraHeaders
	record.HostResolverRules = row.Settings.HostResolverRules
	record.ResolverRange = row.Settings.ResolverRange
	record.NoPostQuantum = row.Settings.NoPostQuantum
	record.Enabled = true
	changed := !exists || record.ProxyURL != before.ProxyURL || record.InsecureConcurrency != before.InsecureConcurrency || record.TunnelTimeout != before.TunnelTimeout || record.IdleTimeout != before.IdleTimeout || record.ExtraHeaders != before.ExtraHeaders || record.HostResolverRules != before.HostResolverRules || record.ResolverRange != before.ResolverRange || record.NoPostQuantum != before.NoPostQuantum
	return record, true, changed, nil
}

func valueOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func applyNaiveSyncResult(result naiveSyncResult) error {
	manager := naive.GetManager()
	var applyErrors []error
	for _, tag := range result.restarted {
		if err := manager.Stop(tag); err != nil {
			applyErrors = append(applyErrors, fmt.Errorf("stop changed outbound %q: %w", tag, err))
		}
	}
	if err := manager.StartAll(); err != nil {
		applyErrors = append(applyErrors, fmt.Errorf("start configured outbounds: %w", err))
	}
	if len(applyErrors) == 0 {
		for _, tag := range result.stopped {
			if err := manager.Stop(tag); err != nil {
				applyErrors = append(applyErrors, fmt.Errorf("stop removed outbound %q: %w", tag, err))
				continue
			}
			if err := naive.RemoveLog(tag); err != nil {
				applyErrors = append(applyErrors, fmt.Errorf("remove log for deleted outbound %q: %w", tag, err))
			}
		}
	}
	if len(applyErrors) == 0 && len(result.started)+len(result.restarted)+len(result.stopped) > 0 {
		(&XrayService{}).SetToNeedRestart()
	}
	return errors.Join(applyErrors...)
}

func validateNaiveStub(outbound []byte) error {
	_, isNaive, err := parseNaiveStub(outbound)
	if err != nil {
		return err
	}
	if !isNaive {
		return fmt.Errorf("outbound protocol is not naive")
	}
	return nil
}
