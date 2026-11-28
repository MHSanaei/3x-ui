package service

import (
	"strconv"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

type SubscriptionRenderSettings struct {
	HostSortMode            string
	Datepicker              string
	ShowIdentityOnAllLinks  bool
	InfoNodeEnabled         bool
	ExpiredTemplate         string
	TrafficDepletedTemplate string
	CalendarExpireInclusive bool
	CalendarExpireLocation  *time.Location
	// PublicHost is subDomain, else webDomain: the address links fall back to.
	PublicHost string
}

// GetSubscriptionRenderSettings loads the presentation settings for one preparation.
// Fall back to single-key reads on query failure to preserve partial results.
func (s *SettingService) GetSubscriptionRenderSettings() SubscriptionRenderSettings {
	keys := []string{
		"subHostSortMode", "datepicker", "subShowIdentityOnAllLinks", "subInfoNodeEnable",
		"subExpiredTemplate", "subTrafficDepletedTemplate", "subCalendarExpireInclusive", "timeLocation",
		"subDomain", "webDomain",
	}
	get := s.getString
	var rows []model.Setting
	if err := database.GetDB().Select("key", "value").Where("key IN ?", keys).Order("id").Find(&rows).Error; err == nil {
		values := make(map[string]string, len(keys))
		for _, row := range rows {
			// The single-key getters use First, so keep the lowest id for duplicate keys.
			if _, exists := values[row.Key]; !exists {
				values[row.Key] = row.Value
			}
		}
		get = func(key string) (string, error) {
			if value, exists := values[key]; exists {
				return value, nil
			}
			return defaultValueMap[key], nil
		}
	}

	getBool := func(key string) bool {
		value, err := get(key)
		if err != nil {
			return false
		}
		enabled, _ := strconv.ParseBool(effectiveSettingValue(key, value))
		return enabled
	}
	var settings SubscriptionRenderSettings
	settings.HostSortMode, _ = get("subHostSortMode")
	settings.Datepicker, _ = get("datepicker")
	settings.ShowIdentityOnAllLinks = getBool("subShowIdentityOnAllLinks")
	settings.InfoNodeEnabled = getBool("subInfoNodeEnable")
	settings.ExpiredTemplate, _ = get("subExpiredTemplate")
	settings.TrafficDepletedTemplate, _ = get("subTrafficDepletedTemplate")
	settings.CalendarExpireInclusive = getBool("subCalendarExpireInclusive")
	if settings.CalendarExpireInclusive {
		if value, err := get("timeLocation"); err == nil {
			settings.CalendarExpireLocation = loadSettingTimeLocation(value)
		}
	}
	settings.PublicHost, _ = get("subDomain")
	if settings.PublicHost == "" {
		settings.PublicHost, _ = get("webDomain")
	}
	return settings
}
