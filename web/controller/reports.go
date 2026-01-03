package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// ReportsController handles client reports endpoints
type ReportsController struct {
	reportsService service.ReportsService
}

// NewReportsController creates a new reports controller
func NewReportsController(g *gin.RouterGroup) *ReportsController {
	r := &ReportsController{
		reportsService: service.ReportsService{},
	}
	r.initRouter(g)
	return r
}

func (r *ReportsController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/reports")
	g.POST("/client", r.generateClientReport)
	g.POST("/send-weekly", r.sendWeeklyReports)
	g.POST("/send-monthly", r.sendMonthlyReports)
}

// generateClientReport generates a usage report for a client
func (r *ReportsController) generateClientReport(c *gin.Context) {
	type request struct {
		Email  string `json:"email"`
		Period string `json:"period"`
	}

	var req request
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, "Invalid request", err)
		return
	}

	if req.Period == "" {
		req.Period = "weekly"
	}

	report, err := r.reportsService.GenerateClientReport(req.Email, req.Period)
	if err != nil {
		jsonMsg(c, "Failed to generate report", err)
		return
	}

	jsonObj(c, report, nil)
}

// sendWeeklyReports sends weekly reports to all clients
func (r *ReportsController) sendWeeklyReports(c *gin.Context) {
	err := r.reportsService.SendWeeklyReports()
	jsonMsg(c, "Send weekly reports", err)
}

// sendMonthlyReports sends monthly reports to all clients
func (r *ReportsController) sendMonthlyReports(c *gin.Context) {
	err := r.reportsService.SendMonthlyReports()
	jsonMsg(c, "Send monthly reports", err)
}
