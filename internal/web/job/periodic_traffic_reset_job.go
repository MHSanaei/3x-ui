package job

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// periodicResetConcurrency bounds how many inbounds or clients one run resets at once:
// each waits on its node, so one at a time a few hanging nodes stretched a run for hours.
const periodicResetConcurrency = 8

// Period represents the time period for traffic resets.
type Period string

// PeriodicTrafficResetJob resets traffic statistics for inbounds based on their configured reset period.
type PeriodicTrafficResetJob struct {
	inboundService service.InboundService
	clientService  service.ClientService
	xrayService    service.XrayService
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

func forEachResetBounded(n int, reset func(i int)) {
	sem := make(chan struct{}, periodicResetConcurrency)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		sem <- struct{}{}
		common.GoRecover("periodic-traffic-reset", func() {
			defer wg.Done()
			defer func() { <-sem }()
			reset(i)
		})
	}
	wg.Wait()
}

// Run resets traffic statistics for all inbounds that match the configured reset
// period, then for the clients carrying that period on their own (#5497).
func (j *PeriodicTrafficResetJob) Run() {
	j.resetInboundsOnSchedule()
	j.resetClientsOnTheirOwnCycle()
}

func (j *PeriodicTrafficResetJob) resetInboundsOnSchedule() {
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

	var resetCount atomic.Int32
	forEachResetBounded(len(inbounds), func(i int) {
		inbound := inbounds[i]
		resetInboundErr := j.inboundService.ResetInboundTraffic(inbound.Id)
		if resetInboundErr != nil {
			logger.Warning("Failed to reset traffic for inbound", inbound.Id, ":", resetInboundErr)
		}

		resetClientErr := j.clientService.ResetAllClientTraffics(&j.inboundService, inbound.Id)
		if resetClientErr != nil {
			logger.Warning("Failed to reset traffic for all users of inbound", inbound.Id, ":", resetClientErr)
		}

		if resetInboundErr == nil && resetClientErr == nil {
			resetCount.Add(1)
		}
	})

	if count := resetCount.Load(); count > 0 {
		logger.Infof("Periodic traffic reset completed: %d inbounds reset", count)
	}
}

// resetClientsOnTheirOwnCycle resets clients whose cycle is set individually. A
// client inside an inbound on the same cycle is reset twice, which is harmless.
func (j *PeriodicTrafficResetJob) resetClientsOnTheirOwnCycle() {
	cycles, err := j.clientService.GetClientsByTrafficReset(string(j.period))
	if err != nil {
		logger.Warning("Failed to get clients for traffic reset:", err)
		return
	}

	now := time.Now().In(j.location)
	due := make([]service.ClientResetCycle, 0, len(cycles))
	for _, c := range cycles {
		// Monthly clients come due on their own day, the rule the inbound-level
		// schedule already follows.
		if j.period == "monthly" && !monthlyResetDue(c.TrafficResetDay, now) {
			continue
		}
		// A reset re-enables, which is right for a client the quota switched off
		// and wrong for one an operator switched off by hand.
		if !c.Enable && !c.Depleted() {
			continue
		}
		due = append(due, c)
	}
	if len(due) == 0 {
		return
	}
	logger.Infof("Running periodic traffic reset job for period: %s (%d matching clients)", j.period, len(due))

	var mu sync.Mutex
	resetCount := 0
	needRestart := false
	forEachResetBounded(len(due), func(i int) {
		c := due[i]
		// ResetTrafficByEmail rather than a bulk UPDATE: it is the path that also
		// propagates to the client's node and clears the MTProto sidecar quota.
		nr, resetErr := j.clientService.ResetTrafficByEmail(&j.inboundService, c.Email)
		if resetErr != nil {
			logger.Warning("Failed to reset traffic for client", c.Email, ":", resetErr)
			return
		}
		mu.Lock()
		needRestart = needRestart || nr
		resetCount++
		mu.Unlock()
	})
	// Dropping this leaves a re-enabled client absent from the running core until
	// something unrelated restarts it.
	if needRestart {
		j.xrayService.SetToNeedRestart()
	}

	if resetCount > 0 {
		logger.Infof("Periodic traffic reset completed: %d clients reset", resetCount)
	}
}
