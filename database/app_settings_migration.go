package database

import (
	"log"

	"github.com/mhsanaei/3x-ui/v2/database/model"

	"gorm.io/gorm"
)

// initAppSettings ensures the typed app_settings row exists and is seeded.
// If missing, it migrates values from legacy key/value settings where available.
func initAppSettings() error {
	var count int64
	if err := db.Model(&model.AppSettings{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	cfg := model.NewDefaultAppSettings("")
	var legacy []model.Setting
	if err := db.Model(&model.Setting{}).Find(&legacy).Error; err != nil {
		return err
	}
	for _, setting := range legacy {
		if _, err := cfg.SetByKey(setting.Key, setting.Value); err != nil {
			log.Printf("Warning: failed to migrate legacy setting key=%s to app_settings: %v", setting.Key, err)
		}
	}

	return db.Create(cfg).Error
}

// GetAppSettings returns the authoritative typed settings row.
func GetAppSettings() (*model.AppSettings, error) {
	cfg := &model.AppSettings{}
	err := db.Model(&model.AppSettings{}).First(cfg).Error
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// GetOrCreateAppSettings returns the typed settings row, creating one when missing.
func GetOrCreateAppSettings(defaultXrayTemplate string) (*model.AppSettings, error) {
	cfg, err := GetAppSettings()
	if err == nil {
		return cfg, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	cfg = model.NewDefaultAppSettings(defaultXrayTemplate)
	if createErr := db.Create(cfg).Error; createErr != nil {
		return nil, createErr
	}
	return cfg, nil
}

// SaveAppSettings persists the typed settings row.
func SaveAppSettings(cfg *model.AppSettings) error {
	return db.Save(cfg).Error
}
