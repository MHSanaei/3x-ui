package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/web/service"
)

// GetClientIPs returns all IPs for a client email
func (a *APIController) GetClientIPs(ctx *gin.Context) {
	email := ctx.Param("email")
	if email == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "email is required"})
		return
	}

	ipSvc := &service.IPLimitService{}
	ips, err := ipSvc.GetClientIPs(email)
	if err != nil {
		logger.Error("GetClientIPs error:", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"msg": "success",
		"ips": ips,
	})
}

// ClearClientIP removes a specific IP
func (a *APIController) ClearClientIP(ctx *gin.Context) {
	email := ctx.Param("email")
	ip := ctx.Param("ip")

	if email == "" || ip == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "email and ip are required"})
		return
	}

	ipSvc := &service.IPLimitService{}
	err := ipSvc.RemoveIP(email, ip)
	if err != nil {
		logger.Error("ClearClientIP error:", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"msg": "ip removed"})
}

// ClearAllClientIPs removes all IPs for a client
func (a *APIController) ClearAllClientIPs(ctx *gin.Context) {
	email := ctx.Param("email")
	if email == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "email is required"})
		return
	}

	db := database.GetDB()
	if err := db.Where("client_email = ?", email).Delete(&model.InboundClientIPs{}).Error; err != nil {
		logger.Error("ClearAllClientIPs error:", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"msg": "all ips cleared"})
}
