package controller

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/web/session"

	"github.com/gin-gonic/gin"
)

// APIController handles the main API routes for the 3x-ui panel, including inbounds and server management.
type APIController struct {
	BaseController
	inboundController *InboundController
	serverController  *ServerController
	Tgbot             service.Tgbot
}

// NewAPIController creates a new APIController instance and initializes its routes.
func NewAPIController(g *gin.RouterGroup) *APIController {
	a := &APIController{}
	a.initRouter(g)
	return a
}

// checkAPIAuth is a middleware that returns 404 for unauthenticated API requests
// to hide the existence of API endpoints from unauthorized users
func (a *APIController) checkAPIAuth(c *gin.Context) {
	if !session.IsLogin(c) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Next()
}

// initRouter sets up the API routes for inbounds, server, and other endpoints.
func (a *APIController) initRouter(g *gin.RouterGroup) {
	// Node push-logs endpoint (no session auth, uses API key)
	// Register in separate group without session auth middleware
	nodeAPI := g.Group("/panel/api/node")
	nodeAPI.POST("/push-logs", a.pushNodeLogs)
	
	// Main API group with session auth
	api := g.Group("/panel/api")
	api.Use(a.checkAPIAuth)

	// Inbounds API
	inbounds := api.Group("/inbounds")
	a.inboundController = NewInboundController(inbounds)

	// Server API
	server := api.Group("/server")
	a.serverController = NewServerController(server)

	// Extra routes
	api.GET("/backuptotgbot", a.BackuptoTgbot)
}

// BackuptoTgbot sends a backup of the panel data to Telegram bot admins.
func (a *APIController) BackuptoTgbot(c *gin.Context) {
	a.Tgbot.SendBackupToAdmins()
}

// extractPort extracts port number from URL address (e.g., "http://192.168.0.7:8080" -> "8080")
func extractPort(address string) string {
	re := regexp.MustCompile(`:(\d+)(?:/|$)`)
	matches := re.FindStringSubmatch(address)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// pushNodeLogs receives logs from a node in real-time and adds them to the panel log buffer.
// This endpoint is called by nodes when new logs are generated.
// It uses API key authentication instead of session authentication.
func (a *APIController) pushNodeLogs(c *gin.Context) {
	type PushLogRequest struct {
		ApiKey      string   `json:"apiKey" binding:"required"`      // Node API key for authentication
		NodeAddress string   `json:"nodeAddress,omitempty"`           // Node's own address for identification (optional, used when multiple nodes share API key)
		Logs        []string `json:"logs" binding:"required"`        // Array of log lines in format "timestamp level - message"
	}

	var req PushLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Find node by API key and optionally by address
	nodeService := service.NodeService{}
	nodes, err := nodeService.GetAllNodes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get nodes"})
		return
	}

	var node *model.Node
	var matchedByKey []*model.Node // Track nodes with matching API key
	
	for _, n := range nodes {
		if n.ApiKey == req.ApiKey {
			matchedByKey = append(matchedByKey, n)
			
			// If nodeAddress is provided, match by both API key and address
			if req.NodeAddress != "" {
				// Normalize addresses for comparison (remove trailing slashes, etc.)
				nodeAddr := strings.TrimSuffix(strings.TrimSpace(n.Address), "/")
				reqAddr := strings.TrimSuffix(strings.TrimSpace(req.NodeAddress), "/")
				
				// Extract port from both addresses for comparison
				// This handles cases where node uses localhost but panel has external IP
				nodePort := extractPort(nodeAddr)
				reqPort := extractPort(reqAddr)
				
				// Match by exact address or by port (if addresses don't match exactly)
				// This allows nodes to use localhost while panel has external IP
				if nodeAddr == reqAddr || (nodePort != "" && nodePort == reqPort) {
					node = n
					break
				}
			} else {
				// If no address provided, use first match (backward compatibility)
				node = n
				break
			}
		}
	}

	if node == nil {
		// Enhanced logging for debugging
		if len(matchedByKey) > 0 {
			logger.Debugf("Failed to find node: API key matches %d node(s), but address mismatch. Request address: '%s', Request port: '%s'. Matched nodes: %v", 
				len(matchedByKey), req.NodeAddress, extractPort(req.NodeAddress), 
				func() []string {
					var addrs []string
					for _, n := range matchedByKey {
						addrs = append(addrs, fmt.Sprintf("%s (port: %s)", n.Address, extractPort(n.Address)))
					}
					return addrs
				}())
		} else {
			logger.Debugf("Failed to find node: No node found with API key (received %d logs, key length: %d, key prefix: %s). Total nodes in DB: %d", 
				len(req.Logs), len(req.ApiKey), 
				func() string {
					if len(req.ApiKey) > 4 {
						return req.ApiKey[:4] + "..."
					}
					return req.ApiKey
				}(), len(nodes))
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
		return
	}

	// Log which node is sending logs (for debugging)
	logger.Debugf("Received %d logs from node: %s (ID: %d, Address: %s, API key length: %d)", 
		len(req.Logs), node.Name, node.Id, node.Address, len(req.ApiKey))

	// Process and add logs to panel buffer
	for _, logLine := range req.Logs {
		if logLine == "" {
			continue
		}

		// Parse log line: format is "timestamp level - message"
		var level string
		var message string

		if idx := strings.Index(logLine, " - "); idx != -1 {
			parts := strings.SplitN(logLine, " - ", 2)
			if len(parts) == 2 {
				levelPart := strings.TrimSpace(parts[0])
				levelFields := strings.Fields(levelPart)
				if len(levelFields) >= 2 {
					level = strings.ToUpper(levelFields[len(levelFields)-1])
					message = parts[1]
				} else {
					level = "INFO"
					message = parts[1]
				}
			} else {
				level = "INFO"
				message = logLine
			}
		} else {
			level = "INFO"
			message = logLine
		}

		// Add log to panel buffer with node prefix
		formattedMessage := fmt.Sprintf("[Node: %s] %s", node.Name, message)
		switch level {
		case "DEBUG":
			logger.Debugf("%s", formattedMessage)
		case "WARNING":
			logger.Warningf("%s", formattedMessage)
		case "ERROR":
			logger.Errorf("%s", formattedMessage)
		case "NOTICE":
			logger.Noticef("%s", formattedMessage)
		default:
			logger.Infof("%s", formattedMessage)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logs received"})
}
