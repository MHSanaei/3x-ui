package tgbot

import (
	"encoding/json"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// clientStore is a small per-client map kept as one JSON settings row, so the
// bot can remember something new about a client without a schema change.
type clientStore struct {
	mu   sync.Mutex
	load func(*Tgbot) (string, error)
	save func(*Tgbot, string) error
}

var (
	// Keyed by email. The value is the expiry the mute was made against, so an
	// extension makes it stale and reminders resume without a cleanup job.
	renewOptOut = &clientStore{
		load: func(t *Tgbot) (string, error) { return t.settingService.GetTgBotRenewOptOut() },
		save: func(t *Tgbot, v string) error { return t.settingService.SetTgBotRenewOptOut(v) },
	}
	renewReqAt = &clientStore{
		load: func(t *Tgbot) (string, error) { return t.settingService.GetTgBotRenewReqAt() },
		save: func(t *Tgbot, v string) error { return t.settingService.SetTgBotRenewReqAt(v) },
	}
	quotaWarned = &clientStore{
		load: func(t *Tgbot) (string, error) { return t.settingService.GetTgBotQuotaWarned() },
		save: func(t *Tgbot, v string) error { return t.settingService.SetTgBotQuotaWarned(v) },
	}
	selfResetAt = &clientStore{
		load: func(t *Tgbot) (string, error) { return t.settingService.GetTgBotSelfResetAt() },
		save: func(t *Tgbot, v string) error { return t.settingService.SetTgBotSelfResetAt(v) },
	}
)

// Anything unreadable degrades to "nothing recorded" rather than failing the
// pass: forgetting a mute is recoverable, skipping the whole run is not.
func parseClientBlob(blob string) map[string]int64 {
	values := map[string]int64{}
	if blob == "" {
		return values
	}

	raw := map[string]int64{}
	if err := json.Unmarshal([]byte(blob), &raw); err != nil {
		logger.Warning("tgbot: client store is unreadable:", err)
		return values
	}

	for key, value := range raw {
		if key == "" {
			continue
		}
		values[key] = value
	}
	return values
}

func (s *clientStore) all(t *Tgbot) map[string]int64 {
	blob, err := s.load(t)
	if err != nil {
		logger.Warning("tgbot: client store lookup failed:", err)
		return map[string]int64{}
	}
	return parseClientBlob(blob)
}

func (s *clientStore) get(t *Tgbot, key string) (int64, bool) {
	value, ok := s.all(t)[key]
	return value, ok
}

func (s *clientStore) put(t *Tgbot, key string, value int64) error {
	return s.update(t, func(values map[string]int64) { values[key] = value })
}

func (s *clientStore) drop(t *Tgbot, key string) error {
	return s.update(t, func(values map[string]int64) { delete(values, key) })
}

// One settings row per store, so a read-modify-write from two updates at once
// would lose one of them.
func (s *clientStore) update(t *Tgbot, apply func(map[string]int64)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	values := s.all(t)
	apply(values)
	encoded, err := json.Marshal(values)
	if err != nil {
		return err
	}
	return s.save(t, string(encoded))
}
