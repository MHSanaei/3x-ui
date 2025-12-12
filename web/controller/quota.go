package controller

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// QuotaController handles quota management endpoints
type QuotaController struct {
	quotaService service.QuotaService
}

// NewQuotaController creates a new quota controller
func NewQuotaController(g *gin.RouterGroup) *QuotaController {
	q := &QuotaController{
		quotaService: service.QuotaService{},
	}
	q.initRouter(g)
	return q
}

func (q *QuotaController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/quota")
	g.POST("/check", q.checkQuota)
	g.POST("/info", q.getQuotaInfo)
	g.POST("/reset", q.resetQuota)
}

// checkQuota checks quota for a client
func (q *QuotaController) checkQuota(c *gin.Context) {
	type request struct {
		Email     string `json:"email" binding:"required"`
		InboundID int    `json:"inbound_id" binding:"required"`
	}

	var req request
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, "Invalid request", err)
		return
	}

	// Validate email format (basic)
	if req.Email == "" {
		jsonMsg(c, "Email is required", errors.New("email is required"))
		return
	}

	// Get inbound
	inboundService := service.InboundService{}
	inbounds, err := inboundService.GetAllInbounds()
	if err != nil {
		jsonMsg(c, "Failed to get inbounds", err)
		return
	}

	var targetInbound *model.Inbound
	for i := range inbounds {
		if inbounds[i].Id == req.InboundID {
			targetInbound = inbounds[i]
			break
		}
	}

	if targetInbound == nil {
		jsonMsg(c, "Inbound not found", errors.New("inbound not found"))
		return
	}

	allowed, info, err := q.quotaService.CheckQuota(req.Email, targetInbound)
	if err != nil {
		jsonMsg(c, "Failed to check quota", err)
		return
	}

	jsonObj(c, gin.H{
		"allowed": allowed,
		"info":    info,
	}, nil)
}

// getQuotaInfo gets quota information for all clients
func (q *QuotaController) getQuotaInfo(c *gin.Context) {
	type request struct {
		InboundID int `json:"inbound_id" binding:"required"`
	}

	var req request
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, "Invalid request", err)
		return
	}

	// Get inbound
	inboundService := service.InboundService{}
	inbounds, err := inboundService.GetAllInbounds()
	if err != nil {
		jsonMsg(c, "Failed to get inbounds", err)
		return
	}

	var targetInbound *model.Inbound
	for i := range inbounds {
		if inbounds[i].Id == req.InboundID {
			targetInbound = inbounds[i]
			break
		}
	}

	if targetInbound == nil {
		jsonMsg(c, "Inbound not found", errors.New("inbound not found"))
		return
	}

	info, err := q.quotaService.GetQuotaInfo(targetInbound)
	if err != nil {
		jsonMsg(c, "Failed to get quota info", err)
		return
	}

	jsonObj(c, info, nil)
}

// resetQuota resets quota for a client
func (q *QuotaController) resetQuota(c *gin.Context) {
	type request struct {
		Email string `json:"email"`
	}

	var req request
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, "Invalid request", err)
		return
	}

	err := q.quotaService.ResetQuota(req.Email)
	jsonMsg(c, "Reset quota", err)
}
