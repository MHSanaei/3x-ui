// Package controller provides HTTP handlers for host management in multi-node mode.
package controller

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/web/session"

	"github.com/gin-gonic/gin"
)

// HostController handles HTTP requests related to host management.
type HostController struct {
	hostService service.HostService
}

// NewHostController creates a new HostController and sets up its routes.
func NewHostController(g *gin.RouterGroup) *HostController {
	a := &HostController{
		hostService: service.HostService{},
	}
	a.initRouter(g)
	return a
}

// initRouter initializes the routes for host-related operations.
func (a *HostController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.getHosts)
	g.GET("/get/:id", a.getHost)
	g.POST("/add", a.addHost)
	g.POST("/update/:id", a.updateHost)
	g.POST("/del/:id", a.deleteHost)
}

// getHosts retrieves the list of all hosts for the current user.
func (a *HostController) getHosts(c *gin.Context) {
	user := session.GetLoginUser(c)
	hosts, err := a.hostService.GetHosts(user.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, hosts, nil)
}

// getHost retrieves a specific host by its ID.
func (a *HostController) getHost(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid host ID", err)
		return
	}
	user := session.GetLoginUser(c)
	host, err := a.hostService.GetHost(id)
	if err != nil {
		jsonMsg(c, "Failed to get host", err)
		return
	}
	if host.UserId != user.Id {
		jsonMsg(c, "Host not found or access denied", nil)
		return
	}
	jsonObj(c, host, nil)
}

// addHost creates a new host.
func (a *HostController) addHost(c *gin.Context) {
	user := session.GetLoginUser(c)
	
	// Extract inboundIds from JSON or form data
	var inboundIdsFromJSON []int
	var hasInboundIdsInJSON bool
	
	if c.ContentType() == "application/json" {
		// Read raw body to extract inboundIds
		bodyBytes, err := c.GetRawData()
		if err == nil && len(bodyBytes) > 0 {
			// Parse JSON to extract inboundIds
			var jsonData map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &jsonData); err == nil {
				// Check for inboundIds array
				if inboundIdsVal, ok := jsonData["inboundIds"]; ok {
					hasInboundIdsInJSON = true
					if inboundIdsArray, ok := inboundIdsVal.([]interface{}); ok {
						for _, val := range inboundIdsArray {
							if num, ok := val.(float64); ok {
								inboundIdsFromJSON = append(inboundIdsFromJSON, int(num))
							} else if num, ok := val.(int); ok {
								inboundIdsFromJSON = append(inboundIdsFromJSON, num)
							}
						}
					} else if num, ok := inboundIdsVal.(float64); ok {
						// Single number instead of array
						inboundIdsFromJSON = append(inboundIdsFromJSON, int(num))
					} else if num, ok := inboundIdsVal.(int); ok {
						inboundIdsFromJSON = append(inboundIdsFromJSON, num)
					}
				}
			}
			// Restore body for ShouldBind
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
	}
	
	host := &model.Host{}
	err := c.ShouldBind(host)
	if err != nil {
		jsonMsg(c, "Invalid host data", err)
		return
	}

	// Set inboundIds from JSON if available
	if hasInboundIdsInJSON {
		host.InboundIds = inboundIdsFromJSON
		logger.Debugf("AddHost: extracted inboundIds from JSON: %v", inboundIdsFromJSON)
	} else {
		// Try to get from form data
		inboundIdsStr := c.PostFormArray("inboundIds")
		if len(inboundIdsStr) > 0 {
			var inboundIds []int
			for _, idStr := range inboundIdsStr {
				if idStr != "" {
					if id, err := strconv.Atoi(idStr); err == nil && id > 0 {
						inboundIds = append(inboundIds, id)
					}
				}
			}
			host.InboundIds = inboundIds
			logger.Debugf("AddHost: extracted inboundIds from form: %v", inboundIds)
		}
	}

	logger.Debugf("AddHost: host.InboundIds before service call: %v", host.InboundIds)
	err = a.hostService.AddHost(user.Id, host)
	if err != nil {
		logger.Errorf("Failed to add host: %v", err)
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	jsonMsgObj(c, I18nWeb(c, "pages.hosts.toasts.hostCreateSuccess"), host, nil)
}

// updateHost updates an existing host.
func (a *HostController) updateHost(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid host ID", err)
		return
	}

	user := session.GetLoginUser(c)
	
	// Extract inboundIds from JSON or form data
	var inboundIdsFromJSON []int
	var hasInboundIdsInJSON bool
	
	if c.ContentType() == "application/json" {
		// Read raw body to extract inboundIds
		bodyBytes, err := c.GetRawData()
		if err == nil && len(bodyBytes) > 0 {
			// Parse JSON to extract inboundIds
			var jsonData map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &jsonData); err == nil {
				// Check for inboundIds array
				if inboundIdsVal, ok := jsonData["inboundIds"]; ok {
					hasInboundIdsInJSON = true
					if inboundIdsArray, ok := inboundIdsVal.([]interface{}); ok {
						for _, val := range inboundIdsArray {
							if num, ok := val.(float64); ok {
								inboundIdsFromJSON = append(inboundIdsFromJSON, int(num))
							} else if num, ok := val.(int); ok {
								inboundIdsFromJSON = append(inboundIdsFromJSON, num)
							}
						}
					} else if num, ok := inboundIdsVal.(float64); ok {
						// Single number instead of array
						inboundIdsFromJSON = append(inboundIdsFromJSON, int(num))
					} else if num, ok := inboundIdsVal.(int); ok {
						inboundIdsFromJSON = append(inboundIdsFromJSON, num)
					}
				}
			}
			// Restore body for ShouldBind
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
	}
	
	host := &model.Host{}
	err = c.ShouldBind(host)
	if err != nil {
		jsonMsg(c, "Invalid host data", err)
		return
	}

	// Set inboundIds from JSON if available
	if hasInboundIdsInJSON {
		host.InboundIds = inboundIdsFromJSON
		logger.Debugf("UpdateHost: extracted inboundIds from JSON: %v", inboundIdsFromJSON)
	} else {
		// Try to get from form data
		inboundIdsStr := c.PostFormArray("inboundIds")
		if len(inboundIdsStr) > 0 {
			var inboundIds []int
			for _, idStr := range inboundIdsStr {
				if idStr != "" {
					if id, err := strconv.Atoi(idStr); err == nil && id > 0 {
						inboundIds = append(inboundIds, id)
					}
				}
			}
			host.InboundIds = inboundIds
			logger.Debugf("UpdateHost: extracted inboundIds from form: %v", inboundIds)
		} else {
			logger.Debugf("UpdateHost: inboundIds not provided, keeping existing assignments")
		}
	}

	host.Id = id
	err = a.hostService.UpdateHost(user.Id, host)
	if err != nil {
		logger.Errorf("Failed to update host: %v", err)
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	jsonMsgObj(c, I18nWeb(c, "pages.hosts.toasts.hostUpdateSuccess"), host, nil)
}

// deleteHost deletes a host by ID.
func (a *HostController) deleteHost(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid host ID", err)
		return
	}

	user := session.GetLoginUser(c)
	err = a.hostService.DeleteHost(user.Id, id)
	if err != nil {
		logger.Errorf("Failed to delete host: %v", err)
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	jsonMsg(c, I18nWeb(c, "pages.hosts.toasts.hostDeleteSuccess"), nil)
}
