package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/naive"
	"gorm.io/gorm"
)

type naiveSyncResult struct {
	started   []string
	restarted []string
	stopped   []string
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
	applyNaiveSyncResult(result)
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

func naiveOutboundFromStubTx(tx *gorm.DB, outbound json.RawMessage, existing map[string]model.NaiveOutbound) (model.NaiveOutbound, bool, bool, error) {
	var row struct {
		Tag      string         `json:"tag"`
		Protocol string         `json:"protocol"`
		Settings map[string]any `json:"settings"`
	}
	if err := json.Unmarshal(outbound, &row); err != nil {
		return model.NaiveOutbound{}, false, false, err
	}
	if row.Protocol != "naive" {
		return model.NaiveOutbound{}, false, false, nil
	}
	if err := naive.ValidateTag(row.Tag); err != nil {
		return model.NaiveOutbound{}, true, false, err
	}
	proxyURL, _ := row.Settings["proxy"].(string)
	if err := naive.ValidateProxyURL(proxyURL); err != nil {
		return model.NaiveOutbound{}, true, false, err
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
	record.ProxyURL = proxyURL
	record.InsecureConcurrency = asInt(row.Settings["insecureConcurrency"])
	record.TunnelTimeout = asInt(row.Settings["tunnelTimeout"])
	record.IdleTimeout = asInt(row.Settings["idleTimeout"])
	record.ExtraHeaders = asString(row.Settings["extraHeaders"])
	record.HostResolverRules = asString(row.Settings["hostResolverRules"])
	record.ResolverRange = asString(row.Settings["resolverRange"])
	record.NoPostQuantum = asBool(row.Settings["noPostQuantum"])
	record.Enabled = true
	changed := !exists || record.ProxyURL != before.ProxyURL || record.InsecureConcurrency != before.InsecureConcurrency || record.TunnelTimeout != before.TunnelTimeout || record.IdleTimeout != before.IdleTimeout || record.ExtraHeaders != before.ExtraHeaders || record.HostResolverRules != before.HostResolverRules || record.ResolverRange != before.ResolverRange || record.NoPostQuantum != before.NoPostQuantum
	return record, true, changed, nil
}

func applyNaiveSyncResult(result naiveSyncResult) {
	manager := naive.GetManager()
	for _, tag := range result.stopped {
		_ = manager.Stop(tag)
	}
	for _, tag := range result.restarted {
		if err := manager.Restart(tag); err != nil && !ignoreNaiveRuntimeError(err) {
			return
		}
	}
	for _, tag := range result.started {
		if err := manager.Start(tag); err != nil && !ignoreNaiveRuntimeError(err) {
			return
		}
	}
	if len(result.started)+len(result.restarted)+len(result.stopped) > 0 {
		(&XrayService{}).SetToNeedRestart()
	}
}

func ignoreNaiveRuntimeError(err error) bool {
	return errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist)
}

func validateNaiveStub(outbound []byte) error {
	var row struct {
		Tag      string         `json:"tag"`
		Settings map[string]any `json:"settings"`
	}
	if err := json.Unmarshal(outbound, &row); err != nil {
		return err
	}
	if err := naive.ValidateTag(row.Tag); err != nil {
		return err
	}
	proxyURL, _ := row.Settings["proxy"].(string)
	if err := naive.ValidateProxyURL(proxyURL); err != nil {
		return err
	}
	return nil
}

func asInt(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case string:
		var out int
		_, _ = fmt.Sscan(typed, &out)
		return out
	default:
		return 0
	}
}

func asString(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

func asBool(value any) bool {
	typed, ok := value.(bool)
	return ok && typed
}
