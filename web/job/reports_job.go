package job

import (
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// ReportsJob sends periodic reports to clients
type ReportsJob struct {
	reportsService   service.ReportsService
	inboundService   service.InboundService
	analyticsService service.AnalyticsService
}

// NewReportsJob creates a new reports job
func NewReportsJob() *ReportsJob {
	return &ReportsJob{
		reportsService:   service.ReportsService{},
		inboundService:   service.InboundService{},
		analyticsService: service.AnalyticsService{},
	}
}

// Run sends weekly reports
func (j *ReportsJob) Run() {
	logger.Info("Reports job started - sending weekly reports")
	err := j.reportsService.SendWeeklyReports()
	if err != nil {
		logger.Warning("Failed to send weekly reports:", err)
	}
}

// RunMonthly sends monthly reports
func (j *ReportsJob) RunMonthly() {
	logger.Info("Reports job started - sending monthly reports")
	err := j.reportsService.SendMonthlyReports()
	if err != nil {
		logger.Warning("Failed to send monthly reports:", err)
	}
}
