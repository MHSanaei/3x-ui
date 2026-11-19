package service

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/trafficguard"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// UserTrafficAlertService consumes only the existing local traffic job's deltas.
// It makes no extra Xray RPC and never changes quotas, clients or core config.
type UserTrafficAlertService struct {
	mu          sync.Mutex
	lastPoll    time.Time
	lastEpoch   *xray.Process
	lastCleanup time.Time
}

func (s *UserTrafficAlertService) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastPoll = time.Time{}
	s.lastEpoch = nil
}

func userTrafficAlertConfig(db *gorm.DB) (trafficguard.Config, error) {
	keys := []string{"smtpEnable", "smtpEnabledEvents", "smtpUserRateMB", "smtpUserRateSeconds", "smtpUserWindowGB", "smtpUserWindowMinutes", "smtpUserCooldownMinutes"}
	var settings []model.Setting
	if err := db.Select("key", "value").Where("key IN ?", keys).Find(&settings).Error; err != nil {
		return trafficguard.Config{}, err
	}
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		values[key] = defaultValueMap[key]
	}
	for _, row := range settings {
		values[row.Key] = row.Value
	}
	windowMinutes, _ := strconv.ParseInt(values["smtpUserWindowMinutes"], 10, 64)
	if windowMinutes < 1 || windowMinutes > 1440 {
		windowMinutes = 30
	}
	c := trafficguard.Config{WindowMS: windowMinutes * 60000}
	if values["smtpEnable"] != "true" {
		return c, nil
	}
	for _, name := range strings.Split(values["smtpEnabledEvents"], ",") {
		switch strings.TrimSpace(name) {
		case string(eventbus.EventUserRateHigh):
			c.RateEnabled = true
		case string(eventbus.EventUserTrafficHigh):
			c.WindowEnabled = true
		}
	}
	if !c.RateEnabled && !c.WindowEnabled {
		return c, nil
	}
	params := []struct {
		key        string
		dest       *int64
		scale, max int64
	}{
		{"smtpUserRateMB", &c.RateBytesPerSecond, 1_000_000, 100000},
		{"smtpUserRateSeconds", &c.RateDurationMS, 1000, 86400},
		{"smtpUserWindowGB", &c.WindowBytes, 1_000_000_000, 1000000},
		{"smtpUserWindowMinutes", &c.WindowMS, 60000, 1440},
		{"smtpUserCooldownMinutes", &c.CooldownMS, 60000, 10080},
	}
	for _, p := range params {
		v, err := strconv.ParseInt(values[p.key], 10, 64)
		if err != nil || v < 1 || v > p.max {
			return c, fmt.Errorf("invalid %s", p.key)
		}
		*p.dest = v * p.scale
	}
	return c, nil
}

// Observe drops startup/core-change baselines and gaps longer than 15 seconds.
func (s *UserTrafficAlertService) Observe(now time.Time, epoch *xray.Process, deltas []*xray.ClientTraffic) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, previousEpoch := s.lastPoll, s.lastEpoch
	s.lastPoll, s.lastEpoch = now, epoch
	if previous.IsZero() || epoch != previousEpoch || now.Sub(previous) <= 0 || now.Sub(previous) > 15*time.Second {
		return nil
	}
	db := database.GetDB()
	c, err := userTrafficAlertConfig(db)
	if err != nil {
		return err
	}
	start, end := previous.UnixMilli(), now.UnixMilli()
	if end <= start {
		return nil
	}
	cleanup := s.lastCleanup.IsZero() || now.Sub(s.lastCleanup) >= time.Minute
	if !c.RateEnabled && !c.WindowEnabled {
		if cleanup {
			if err := db.Transaction(func(tx *gorm.DB) error { return pruneUserTrafficAlerts(tx, end, c.WindowMS) }); err != nil {
				return err
			}
			s.lastCleanup = now
		}
		return nil
	}
	amounts := map[string]int64{}
	for _, ct := range deltas {
		if ct == nil || ct.Email == "" || ct.Up < 0 || ct.Down < 0 || ct.Up > math.MaxInt64-ct.Down {
			continue
		}
		email := strings.ToLower(strings.TrimSpace(ct.Email))
		used := ct.Up + ct.Down
		if email == "" || amounts[email] > math.MaxInt64-used {
			continue
		}
		amounts[email] += used
	}
	// Missing users deliberately receive no step. Their next sample's timestamp
	// will not touch LastEnd, so their sustained condition cannot cross the gap.
	emails := make([]string, 0, len(amounts))
	samples := make([]model.UserTrafficAlertSample, 0, len(amounts))
	for email, used := range amounts {
		emails = append(emails, email)
		if used > 0 && c.WindowEnabled {
			samples = append(samples, model.UserTrafficAlertSample{Email: email, StartMS: start, EndMS: end, Bytes: used})
		}
	}
	var events []eventbus.Event
	err = db.Transaction(func(tx *gorm.DB) error {
		if cleanup {
			if err := pruneUserTrafficAlerts(tx, end, c.WindowMS); err != nil {
				return err
			}
		}
		if len(emails) == 0 {
			return nil
		}
		if len(samples) > 0 {
			if err := tx.CreateInBatches(samples, 100).Error; err != nil {
				return err
			}
		}
		var rows []model.UserTrafficAlertState
		for offset := 0; offset < len(emails); offset += 500 {
			var batch []model.UserTrafficAlertState
			if err := tx.Where("email IN ?", emails[offset:min(offset+500, len(emails))]).Find(&batch).Error; err != nil {
				return err
			}
			rows = append(rows, batch...)
		}
		states := make(map[string]trafficguard.State, len(rows))
		updated := make(map[string]int64, len(rows))
		for _, row := range rows {
			var state trafficguard.State
			if err := json.Unmarshal([]byte(row.Payload), &state); err != nil {
				return fmt.Errorf("invalid persisted traffic alert state: %w", err)
			}
			states[row.Email] = state
			updated[row.Email] = row.UpdatedMS
		}
		windows := map[string]int64{}
		if c.WindowEnabled {
			var sums []struct {
				Email string
				Total int64
			}
			// Exclude partial boundary intervals so bytes before the window never count.
			for offset := 0; offset < len(emails); offset += 500 {
				var batch []struct {
					Email string
					Total int64
				}
				if err := tx.Model(&model.UserTrafficAlertSample{}).Select("email, SUM(bytes) AS total").
					Where("email IN ? AND end_ms > ? AND start_ms >= ? AND end_ms <= ?", emails[offset:min(offset+500, len(emails))], end-c.WindowMS, end-c.WindowMS, end).
					Group("email").Scan(&batch).Error; err != nil {
					return err
				}
				sums = append(sums, batch...)
			}
			for _, sum := range sums {
				windows[sum.Email] = sum.Total
			}
		}
		rows = rows[:0]
		for email, used := range amounts {
			state := states[email]
			for _, alert := range trafficguard.Step(&state, c, start, end, used, windows[email]) {
				kind := eventbus.EventUserRateHigh
				if alert.Rule == "window" {
					kind = eventbus.EventUserTrafficHigh
				}
				events = append(events, eventbus.Event{Type: kind, Source: email, Timestamp: now, Data: &eventbus.UserTrafficData{Email: email, Bytes: alert.Bytes, Threshold: alert.Threshold, DurationMS: alert.DurationMS}})
			}
			previousState, exists := states[email]
			if !userTrafficStateNeedsWrite(previousState, state, exists, updated[email], end) {
				continue
			}
			payload, err := json.Marshal(state)
			if err != nil {
				return err
			}
			rows = append(rows, model.UserTrafficAlertState{Email: email, Payload: string(payload), UpdatedMS: end})
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "email"}}, DoUpdates: clause.AssignmentColumns([]string{"payload", "updated_ms"})}).CreateInBatches(rows, 100).Error
	})
	if err != nil {
		return err
	}
	if cleanup {
		s.lastCleanup = now
	}
	for _, e := range events {
		data := e.Data.(*eventbus.UserTrafficData)
		// Fetch only presentation fields; client credentials are never loaded.
		var client struct{ Comment string }
		if err := db.Model(&model.ClientRecord{}).Select("comment").Where("email = ?", data.Email).Take(&client).Error; err == nil {
			data.Name = client.Comment
		}
		logger.Warningf("user traffic alert: rule=%s user=%q bytes=%d threshold=%d duration_ms=%d", e.Type, e.Source, data.Bytes, data.Threshold, data.DurationMS)
		if eventBus != nil {
			eventBus.Publish(e)
		}
	}
	return nil
}

func pruneUserTrafficAlerts(tx *gorm.DB, end, windowMS int64) error {
	if err := tx.Where("end_ms < ?", end-windowMS).Delete(&model.UserTrafficAlertSample{}).Error; err != nil {
		return err
	}
	return tx.Where("updated_ms < ?", end-14*86400000).Delete(&model.UserTrafficAlertState{}).Error
}

// Idle users need neither five-second state writes nor empty baseline rows.
func userTrafficStateNeedsWrite(before, after trafficguard.State, exists bool, updated, now int64) bool {
	if !exists {
		return after.RateSince != 0 || after.RateNotified || after.WindowNotified || after.LastRateAlert != 0 || after.LastWindowAlert != 0
	}
	if before.RateSince == 0 && after.RateSince == 0 {
		before.LastEnd, after.LastEnd = 0, 0
	}
	return before != after || now-updated >= int64(time.Hour/time.Millisecond)
}
