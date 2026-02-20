package service

import (
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
)

// settingsRepository encapsulates typed settings row access.
type settingsRepository struct{}

func (r *settingsRepository) getOrCreate() (*model.AppSettings, error) {
	return database.GetOrCreateAppSettings(xrayTemplateConfig)
}

func (r *settingsRepository) get(key string) (value string, recognized bool, err error) {
	cfg, err := r.getOrCreate()
	if err != nil {
		return "", false, err
	}
	value, recognized, err = cfg.GetByKey(key)
	return value, recognized, err
}

func (r *settingsRepository) set(key string, value string) (recognized bool, err error) {
	cfg, err := r.getOrCreate()
	if err != nil {
		return false, err
	}
	recognized, err = cfg.SetByKey(key, value)
	if err != nil {
		return recognized, err
	}
	if !recognized {
		return false, nil
	}
	return true, database.SaveAppSettings(cfg)
}
