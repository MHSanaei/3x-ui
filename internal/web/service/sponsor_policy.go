package service

import (
	"errors"
	"sync"
)

var ErrSponsorAdsDisabled = errors.New("sponsor ads disabled")

// A settings save waits for sponsor reads to finish, so disabling cannot race
// with a subsequent list or logo fetch. The database remains the source of truth.
var sponsorContentMu sync.RWMutex

func (s *SettingService) GetShowSponsorAds() (bool, error) {
	return s.getBool("showSponsorAds")
}

func WithSponsorContent(read func() error) error {
	sponsorContentMu.RLock()
	defer sponsorContentMu.RUnlock()
	enabled, err := (&SettingService{}).GetShowSponsorAds()
	if err != nil {
		return err
	}
	if !enabled {
		return ErrSponsorAdsDisabled
	}
	return read()
}
