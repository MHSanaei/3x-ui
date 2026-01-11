// Package controller provides HTTP handlers for client management.
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

// ClientController handles HTTP requests related to client management.
type ClientController struct {
	clientService service.ClientService
	xrayService   service.XrayService
}

// NewClientController creates a new ClientController and sets up its routes.
func NewClientController(g *gin.RouterGroup) *ClientController {
	a := &ClientController{
		clientService: service.ClientService{},
		xrayService:   service.XrayService{},
	}
	a.initRouter(g)
	return a
}

// initRouter initializes the routes for client-related operations.
func (a *ClientController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.getClients)
	g.GET("/get/:id", a.getClient)
	g.POST("/add", a.addClient)
	g.POST("/update/:id", a.updateClient)
	g.POST("/del/:id", a.deleteClient)
	g.POST("/resetAllTraffics", a.resetAllClientTraffics)
	g.POST("/resetTraffic/:id", a.resetClientTraffic)
	g.POST("/delDepletedClients", a.delDepletedClients)
}

// getClients retrieves the list of all clients for the current user.
func (a *ClientController) getClients(c *gin.Context) {
	user := session.GetLoginUser(c)
	clients, err := a.clientService.GetClients(user.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, clients, nil)
}

// getClient retrieves a specific client by its ID.
func (a *ClientController) getClient(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid client ID", err)
		return
	}
	user := session.GetLoginUser(c)
	client, err := a.clientService.GetClient(id)
	if err != nil {
		jsonMsg(c, "Failed to get client", err)
		return
	}
	if client.UserId != user.Id {
		jsonMsg(c, "Client not found or access denied", nil)
		return
	}
	jsonObj(c, client, nil)
}

// addClient creates a new client.
func (a *ClientController) addClient(c *gin.Context) {
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
	
	client := &model.ClientEntity{}
	err := c.ShouldBind(client)
	if err != nil {
		jsonMsg(c, "Invalid client data", err)
		return
	}
	
	// Set inboundIds from JSON if available
	if hasInboundIdsInJSON {
		client.InboundIds = inboundIdsFromJSON
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
			client.InboundIds = inboundIds
		}
	}

	needRestart, err := a.clientService.AddClient(user.Id, client)
	if err != nil {
		logger.Errorf("Failed to add client: %v", err)
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	jsonMsgObj(c, I18nWeb(c, "pages.clients.toasts.clientCreateSuccess"), client, nil)
	if needRestart {
		// In multi-node mode, this will send config to nodes immediately
		// In single mode, this will restart local Xray
		if err := a.xrayService.RestartXray(false); err != nil {
			logger.Warningf("Failed to restart Xray after client creation: %v", err)
		}
	}
}

// updateClient updates an existing client.
func (a *ClientController) updateClient(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid client ID", err)
		return
	}

	user := session.GetLoginUser(c)
	
	// Get existing client first to preserve fields not being updated
	existing, err := a.clientService.GetClient(id)
	if err != nil {
		jsonMsg(c, "Client not found", err)
		return
	}
	if existing.UserId != user.Id {
		jsonMsg(c, "Client not found or access denied", nil)
		return
	}
	
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
	
	// Use existing client as base and update only provided fields
	client := existing
	
	// Try to bind only provided fields - use ShouldBindJSON for JSON requests
	if c.ContentType() == "application/json" {
		var updateData map[string]interface{}
		if err := c.ShouldBindJSON(&updateData); err == nil {
			// Update only fields that are present in the request
			if email, ok := updateData["email"].(string); ok && email != "" {
				client.Email = email
			}
			if uuid, ok := updateData["uuid"].(string); ok && uuid != "" {
				client.UUID = uuid
			}
			if security, ok := updateData["security"].(string); ok && security != "" {
				client.Security = security
			}
			if password, ok := updateData["password"].(string); ok && password != "" {
				client.Password = password
			}
			if flow, ok := updateData["flow"].(string); ok && flow != "" {
				client.Flow = flow
			}
			if limitIP, ok := updateData["limitIp"].(float64); ok {
				client.LimitIP = int(limitIP)
			} else if limitIP, ok := updateData["limitIp"].(int); ok {
				client.LimitIP = limitIP
			}
			if totalGB, ok := updateData["totalGB"].(float64); ok {
				client.TotalGB = totalGB
			} else if totalGB, ok := updateData["totalGB"].(int); ok {
				client.TotalGB = float64(totalGB)
			} else if totalGB, ok := updateData["totalGB"].(int64); ok {
				client.TotalGB = float64(totalGB)
			}
			if expiryTime, ok := updateData["expiryTime"].(float64); ok {
				client.ExpiryTime = int64(expiryTime)
			} else if expiryTime, ok := updateData["expiryTime"].(int64); ok {
				client.ExpiryTime = expiryTime
			}
			if enable, ok := updateData["enable"].(bool); ok {
				client.Enable = enable
			}
			if tgID, ok := updateData["tgId"].(float64); ok {
				client.TgID = int64(tgID)
			} else if tgID, ok := updateData["tgId"].(int64); ok {
				client.TgID = tgID
			}
			if subID, ok := updateData["subId"].(string); ok && subID != "" {
				client.SubID = subID
			}
			if comment, ok := updateData["comment"].(string); ok && comment != "" {
				client.Comment = comment
			}
			if reset, ok := updateData["reset"].(float64); ok {
				client.Reset = int(reset)
			} else if reset, ok := updateData["reset"].(int); ok {
				client.Reset = reset
			}
			if hwidEnabled, ok := updateData["hwidEnabled"].(bool); ok {
				client.HWIDEnabled = hwidEnabled
			}
			if maxHwid, ok := updateData["maxHwid"].(float64); ok {
				client.MaxHWID = int(maxHwid)
			} else if maxHwid, ok := updateData["maxHwid"].(int); ok {
				client.MaxHWID = maxHwid
			}
		}
	} else {
		// For form data, use ShouldBind
		updateClient := &model.ClientEntity{}
		if err := c.ShouldBind(updateClient); err == nil {
			// Update only non-empty fields
			if updateClient.Email != "" {
				client.Email = updateClient.Email
			}
			if updateClient.UUID != "" {
				client.UUID = updateClient.UUID
			}
			if updateClient.Security != "" {
				client.Security = updateClient.Security
			}
			if updateClient.Password != "" {
				client.Password = updateClient.Password
			}
			if updateClient.Flow != "" {
				client.Flow = updateClient.Flow
			}
			if updateClient.LimitIP > 0 {
				client.LimitIP = updateClient.LimitIP
			}
			if updateClient.TotalGB > 0 {
				client.TotalGB = updateClient.TotalGB
			}
			if updateClient.ExpiryTime != 0 {
				client.ExpiryTime = updateClient.ExpiryTime
			}
			// Always update enable if it's in the request (even if false)
			enableStr := c.PostForm("enable")
			if enableStr != "" {
				client.Enable = enableStr == "true" || enableStr == "1"
			}
			if updateClient.TgID > 0 {
				client.TgID = updateClient.TgID
			}
			if updateClient.SubID != "" {
				client.SubID = updateClient.SubID
			}
			if updateClient.Comment != "" {
				client.Comment = updateClient.Comment
			}
			if updateClient.Reset > 0 {
				client.Reset = updateClient.Reset
			}
		}
	}
	
	// Set inboundIds from JSON if available
	if hasInboundIdsInJSON {
		client.InboundIds = inboundIdsFromJSON
		logger.Debugf("UpdateClient: extracted inboundIds from JSON: %v", inboundIdsFromJSON)
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
			client.InboundIds = inboundIds
			logger.Debugf("UpdateClient: extracted inboundIds from form: %v", inboundIds)
		} else {
			logger.Debugf("UpdateClient: inboundIds not provided, keeping existing assignments")
		}
	}

	client.Id = id
	logger.Debugf("UpdateClient: client.InboundIds = %v", client.InboundIds)
	needRestart, err := a.clientService.UpdateClient(user.Id, client)
	if err != nil {
		logger.Errorf("Failed to update client: %v", err)
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	jsonMsgObj(c, I18nWeb(c, "pages.clients.toasts.clientUpdateSuccess"), client, nil)
	if needRestart {
		// In multi-node mode, this will send config to nodes immediately
		// In single mode, this will restart local Xray
		if err := a.xrayService.RestartXray(false); err != nil {
			logger.Warningf("Failed to restart Xray after client update: %v", err)
		}
	}
}

// deleteClient deletes a client by ID.
func (a *ClientController) deleteClient(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid client ID", err)
		return
	}

	user := session.GetLoginUser(c)
	needRestart, err := a.clientService.DeleteClient(user.Id, id)
	if err != nil {
		logger.Errorf("Failed to delete client: %v", err)
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	jsonMsg(c, I18nWeb(c, "pages.clients.toasts.clientDeleteSuccess"), nil)
	if needRestart {
		// In multi-node mode, this will send config to nodes immediately
		// In single mode, this will restart local Xray
		if err := a.xrayService.RestartXray(false); err != nil {
			logger.Warningf("Failed to restart Xray after client deletion: %v", err)
		}
	}
}

// resetAllClientTraffics resets traffic counters for all clients of the current user.
func (a *ClientController) resetAllClientTraffics(c *gin.Context) {
	user := session.GetLoginUser(c)
	needRestart, err := a.clientService.ResetAllClientTraffics(user.Id)
	if err != nil {
		logger.Errorf("Failed to reset all client traffics: %v", err)
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.resetAllClientTrafficSuccess"), nil)
	if needRestart {
		// In multi-node mode, this will send config to nodes immediately
		// In single mode, this will restart local Xray
		if err := a.xrayService.RestartXray(false); err != nil {
			logger.Warningf("Failed to restart Xray after resetting all client traffics: %v", err)
		}
	}
}

// resetClientTraffic resets traffic counter for a specific client.
func (a *ClientController) resetClientTraffic(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid client ID", err)
		return
	}

	user := session.GetLoginUser(c)
	needRestart, err := a.clientService.ResetClientTraffic(user.Id, id)
	if err != nil {
		logger.Errorf("Failed to reset client traffic: %v", err)
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.resetInboundClientTrafficSuccess"), nil)
	if needRestart {
		// In multi-node mode, this will send config to nodes immediately
		// In single mode, this will restart local Xray
		if err := a.xrayService.RestartXray(false); err != nil {
			logger.Warningf("Failed to restart Xray after client traffic reset: %v", err)
		}
	}
}

// delDepletedClients deletes clients that have exhausted their traffic limits or expired.
func (a *ClientController) delDepletedClients(c *gin.Context) {
	user := session.GetLoginUser(c)
	count, needRestart, err := a.clientService.DelDepletedClients(user.Id)
	if err != nil {
		logger.Errorf("Failed to delete depleted clients: %v", err)
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	
	if count > 0 {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.delDepletedClientsSuccess"), nil)
		if needRestart {
			// In multi-node mode, this will send config to nodes immediately
			// In single mode, this will restart local Xray
			if err := a.xrayService.RestartXray(false); err != nil {
				logger.Warningf("Failed to restart Xray after deleting depleted clients: %v", err)
			}
		}
	} else {
		jsonMsg(c, "No depleted clients found", nil)
	}
}
