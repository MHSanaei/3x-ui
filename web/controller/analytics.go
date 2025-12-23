package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// AnalyticsController handles analytics endpoints
type AnalyticsController struct {
	analyticsService service.AnalyticsService
}

// NewAnalyticsController creates a new analytics controller
func NewAnalyticsController(g *gin.RouterGroup) *AnalyticsController {
	a := &AnalyticsController{
		analyticsService: service.AnalyticsService{},
	}
	a.initRouter(g)
	return a
}

func (a *AnalyticsController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/analytics")
	g.POST("/hourly", a.getHourlyStats)
	g.POST("/daily", a.getDailyStats)
	g.POST("/top-clients", a.getTopClients)
}

// getHourlyStats gets hourly traffic statistics
func (a *AnalyticsController) getHourlyStats(c *gin.Context) {
	type request struct {
		InboundID int `json:"inbound_id"`
	}

	var req request
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, "Invalid request", err)
		return
	}

	stats, err := a.analyticsService.GetHourlyStats(req.InboundID)
	if err != nil {
		jsonMsg(c, "Failed to get hourly stats", err)
		return
	}

	jsonObj(c, stats, nil)
}

// getDailyStats gets daily traffic statistics
func (a *AnalyticsController) getDailyStats(c *gin.Context) {
	type request struct {
		InboundID int `json:"inbound_id"`
	}

	var req request
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, "Invalid request", err)
		return
	}

	stats, err := a.analyticsService.GetDailyStats(req.InboundID)
	if err != nil {
		jsonMsg(c, "Failed to get daily stats", err)
		return
	}

	jsonObj(c, stats, nil)
}

// getTopClients gets top clients by traffic
func (a *AnalyticsController) getTopClients(c *gin.Context) {
	type request struct {
		InboundID int `json:"inbound_id"`
		Limit     int `json:"limit"`
	}

	var req request
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, "Invalid request", err)
		return
	}

	if req.Limit <= 0 {
		req.Limit = 10
	}

	clients, err := a.analyticsService.GetTopClients(req.InboundID, req.Limit)
	if err != nil {
		jsonMsg(c, "Failed to get top clients", err)
		return
	}

	jsonObj(c, clients, nil)
}
