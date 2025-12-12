package controller

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// AuditController handles audit log operations
type AuditController struct {
	auditService service.AuditLogService
}

// NewAuditController creates a new audit controller
func NewAuditController(g *gin.RouterGroup) *AuditController {
	a := &AuditController{
		auditService: service.AuditLogService{},
	}
	a.initRouter(g)
	return a
}

func (a *AuditController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/audit")
	g.POST("/logs", a.getAuditLogs)
	g.POST("/clean", a.cleanOldLogs)
}

// getAuditLogs retrieves audit logs with filters
func (a *AuditController) getAuditLogs(c *gin.Context) {
	type request struct {
		UserID    int    `json:"user_id"`
		Action    string `json:"action"`
		Resource  string `json:"resource"`
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
		Limit     int    `json:"limit"`
		Offset    int    `json:"offset"`
	}

	var req request
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, "Invalid request", err)
		return
	}

	// Validate and set defaults
	if req.Limit <= 0 || req.Limit > 1000 {
		req.Limit = 50
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	var startTime, endTime *time.Time
	if req.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, req.StartTime); err == nil {
			startTime = &t
		}
	}
	if req.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, req.EndTime); err == nil {
			endTime = &t
		}
	}

	logs, total, err := a.auditService.GetAuditLogs(req.UserID, req.Limit, req.Offset, req.Action, req.Resource, startTime, endTime)
	if err != nil {
		jsonMsg(c, "Failed to get audit logs", err)
		return
	}

	jsonObj(c, gin.H{
		"logs":  logs,
		"total": total,
	}, nil)
}

// cleanOldLogs removes old audit logs
func (a *AuditController) cleanOldLogs(c *gin.Context) {
	type request struct {
		Days int `json:"days"`
	}

	var req request
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, "Invalid request", err)
		return
	}

	if req.Days <= 0 {
		req.Days = 90
	}

	err := a.auditService.CleanOldLogs(req.Days)
	jsonMsg(c, "Clean old logs", err)
}
