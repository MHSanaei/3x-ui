package job

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// Period represents the time period for traffic resets.
type Period string

// PeriodicTrafficResetJob resets traffic statistics for inbounds based on their configured reset period.
type PeriodicTrafficResetJob struct {
	inboundService service.InboundService
	clientService  service.ClientService
	period         Period
	location       *time.Location
}

// NewPeriodicTrafficResetJob creates a new periodic traffic reset job for the specified period.
func NewPeriodicTrafficResetJob(period Period, location *time.Location) *PeriodicTrafficResetJob {
	return &PeriodicTrafficResetJob{
		period:   period,
		location: location,
	}
}

func monthlyResetDue(resetDay int, now time.Time) bool {
	if resetDay < 1 {
		resetDay = 1
	}
	lastDay := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()
	return now.Day() == min(resetDay, lastDay)
}

// Run resets traffic statistics for all inbounds that match the configured reset period.
func (j *PeriodicTrafficResetJob) Run() {
	inbounds, err := j.inboundService.GetInboundsByTrafficReset(string(j.period))
	if err != nil {
		logger.Warning("Failed to get inbounds for traffic reset:", err)
		return
	}

	if j.period == "monthly" {
		now := time.Now().In(j.location)
		due := inbounds[:0]
		for _, inbound := range inbounds {
			if monthlyResetDue(inbound.TrafficResetDay, now) {
				due = append(due, inbound)
			}
		}
		inbounds = due
	}
	if len(inbounds) == 0 {
		return
	}
	logger.Infof("Running periodic traffic reset job for period: %s (%d matching inbounds)", j.period, len(inbounds))

	resetCount := 0
	for _, inbound := range inbounds {
		resetInboundErr := j.inboundService.ResetInboundTraffic(inbound.Id)
		if resetInboundErr != nil {
			logger.Warning("Failed to reset traffic for inbound", inbound.Id, ":", resetInboundErr)
		}

		resetClientErr := j.clientService.ResetAllClientTraffics(&j.inboundService, inbound.Id)
		if resetClientErr != nil {
			logger.Warning("Failed to reset traffic for all users of inbound", inbound.Id, ":", resetClientErr)
		}

		if resetInboundErr == nil && resetClientErr == nil {
			resetCount++
		}
	}

	if resetCount > 0 {
		logger.Infof("Periodic traffic reset completed: %d inbounds reset", resetCount)
	}
}
