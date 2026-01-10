// Package controller provides HTTP handlers for node management in multi-node mode.
package controller

import (
	"strconv"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"

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

// addNode creates a new node.
func (a *NodeController) addNode(c *gin.Context) {
	node := &model.Node{}
	err := c.ShouldBind(node)
	if err != nil {
		jsonMsg(c, "Invalid node data", err)
		return
	}

	// Log received data for debugging
	logger.Debugf("Adding node: name=%s, address=%s, apiKey=%s", node.Name, node.Address, node.ApiKey)

	// Validate API key before saving
	err = a.nodeService.ValidateApiKey(node)
	if err != nil {
		logger.Errorf("API key validation failed for node %s: %v", node.Address, err)
		jsonMsg(c, "Invalid API key or node unreachable: "+err.Error(), err)
		return
	}

	// Set default status
	if node.Status == "" {
		node.Status = "unknown"
	}

	err = a.nodeService.AddNode(node)
	if err != nil {
		jsonMsg(c, "Failed to add node", err)
		return
	}

	// Check health immediately
	go a.nodeService.CheckNodeHealth(node)

	jsonMsgObj(c, "Node added successfully", node, nil)
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

	jsonMsgObj(c, "Node health check completed", node, nil)
}

// checkAllNodes checks the health of all nodes.
func (a *NodeController) checkAllNodes(c *gin.Context) {
	a.nodeService.CheckAllNodesHealth()
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
