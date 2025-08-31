package job

import (
	"sync"
	"time"

	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/web/service"
)

const (
	resetTypeNever   = "never"
	resetTypeDaily   = "daily"
	resetTypeWeekly  = "weekly"
	resetTypeMonthly = "monthly"
)

var validResetTypes = map[string]bool{
	resetTypeNever:   true,
	resetTypeDaily:   true,
	resetTypeWeekly:  true,
	resetTypeMonthly: true,
}

type PeriodicTrafficResetJob struct {
	inboundService service.InboundService
	lastResetTimes map[string]time.Time
	mu             sync.RWMutex
}

func NewPeriodicTrafficResetJob() *PeriodicTrafficResetJob {
	return &PeriodicTrafficResetJob{
		lastResetTimes: make(map[string]time.Time),
		mu:             sync.RWMutex{},
	}
}

func (j *PeriodicTrafficResetJob) Run() {
	inbounds, err := j.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("Failed to get inbounds for traffic reset:", err)
		return
	}

	resetCount := 0
	now := time.Now()

	for _, inbound := range inbounds {
		if !j.shouldResetTraffic(inbound, now) {
			continue
		}

		if err := j.inboundService.ResetAllClientTraffics(inbound.Id); err != nil {
			logger.Warning("Failed to reset traffic for inbound", inbound.Id, ":", err)
			continue
		}

		j.updateLastResetTime(inbound, now)
		resetCount++
		logger.Infof("Reset traffic for inbound %d (%s)", inbound.Id, inbound.Remark)
	}

	if resetCount > 0 {
		logger.Infof("Periodic traffic reset completed: %d inbounds reset", resetCount)
	}
}

func (j *PeriodicTrafficResetJob) shouldResetTraffic(inbound *model.Inbound, now time.Time) bool {
	if !validResetTypes[inbound.PeriodicTrafficReset] || inbound.PeriodicTrafficReset == resetTypeNever {
		return false
	}

	resetKey := j.getResetKey(inbound)
	lastReset := j.getLastResetTime(resetKey)

	switch inbound.PeriodicTrafficReset {
	case resetTypeDaily:
		return j.shouldResetDaily(now, lastReset)
	case resetTypeWeekly:
		return j.shouldResetWeekly(now, lastReset)
	case resetTypeMonthly:
		return j.shouldResetMonthly(now, lastReset)
	default:
		return false
	}
}

func (j *PeriodicTrafficResetJob) shouldResetDaily(now, lastReset time.Time) bool {
	if lastReset.IsZero() {
		return now.Hour() == 0 && now.Minute() < 10
	}
	return now.Sub(lastReset) >= 24*time.Hour && now.Hour() == 0 && now.Minute() < 10
}

func (j *PeriodicTrafficResetJob) shouldResetWeekly(now, lastReset time.Time) bool {
	if lastReset.IsZero() {
		return now.Weekday() == time.Sunday && now.Hour() == 0 && now.Minute() < 10
	}
	return now.Sub(lastReset) >= 7*24*time.Hour && now.Weekday() == time.Sunday && now.Hour() == 0 && now.Minute() < 10
}

func (j *PeriodicTrafficResetJob) shouldResetMonthly(now, lastReset time.Time) bool {
	if lastReset.IsZero() {
		return now.Day() == 1 && now.Hour() == 0 && now.Minute() < 10
	}
	return now.Sub(lastReset) >= 28*24*time.Hour && now.Day() == 1 && now.Hour() == 0 && now.Minute() < 10
}

func (j *PeriodicTrafficResetJob) getResetKey(inbound *model.Inbound) string {
	return inbound.PeriodicTrafficReset + "_" + inbound.Tag
}

func (j *PeriodicTrafficResetJob) getLastResetTime(key string) time.Time {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.lastResetTimes[key]
}

func (j *PeriodicTrafficResetJob) updateLastResetTime(inbound *model.Inbound, resetTime time.Time) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.lastResetTimes[j.getResetKey(inbound)] = resetTime
}
