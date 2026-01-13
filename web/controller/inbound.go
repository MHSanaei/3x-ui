package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/web/session"
	"github.com/mhsanaei/3x-ui/v2/web/websocket"

	"github.com/gin-gonic/gin"
)

// InboundController handles HTTP requests related to Xray inbounds management.
type InboundController struct {
	inboundService service.InboundService
	xrayService    service.XrayService
}

// NewInboundController creates a new InboundController and sets up its routes.
func NewInboundController(g *gin.RouterGroup) *InboundController {
	a := &InboundController{}
	a.initRouter(g)
	return a
}

// initRouter initializes the routes for inbound-related operations.
func (a *InboundController) initRouter(g *gin.RouterGroup) {

	g.GET("/list", a.getInbounds)
	g.GET("/get/:id", a.getInbound)
	g.GET("/getClientTraffics/:email", a.getClientTraffics)
	g.GET("/getClientTrafficsById/:id", a.getClientTrafficsById)

	g.POST("/add", a.addInbound)
	g.POST("/del/:id", a.delInbound)
	g.POST("/update/:id", a.updateInbound)
	g.POST("/clientIps/:email", a.getClientIps)
	g.POST("/clearClientIps/:email", a.clearClientIps)
	g.POST("/addClient", a.addInboundClient)
	g.POST("/:id/delClient/:clientId", a.delInboundClient)
	g.POST("/updateClient/:clientId", a.updateInboundClient)
	g.POST("/:id/resetClientTraffic/:email", a.resetClientTraffic)
	g.POST("/resetAllTraffics", a.resetAllTraffics)
	g.POST("/resetAllClientTraffics/:id", a.resetAllClientTraffics)
	g.POST("/delDepletedClients/:id", a.delDepletedClients)
	g.POST("/import", a.importInbound)
	g.POST("/onlines", a.onlines)
	g.POST("/lastOnline", a.lastOnline)
	g.POST("/updateClientTraffic/:email", a.updateClientTraffic)
	g.POST("/:id/delClientByEmail/:email", a.delInboundClientByEmail)
}

// getInbounds retrieves the list of inbounds for the logged-in user.
func (a *InboundController) getInbounds(c *gin.Context) {
	user := session.GetLoginUser(c)
	inbounds, err := a.inboundService.GetInbounds(user.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, inbounds, nil)
}

// getInbound retrieves a specific inbound by its ID.
func (a *InboundController) getInbound(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	inbound, err := a.inboundService.GetInbound(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, inbound, nil)
}

// getClientTraffics retrieves client traffic information by email.
func (a *InboundController) getClientTraffics(c *gin.Context) {
	email := c.Param("email")
	clientTraffics, err := a.inboundService.GetClientTrafficByEmail(email)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.trafficGetError"), err)
		return
	}
	jsonObj(c, clientTraffics, nil)
}

// getClientTrafficsById retrieves client traffic information by inbound ID.
func (a *InboundController) getClientTrafficsById(c *gin.Context) {
	id := c.Param("id")
	clientTraffics, err := a.inboundService.GetClientTrafficByID(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.trafficGetError"), err)
		return
	}
	jsonObj(c, clientTraffics, nil)
}

// addInbound creates a new inbound configuration.
func (a *InboundController) addInbound(c *gin.Context) {
	// Try to get nodeIds from JSON body first (if Content-Type is application/json)
	// This must be done BEFORE ShouldBind, which reads the body
	var nodeIdsFromJSON []int
	var nodeIdFromJSON *int
	var hasNodeIdsInJSON, hasNodeIdInJSON bool
	
	if c.ContentType() == "application/json" {
		// Read raw body to extract nodeIds
		bodyBytes, err := c.GetRawData()
		if err == nil && len(bodyBytes) > 0 {
			// Parse JSON to extract nodeIds
			var jsonData map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &jsonData); err == nil {
				// Check for nodeIds array
				if nodeIdsVal, ok := jsonData["nodeIds"]; ok {
					hasNodeIdsInJSON = true
					if nodeIdsArray, ok := nodeIdsVal.([]interface{}); ok {
						for _, val := range nodeIdsArray {
							if num, ok := val.(float64); ok {
								nodeIdsFromJSON = append(nodeIdsFromJSON, int(num))
							} else if num, ok := val.(int); ok {
								nodeIdsFromJSON = append(nodeIdsFromJSON, num)
							}
						}
					} else if num, ok := nodeIdsVal.(float64); ok {
						// Single number instead of array
						nodeIdsFromJSON = append(nodeIdsFromJSON, int(num))
					} else if num, ok := nodeIdsVal.(int); ok {
						nodeIdsFromJSON = append(nodeIdsFromJSON, num)
					}
				}
				// Check for nodeId (backward compatibility)
				if nodeIdVal, ok := jsonData["nodeId"]; ok {
					hasNodeIdInJSON = true
					if num, ok := nodeIdVal.(float64); ok {
						nodeId := int(num)
						nodeIdFromJSON = &nodeId
					} else if num, ok := nodeIdVal.(int); ok {
						nodeIdFromJSON = &num
					}
				}
			}
			// Restore body for ShouldBind
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
	}
	
	inbound := &model.Inbound{}
	err := c.ShouldBind(inbound)
	if err != nil {
		logger.Errorf("Failed to bind inbound data: %v", err)
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundCreateSuccess"), err)
		return
	}
	
	user := session.GetLoginUser(c)
	inbound.UserId = user.Id
	if inbound.Listen == "" || inbound.Listen == "0.0.0.0" || inbound.Listen == "::" || inbound.Listen == "::0" {
		inbound.Tag = fmt.Sprintf("inbound-%v", inbound.Port)
	} else {
		inbound.Tag = fmt.Sprintf("inbound-%v:%v", inbound.Listen, inbound.Port)
	}

	inbound, needRestart, err := a.inboundService.AddInbound(inbound)
	if err != nil {
		logger.Errorf("Failed to add inbound: %v", err)
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	// Handle node assignment in multi-node mode
	nodeService := service.NodeService{}
	
	// Get nodeIds from form (for form-encoded requests)
	nodeIdsStr := c.PostFormArray("nodeIds")
	logger.Debugf("Received nodeIds from form: %v", nodeIdsStr)
	
	// Check if nodeIds array was provided (even if empty)
	nodeIdStr := c.PostForm("nodeId")
	
	// Determine which source to use: JSON takes precedence over form data
	useJSON := hasNodeIdsInJSON || hasNodeIdInJSON
	useForm := (len(nodeIdsStr) > 0 || nodeIdStr != "") && !useJSON
	
	if useJSON || useForm {
		var nodeIds []int
		var nodeId *int
		
		if useJSON {
			// Use data from JSON
			nodeIds = nodeIdsFromJSON
			nodeId = nodeIdFromJSON
		} else {
			// Parse nodeIds array from form
			for _, idStr := range nodeIdsStr {
				if idStr != "" {
					if id, err := strconv.Atoi(idStr); err == nil && id > 0 {
						nodeIds = append(nodeIds, id)
					}
				}
			}
			// Parse single nodeId from form
			if nodeIdStr != "" && nodeIdStr != "null" {
				if parsedId, err := strconv.Atoi(nodeIdStr); err == nil && parsedId > 0 {
					nodeId = &parsedId
				}
			}
		}
		
		if len(nodeIds) > 0 {
			// Assign to multiple nodes
			if err := nodeService.AssignInboundToNodes(inbound.Id, nodeIds); err != nil {
				logger.Errorf("Failed to assign inbound %d to nodes %v: %v", inbound.Id, nodeIds, err)
				jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
				return
			}
		} else if nodeId != nil && *nodeId > 0 {
			// Backward compatibility: single nodeId
			if err := nodeService.AssignInboundToNode(inbound.Id, *nodeId); err != nil {
				logger.Warningf("Failed to assign inbound %d to node %d: %v", inbound.Id, *nodeId, err)
			}
		}
	}

	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundCreateSuccess"), inbound, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	// Broadcast inbounds update via WebSocket
	inbounds, _ := a.inboundService.GetInbounds(user.Id)
	websocket.BroadcastInbounds(inbounds)
}

// delInbound deletes an inbound configuration by its ID.
func (a *InboundController) delInbound(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundDeleteSuccess"), err)
		return
	}
	needRestart, err := a.inboundService.DelInbound(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundDeleteSuccess"), id, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	// Broadcast inbounds update via WebSocket
	user := session.GetLoginUser(c)
	inbounds, _ := a.inboundService.GetInbounds(user.Id)
	websocket.BroadcastInbounds(inbounds)
}

// updateInbound updates an existing inbound configuration.
func (a *InboundController) updateInbound(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundUpdateSuccess"), err)
		return
	}
	
	// Try to get nodeIds from JSON body first (if Content-Type is application/json)
	var nodeIdsFromJSON []int
	var nodeIdFromJSON *int
	var hasNodeIdsInJSON, hasNodeIdInJSON bool
	
	if c.ContentType() == "application/json" {
		// Read raw body to extract nodeIds
		bodyBytes, err := c.GetRawData()
		if err == nil && len(bodyBytes) > 0 {
			// Parse JSON to extract nodeIds
			var jsonData map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &jsonData); err == nil {
				// Check for nodeIds array
				if nodeIdsVal, ok := jsonData["nodeIds"]; ok {
					hasNodeIdsInJSON = true
					if nodeIdsArray, ok := nodeIdsVal.([]interface{}); ok {
						for _, val := range nodeIdsArray {
							if num, ok := val.(float64); ok {
								nodeIdsFromJSON = append(nodeIdsFromJSON, int(num))
							} else if num, ok := val.(int); ok {
								nodeIdsFromJSON = append(nodeIdsFromJSON, num)
							}
						}
					} else if num, ok := nodeIdsVal.(float64); ok {
						// Single number instead of array
						nodeIdsFromJSON = append(nodeIdsFromJSON, int(num))
					} else if num, ok := nodeIdsVal.(int); ok {
						nodeIdsFromJSON = append(nodeIdsFromJSON, num)
					}
				}
				// Check for nodeId (backward compatibility)
				if nodeIdVal, ok := jsonData["nodeId"]; ok {
					hasNodeIdInJSON = true
					if num, ok := nodeIdVal.(float64); ok {
						nodeId := int(num)
						nodeIdFromJSON = &nodeId
					} else if num, ok := nodeIdVal.(int); ok {
						nodeIdFromJSON = &num
					}
				}
			}
			// Restore body for ShouldBind
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
	}
	
	// Get nodeIds from form (for form-encoded requests)
	nodeIdsStr := c.PostFormArray("nodeIds")
	logger.Debugf("Received nodeIds from form: %v (count: %d)", nodeIdsStr, len(nodeIdsStr))
	
	// Check if nodeIds array was provided
	nodeIdStr := c.PostForm("nodeId")
	logger.Debugf("Received nodeId from form: %s", nodeIdStr)
	
	// Check if nodeIds or nodeId was explicitly provided in the form
	_, hasNodeIds := c.GetPostForm("nodeIds")
	_, hasNodeId := c.GetPostForm("nodeId")
	logger.Debugf("Form has nodeIds: %v, has nodeId: %v", hasNodeIds, hasNodeId)
	logger.Debugf("JSON has nodeIds: %v (values: %v), has nodeId: %v (value: %v)", hasNodeIdsInJSON, nodeIdsFromJSON, hasNodeIdInJSON, nodeIdFromJSON)
	
	inbound := &model.Inbound{
		Id: id,
	}
	// Bind inbound data (nodeIds will be ignored since we handle it separately)
	err = c.ShouldBind(inbound)
	if err != nil {
		logger.Errorf("Failed to bind inbound data: %v", err)
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundUpdateSuccess"), err)
		return
	}
	inbound, needRestart, err := a.inboundService.UpdateInbound(inbound)
	if err != nil {
		logger.Errorf("Failed to update inbound: %v", err)
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	// Handle node assignment in multi-node mode
	nodeService := service.NodeService{}
	
	// Determine which source to use: JSON takes precedence over form data
	useJSON := hasNodeIdsInJSON || hasNodeIdInJSON
	useForm := (hasNodeIds || hasNodeId) && !useJSON
	
	if useJSON || useForm {
		var nodeIds []int
		var nodeId *int
		var hasNodeIdsFlag bool
		
		if useJSON {
			// Use data from JSON
			nodeIds = nodeIdsFromJSON
			nodeId = nodeIdFromJSON
			hasNodeIdsFlag = hasNodeIdsInJSON
		} else {
			// Use data from form
			hasNodeIdsFlag = hasNodeIds
			// Parse nodeIds array from form
			for _, idStr := range nodeIdsStr {
				if idStr != "" {
					if id, err := strconv.Atoi(idStr); err == nil && id > 0 {
						nodeIds = append(nodeIds, id)
					} else {
						logger.Warningf("Invalid nodeId in array: %s (error: %v)", idStr, err)
					}
				}
			}
			// Parse single nodeId from form
			if nodeIdStr != "" && nodeIdStr != "null" {
				if parsedId, err := strconv.Atoi(nodeIdStr); err == nil && parsedId > 0 {
					nodeId = &parsedId
				}
			}
		}
		
		logger.Debugf("Parsed nodeIds: %v, nodeId: %v", nodeIds, nodeId)
		
		if len(nodeIds) > 0 {
			// Assign to multiple nodes
			if err := nodeService.AssignInboundToNodes(inbound.Id, nodeIds); err != nil {
				logger.Errorf("Failed to assign inbound %d to nodes %v: %v", inbound.Id, nodeIds, err)
				jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
				return
			}
			logger.Debugf("Successfully assigned inbound %d to nodes %v", inbound.Id, nodeIds)
		} else if nodeId != nil && *nodeId > 0 {
			// Backward compatibility: single nodeId
			if err := nodeService.AssignInboundToNode(inbound.Id, *nodeId); err != nil {
				logger.Errorf("Failed to assign inbound %d to node %d: %v", inbound.Id, *nodeId, err)
				jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
				return
			}
			logger.Debugf("Successfully assigned inbound %d to node %d", inbound.Id, *nodeId)
		} else if hasNodeIdsFlag {
			// nodeIds was explicitly provided but is empty - unassign all
			if err := nodeService.UnassignInboundFromNode(inbound.Id); err != nil {
				logger.Warningf("Failed to unassign inbound %d from nodes: %v", inbound.Id, err)
			} else {
				logger.Debugf("Successfully unassigned inbound %d from all nodes", inbound.Id)
			}
		}
		// If neither nodeIds nor nodeId was provided, don't change assignments
	}

	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundUpdateSuccess"), inbound, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	// Broadcast inbounds update via WebSocket
	user := session.GetLoginUser(c)
	inbounds, _ := a.inboundService.GetInbounds(user.Id)
	websocket.BroadcastInbounds(inbounds)
}

// getClientIps retrieves the IP addresses associated with a client by email.
func (a *InboundController) getClientIps(c *gin.Context) {
	email := c.Param("email")

	ips, err := a.inboundService.GetInboundClientIps(email)
	if err != nil || ips == "" {
		jsonObj(c, "No IP Record", nil)
		return
	}

	jsonObj(c, ips, nil)
}

// clearClientIps clears the IP addresses for a client by email.
func (a *InboundController) clearClientIps(c *gin.Context) {
	email := c.Param("email")

	err := a.inboundService.ClearClientIps(email)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.updateSuccess"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.logCleanSuccess"), nil)
}

// addInboundClient adds a new client to an existing inbound.
func (a *InboundController) addInboundClient(c *gin.Context) {
	data := &model.Inbound{}
	err := c.ShouldBind(data)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundUpdateSuccess"), err)
		return
	}

	needRestart, err := a.inboundService.AddInboundClient(data)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientAddSuccess"), nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
}

// delInboundClient deletes a client from an inbound by inbound ID and client ID.
func (a *InboundController) delInboundClient(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundUpdateSuccess"), err)
		return
	}
	clientId := c.Param("clientId")

	needRestart, err := a.inboundService.DelInboundClient(id, clientId)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientDeleteSuccess"), nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
}

// updateInboundClient updates a client's configuration in an inbound.
func (a *InboundController) updateInboundClient(c *gin.Context) {
	clientId := c.Param("clientId")

	inbound := &model.Inbound{}
	err := c.ShouldBind(inbound)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundUpdateSuccess"), err)
		return
	}

	needRestart, err := a.inboundService.UpdateInboundClient(inbound, clientId)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientUpdateSuccess"), nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
}

// resetClientTraffic resets the traffic counter for a specific client in an inbound.
func (a *InboundController) resetClientTraffic(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundUpdateSuccess"), err)
		return
	}
	email := c.Param("email")

	needRestart, err := a.inboundService.ResetClientTraffic(id, email)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.resetInboundClientTrafficSuccess"), nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
}

// resetAllTraffics resets all traffic counters across all inbounds.
func (a *InboundController) resetAllTraffics(c *gin.Context) {
	err := a.inboundService.ResetAllTraffics()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	} else {
		a.xrayService.SetToNeedRestart()
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.resetAllTrafficSuccess"), nil)
}

// resetAllClientTraffics resets traffic counters for all clients in a specific inbound.
func (a *InboundController) resetAllClientTraffics(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundUpdateSuccess"), err)
		return
	}

	err = a.inboundService.ResetAllClientTraffics(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	} else {
		a.xrayService.SetToNeedRestart()
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.resetAllClientTrafficSuccess"), nil)
}

// importInbound imports an inbound configuration from provided data.
func (a *InboundController) importInbound(c *gin.Context) {
	inbound := &model.Inbound{}
	err := json.Unmarshal([]byte(c.PostForm("data")), inbound)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	user := session.GetLoginUser(c)
	inbound.Id = 0
	inbound.UserId = user.Id
	if inbound.Listen == "" || inbound.Listen == "0.0.0.0" || inbound.Listen == "::" || inbound.Listen == "::0" {
		inbound.Tag = fmt.Sprintf("inbound-%v", inbound.Port)
	} else {
		inbound.Tag = fmt.Sprintf("inbound-%v:%v", inbound.Listen, inbound.Port)
	}

	for index := range inbound.ClientStats {
		inbound.ClientStats[index].Id = 0
		inbound.ClientStats[index].Enable = true
	}

	needRestart := false
	inbound, needRestart, err = a.inboundService.AddInbound(inbound)
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundCreateSuccess"), inbound, err)
	if err == nil && needRestart {
		a.xrayService.SetToNeedRestart()
	}
}

// delDepletedClients deletes clients in an inbound who have exhausted their traffic limits.
func (a *InboundController) delDepletedClients(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundUpdateSuccess"), err)
		return
	}
	err = a.inboundService.DelDepletedClients(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.delDepletedClientsSuccess"), nil)
}

// onlines retrieves the list of currently online clients.
func (a *InboundController) onlines(c *gin.Context) {
	clients := a.inboundService.GetOnlineClients()
	jsonObj(c, clients, nil)
}

// lastOnline retrieves the last online timestamps for clients.
func (a *InboundController) lastOnline(c *gin.Context) {
	data, err := a.inboundService.GetClientsLastOnline()
	jsonObj(c, data, err)
}

// updateClientTraffic updates the traffic statistics for a client by email.
func (a *InboundController) updateClientTraffic(c *gin.Context) {
	email := c.Param("email")

	// Define the request structure for traffic update
	type TrafficUpdateRequest struct {
		Upload   int64 `json:"upload"`
		Download int64 `json:"download"`
	}

	var request TrafficUpdateRequest
	err := c.ShouldBindJSON(&request)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundUpdateSuccess"), err)
		return
	}

	err = a.inboundService.UpdateClientTrafficByEmail(email, request.Upload, request.Download)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientUpdateSuccess"), nil)
}

// delInboundClientByEmail deletes a client from an inbound by email address.
func (a *InboundController) delInboundClientByEmail(c *gin.Context) {
	inboundId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid inbound ID", err)
		return
	}

	email := c.Param("email")
	needRestart, err := a.inboundService.DelInboundClientByEmail(inboundId, email)
	if err != nil {
		jsonMsg(c, "Failed to delete client by email", err)
		return
	}

	jsonMsg(c, "Client deleted successfully", nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
}
