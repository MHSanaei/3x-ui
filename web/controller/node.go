// Package controller provides HTTP handlers for node management in multi-node mode.
package controller

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/web/websocket"

	"github.com/gin-gonic/gin"
)

// NodeController handles HTTP requests related to node management.
type NodeController struct {
	nodeService service.NodeService
}

// NewNodeController creates a new NodeController and sets up its routes.
func NewNodeController(g *gin.RouterGroup) *NodeController {
	a := &NodeController{
		nodeService: service.NodeService{},
	}
	a.initRouter(g)
	return a
}

// initRouter initializes the routes for node-related operations.
func (a *NodeController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.getNodes)
	g.GET("/get/:id", a.getNode)
	g.POST("/add", a.addNode)
	g.POST("/update/:id", a.updateNode)
	g.POST("/del/:id", a.deleteNode)
	g.POST("/check/:id", a.checkNode)
	g.POST("/checkAll", a.checkAllNodes)
	g.POST("/reload/:id", a.reloadNode)
	g.POST("/reloadAll", a.reloadAllNodes)
	g.GET("/status/:id", a.getNodeStatus)
	g.POST("/logs/:id", a.getNodeLogs)
	g.POST("/check-connection", a.checkNodeConnection) // Check node connection without API key
	// push-logs endpoint moved to APIController to bypass session auth
}

// getNodes retrieves the list of all nodes.
func (a *NodeController) getNodes(c *gin.Context) {
	nodes, err := a.nodeService.GetAllNodes()
	if err != nil {
		jsonMsg(c, "Failed to get nodes", err)
		return
	}
	
	// Enrich nodes with assigned inbounds information
	type NodeWithInbounds struct {
		*model.Node
		Inbounds []*model.Inbound `json:"inbounds,omitempty"`
	}
	
	result := make([]NodeWithInbounds, 0, len(nodes))
	for _, node := range nodes {
		inbounds, _ := a.nodeService.GetInboundsForNode(node.Id)
		result = append(result, NodeWithInbounds{
			Node:     node,
			Inbounds: inbounds,
		})
	}
	
	jsonObj(c, result, nil)
}

// getNode retrieves a specific node by its ID.
func (a *NodeController) getNode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid node ID", err)
		return
	}
	node, err := a.nodeService.GetNode(id)
	if err != nil {
		jsonMsg(c, "Failed to get node", err)
		return
	}
	jsonObj(c, node, nil)
}

// addNode creates a new node and registers it with a generated API key.
func (a *NodeController) addNode(c *gin.Context) {
	node := &model.Node{}
	err := c.ShouldBind(node)
	if err != nil {
		jsonMsg(c, "Invalid node data", err)
		return
	}

	// Log received data for debugging
	logger.Debugf("[Node: %s] Adding node: address=%s", node.Name, node.Address)

	// Note: Connection check is done on frontend via /panel/node/check-connection endpoint
	// to avoid CORS issues. Here we proceed directly to registration.

	// Generate API key and register node
	apiKey, err := a.nodeService.RegisterNode(node)
	if err != nil {
		logger.Errorf("[Node: %s] Registration failed: %v", node.Name, err)
		jsonMsg(c, "Failed to register node: "+err.Error(), err)
		return
	}

	// Set the generated API key
	node.ApiKey = apiKey

	// Set default status
	if node.Status == "" {
		node.Status = "unknown"
	}

	// Save node to database
	err = a.nodeService.AddNode(node)
	if err != nil {
		jsonMsg(c, "Failed to add node to database", err)
		return
	}

	// Check health immediately
	go a.nodeService.CheckNodeHealth(node)

	// Broadcast nodes update via WebSocket
	a.broadcastNodesUpdate()

	logger.Infof("[Node: %s] Node added and registered successfully", node.Name)
	jsonMsgObj(c, "Node added and registered successfully", node, nil)
}

// updateNode updates an existing node.
func (a *NodeController) updateNode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid node ID", err)
		return
	}

	// Get existing node first to preserve fields that are not being updated
	existingNode, err := a.nodeService.GetNode(id)
	if err != nil {
		jsonMsg(c, "Failed to get existing node", err)
		return
	}

	// Create node with only provided fields
	node := &model.Node{Id: id}
	
	// Try to parse as JSON first (for API calls)
	contentType := c.GetHeader("Content-Type")
	if contentType == "application/json" {
		var jsonData map[string]interface{}
		if err := c.ShouldBindJSON(&jsonData); err == nil {
			// Only set fields that are provided in JSON
			if nameVal, ok := jsonData["name"].(string); ok && nameVal != "" {
				node.Name = nameVal
			}
			if addressVal, ok := jsonData["address"].(string); ok && addressVal != "" {
				node.Address = addressVal
			}
			if apiKeyVal, ok := jsonData["apiKey"].(string); ok && apiKeyVal != "" {
				node.ApiKey = apiKeyVal
			}
			// TLS settings
			if useTlsVal, ok := jsonData["useTls"].(bool); ok {
				node.UseTLS = useTlsVal
			}
			if certPathVal, ok := jsonData["certPath"].(string); ok {
				node.CertPath = certPathVal
			}
			if keyPathVal, ok := jsonData["keyPath"].(string); ok {
				node.KeyPath = keyPathVal
			}
			if insecureTlsVal, ok := jsonData["insecureTls"].(bool); ok {
				node.InsecureTLS = insecureTlsVal
			}
		}
	} else {
		// Parse as form data (default for web UI)
		// Only extract fields that are actually provided
		if name := c.PostForm("name"); name != "" {
			node.Name = name
		}
		if address := c.PostForm("address"); address != "" {
			node.Address = address
		}
		if apiKey := c.PostForm("apiKey"); apiKey != "" {
			node.ApiKey = apiKey
		}
		// TLS settings
		node.UseTLS = c.PostForm("useTls") == "true" || c.PostForm("useTls") == "on"
		if certPath := c.PostForm("certPath"); certPath != "" {
			node.CertPath = certPath
		}
		if keyPath := c.PostForm("keyPath"); keyPath != "" {
			node.KeyPath = keyPath
		}
		node.InsecureTLS = c.PostForm("insecureTls") == "true" || c.PostForm("insecureTls") == "on"
	}

	// Validate API key if it was changed
	if node.ApiKey != "" && node.ApiKey != existingNode.ApiKey {
		// Create a temporary node for validation
		validationNode := &model.Node{
			Id:      id,
			Address: node.Address,
			ApiKey:  node.ApiKey,
		}
		if validationNode.Address == "" {
			validationNode.Address = existingNode.Address
		}
		err = a.nodeService.ValidateApiKey(validationNode)
		if err != nil {
			jsonMsg(c, "Invalid API key or node unreachable: "+err.Error(), err)
			return
		}
	}

	err = a.nodeService.UpdateNode(node)
	if err != nil {
		jsonMsg(c, "Failed to update node", err)
		return
	}

	// Broadcast nodes update via WebSocket
	a.broadcastNodesUpdate()

	jsonMsgObj(c, "Node updated successfully", node, nil)
}

// deleteNode deletes a node by its ID.
func (a *NodeController) deleteNode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid node ID", err)
		return
	}

	err = a.nodeService.DeleteNode(id)
	if err != nil {
		jsonMsg(c, "Failed to delete node", err)
		return
	}

	// Broadcast nodes update via WebSocket
	a.broadcastNodesUpdate()

	jsonMsg(c, "Node deleted successfully", nil)
}

// checkNode checks the health of a specific node.
func (a *NodeController) checkNode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid node ID", err)
		return
	}

	node, err := a.nodeService.GetNode(id)
	if err != nil {
		jsonMsg(c, "Failed to get node", err)
		return
	}

	err = a.nodeService.CheckNodeHealth(node)
	if err != nil {
		jsonMsg(c, "Node health check failed", err)
		return
	}

	// Broadcast nodes update via WebSocket (to update status and response time)
	a.broadcastNodesUpdate()

	jsonMsgObj(c, "Node health check completed", node, nil)
}

// checkAllNodes checks the health of all nodes.
func (a *NodeController) checkAllNodes(c *gin.Context) {
	a.nodeService.CheckAllNodesHealth()
	// Broadcast nodes update after health check (with delay to allow all checks to complete)
	go func() {
		time.Sleep(3 * time.Second) // Wait for health checks to complete
		a.broadcastNodesUpdate()
	}()
	jsonMsg(c, "Health check initiated for all nodes", nil)
}

// getNodeStatus retrieves the detailed status of a node.
func (a *NodeController) getNodeStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid node ID", err)
		return
	}

	node, err := a.nodeService.GetNode(id)
	if err != nil {
		jsonMsg(c, "Failed to get node", err)
		return
	}

	status, err := a.nodeService.GetNodeStatus(node)
	if err != nil {
		jsonMsg(c, "Failed to get node status", err)
		return
	}

	jsonObj(c, status, nil)
}

// reloadNode reloads XRAY on a specific node.
func (a *NodeController) reloadNode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid node ID", err)
		return
	}

	node, err := a.nodeService.GetNode(id)
	if err != nil {
		jsonMsg(c, "Failed to get node", err)
		return
	}

	// Use force reload to handle hung nodes
	err = a.nodeService.ForceReloadNode(node)
	if err != nil {
		jsonMsg(c, "Failed to reload node", err)
		return
	}

	jsonMsg(c, "Node reloaded successfully", nil)
}

// reloadAllNodes reloads XRAY on all nodes.
func (a *NodeController) reloadAllNodes(c *gin.Context) {
	err := a.nodeService.ReloadAllNodes()
	if err != nil {
		jsonMsg(c, "Failed to reload some nodes", err)
		return
	}

	jsonMsg(c, "All nodes reloaded successfully", nil)
}

// getNodeLogs retrieves XRAY logs from a specific node.
func (a *NodeController) getNodeLogs(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid node ID", err)
		return
	}

	node, err := a.nodeService.GetNode(id)
	if err != nil {
		jsonMsg(c, "Failed to get node", err)
		return
	}

	count := c.DefaultPostForm("count", "100")
	filter := c.PostForm("filter")
	showDirect := c.DefaultPostForm("showDirect", "true")
	showBlocked := c.DefaultPostForm("showBlocked", "true")
	showProxy := c.DefaultPostForm("showProxy", "true")

	countInt, _ := strconv.Atoi(count)

	// Get raw logs from node
	rawLogs, err := a.nodeService.GetNodeLogs(node, countInt, filter)
	if err != nil {
		jsonMsg(c, "Failed to get logs from node", err)
		return
	}

	// Parse logs into LogEntry format (similar to ServerService.GetXrayLogs)
	type LogEntry struct {
		DateTime    time.Time `json:"DateTime"`
		FromAddress string    `json:"FromAddress"`
		ToAddress   string    `json:"ToAddress"`
		Inbound     string    `json:"Inbound"`
		Outbound    string    `json:"Outbound"`
		Email       string    `json:"Email"`
		Event       int       `json:"Event"`
	}

	const (
		Direct = iota
		Blocked
		Proxied
	)

	var freedoms []string
	var blackholes []string

	// Get tags for freedom and blackhole outbounds from default config
	settingService := service.SettingService{}
	config, err := settingService.GetDefaultXrayConfig()
	if err == nil && config != nil {
		if cfgMap, ok := config.(map[string]any); ok {
			if outbounds, ok := cfgMap["outbounds"].([]any); ok {
				for _, outbound := range outbounds {
					if obMap, ok := outbound.(map[string]any); ok {
						switch obMap["protocol"] {
						case "freedom":
							if tag, ok := obMap["tag"].(string); ok {
								freedoms = append(freedoms, tag)
							}
						case "blackhole":
							if tag, ok := obMap["tag"].(string); ok {
								blackholes = append(blackholes, tag)
							}
						}
					}
				}
			}
		}
	}

	if len(freedoms) == 0 {
		freedoms = []string{"direct"}
	}
	if len(blackholes) == 0 {
		blackholes = []string{"blocked"}
	}

	var entries []LogEntry
	for _, line := range rawLogs {
		var entry LogEntry
		parts := strings.Fields(line)

		for i, part := range parts {
			if i == 0 && len(parts) > 1 {
				dateTime, err := time.ParseInLocation("2006/01/02 15:04:05.999999", parts[0]+" "+parts[1], time.Local)
				if err == nil {
					entry.DateTime = dateTime.UTC()
				}
			}

			if part == "from" && i+1 < len(parts) {
				entry.FromAddress = strings.TrimLeft(parts[i+1], "/")
			} else if part == "accepted" && i+1 < len(parts) {
				entry.ToAddress = strings.TrimLeft(parts[i+1], "/")
			} else if strings.HasPrefix(part, "[") {
				entry.Inbound = part[1:]
			} else if strings.HasSuffix(part, "]") {
				entry.Outbound = part[:len(part)-1]
			} else if part == "email:" && i+1 < len(parts) {
				entry.Email = parts[i+1]
			}
		}

		// Determine event type
		logEntryContains := func(line string, suffixes []string) bool {
			for _, sfx := range suffixes {
				if strings.Contains(line, sfx+"]") {
					return true
				}
			}
			return false
		}

		if logEntryContains(line, freedoms) {
			if showDirect == "false" {
				continue
			}
			entry.Event = Direct
		} else if logEntryContains(line, blackholes) {
			if showBlocked == "false" {
				continue
			}
			entry.Event = Blocked
		} else {
			if showProxy == "false" {
				continue
			}
			entry.Event = Proxied
		}

		entries = append(entries, entry)
	}

	jsonObj(c, entries, nil)
}

// checkNodeConnection checks if a node is reachable (health check without API key).
// This is used during node registration to verify connectivity before registration.
func (a *NodeController) checkNodeConnection(c *gin.Context) {
	type CheckConnectionRequest struct {
		Address string `json:"address" form:"address" binding:"required"`
	}

	var req CheckConnectionRequest
	// HttpUtil.post sends data as form-urlencoded (see axios-init.js)
	// So we use ShouldBind which handles both form and JSON
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, "Invalid request: "+err.Error(), err)
		return
	}

	if req.Address == "" {
		jsonMsg(c, "Address is required", nil)
		return
	}

	// Create a temporary node object for health check
	tempNode := &model.Node{
		Address: req.Address,
	}

	// Check node health (this only uses /health endpoint, no API key required)
	status, responseTime, err := a.nodeService.CheckNodeStatus(tempNode)
	if err != nil {
		jsonMsg(c, "Node is not reachable: "+err.Error(), err)
		return
	}

	if status != "online" {
		jsonMsg(c, "Node is not online (status: "+status+")", nil)
		return
	}

	// Return response time along with success message
	jsonMsgObj(c, fmt.Sprintf("Node is reachable (response time: %d ms)", responseTime), map[string]interface{}{
		"responseTime": responseTime,
	}, nil)
}

// broadcastNodesUpdate broadcasts the current nodes list to all WebSocket clients
func (a *NodeController) broadcastNodesUpdate() {
	// Get all nodes with their inbounds
	nodes, err := a.nodeService.GetAllNodes()
	if err != nil {
		logger.Warningf("Failed to get nodes for WebSocket broadcast: %v", err)
		return
	}

	// Enrich nodes with assigned inbounds information
	type NodeWithInbounds struct {
		*model.Node
		Inbounds []*model.Inbound `json:"inbounds,omitempty"`
	}

	result := make([]NodeWithInbounds, 0, len(nodes))
	for _, node := range nodes {
		inbounds, _ := a.nodeService.GetInboundsForNode(node.Id)
		result = append(result, NodeWithInbounds{
			Node:     node,
			Inbounds: inbounds,
		})
	}

	// Broadcast via WebSocket
	websocket.BroadcastNodes(result)
}
