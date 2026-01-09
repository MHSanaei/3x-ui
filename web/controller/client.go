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
	err = c.ShouldBind(client)
	if err != nil {
		jsonMsg(c, "Invalid client data", err)
		return
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
