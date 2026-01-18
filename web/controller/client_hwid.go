// Package controller provides HTTP handlers for client HWID management.
package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// ClientHWIDController handles HTTP requests for client HWID management.
type ClientHWIDController struct {
	clientHWIDService *service.ClientHWIDService
	clientService      *service.ClientService
}

// NewClientHWIDController creates a new ClientHWIDController.
func NewClientHWIDController(g *gin.RouterGroup) *ClientHWIDController {
	a := &ClientHWIDController{
		clientHWIDService: &service.ClientHWIDService{},
		clientService:      &service.ClientService{},
	}
	a.initRouter(g)
	return a
}

// initRouter sets up routes for client HWID management.
func (a *ClientHWIDController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/hwid")
	{
		g.GET("/list/:clientId", a.getHWIDs)
		g.POST("/add", a.addHWID)
		g.POST("/del/:id", a.removeHWID) // Changed to /del/:id to match API style
		g.POST("/deactivate/:id", a.deactivateHWID)
		g.POST("/check", a.checkHWID)
		g.POST("/register", a.registerHWID)
	}
}

// getHWIDs retrieves all HWIDs for a specific client.
func (a *ClientHWIDController) getHWIDs(c *gin.Context) {
	clientIdStr := c.Param("clientId")
	clientId, err := strconv.Atoi(clientIdStr)
	if err != nil {
		jsonMsg(c, "Invalid client ID", nil)
		return
	}

	hwids, err := a.clientHWIDService.GetHWIDsForClient(clientId)
	if err != nil {
		jsonMsg(c, "Failed to get HWIDs", err)
		return
	}

	jsonObj(c, hwids, nil)
}

// addHWID adds a new HWID for a client (manual addition by admin).
func (a *ClientHWIDController) addHWID(c *gin.Context) {
	var req struct {
		ClientId    int    `json:"clientId" form:"clientId" binding:"required"`
		HWID        string `json:"hwid" form:"hwid" binding:"required"`
		DeviceOS    string `json:"deviceOs" form:"deviceOs"`
		DeviceModel string `json:"deviceModel" form:"deviceModel"`
		OSVersion   string `json:"osVersion" form:"osVersion"`
		IPAddress   string `json:"ipAddress" form:"ipAddress"`
		UserAgent   string `json:"userAgent" form:"userAgent"`
	}

	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, "Invalid request", err)
		return
	}

	hwid, err := a.clientHWIDService.AddHWIDForClient(req.ClientId, req.HWID, req.DeviceOS, req.DeviceModel, req.OSVersion, req.IPAddress, req.UserAgent)
	if err != nil {
		jsonMsg(c, "Failed to add HWID", err)
		return
	}

	jsonObj(c, hwid, nil)
}

// removeHWID removes a HWID from a client.
func (a *ClientHWIDController) removeHWID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		jsonMsg(c, "Invalid HWID ID", nil)
		return
	}

	err = a.clientHWIDService.RemoveHWID(id)
	if err != nil {
		jsonMsg(c, "Failed to remove HWID", err)
		return
	}

	jsonMsg(c, "HWID removed successfully", nil)
}

// deactivateHWID deactivates a HWID (marks as inactive).
func (a *ClientHWIDController) deactivateHWID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		jsonMsg(c, "Invalid HWID ID", nil)
		return
	}

	err = a.clientHWIDService.DeactivateHWID(id)
	if err != nil {
		jsonMsg(c, "Failed to deactivate HWID", err)
		return
	}

	jsonMsg(c, "HWID deactivated successfully", nil)
}

// checkHWID checks if a HWID is allowed for a client.
func (a *ClientHWIDController) checkHWID(c *gin.Context) {
	var req struct {
		ClientId int    `json:"clientId" form:"clientId" binding:"required"`
		HWID     string `json:"hwid" form:"hwid" binding:"required"`
	}

	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, "Invalid request", err)
		return
	}

	allowed, err := a.clientHWIDService.CheckHWIDAllowed(req.ClientId, req.HWID)
	if err != nil {
		jsonMsg(c, "Failed to check HWID", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"obj": gin.H{
			"allowed": allowed,
		},
	})
}

// registerHWID registers a HWID for a client (called by client applications).
// This endpoint reads HWID and device metadata from HTTP headers:
//   - x-hwid (required): Hardware ID
//   - x-device-os (optional): Device operating system
//   - x-device-model (optional): Device model
//   - x-ver-os (optional): OS version
//   - user-agent (optional): User agent string
func (a *ClientHWIDController) registerHWID(c *gin.Context) {
	var req struct {
		Email string `json:"email" form:"email" binding:"required"`
	}

	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, "Invalid request", err)
		return
	}

	// Read HWID from headers (primary method)
	hwid := c.GetHeader("x-hwid")
	if hwid == "" {
		// Try alternative header name (case-insensitive)
		hwid = c.GetHeader("X-HWID")
	}
	if hwid == "" {
		jsonMsg(c, "HWID is required (x-hwid header missing)", nil)
		return
	}

	// Read device metadata from headers
	deviceOS := c.GetHeader("x-device-os")
	if deviceOS == "" {
		deviceOS = c.GetHeader("X-Device-OS")
	}
	deviceModel := c.GetHeader("x-device-model")
	if deviceModel == "" {
		deviceModel = c.GetHeader("X-Device-Model")
	}
	osVersion := c.GetHeader("x-ver-os")
	if osVersion == "" {
		osVersion = c.GetHeader("X-Ver-OS")
	}
	userAgent := c.GetHeader("User-Agent")
	ipAddress := c.ClientIP()

	// Get client by email
	client, err := a.clientService.GetClientByEmail(1, req.Email) // TODO: Get userId from session
	if err != nil {
		jsonMsg(c, "Client not found", err)
		return
	}

	// Register HWID using RegisterHWIDFromHeaders
	hwidRecord, err := a.clientHWIDService.RegisterHWIDFromHeaders(client.Id, hwid, deviceOS, deviceModel, osVersion, ipAddress, userAgent)
	if err != nil {
		// Check if error is HWID limit exceeded
		if strings.Contains(err.Error(), "HWID limit exceeded") {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"msg":     err.Error(),
			})
			return
		}
		jsonMsg(c, "Failed to register HWID", err)
		return
	}

	if hwidRecord == nil {
		// HWID tracking disabled (hwidMode = "off")
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"msg":     "HWID tracking is disabled",
		})
		return
	}

	jsonObj(c, hwidRecord, nil)
}
