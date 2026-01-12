// Package api provides REST API endpoints for the node service.
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/mhsanaei/3x-ui/v2/logger"
	nodeConfig "github.com/mhsanaei/3x-ui/v2/node/config"
	nodeLogs "github.com/mhsanaei/3x-ui/v2/node/logs"
	"github.com/mhsanaei/3x-ui/v2/node/xray"
	"github.com/gin-gonic/gin"
)

// Server provides REST API for managing the node.
type Server struct {
	port       int
	apiKey     string
	xrayManager *xray.Manager
	httpServer *http.Server
}

// NewServer creates a new API server instance.
func NewServer(port int, apiKey string, xrayManager *xray.Manager) *Server {
	return &Server{
		port:        port,
		apiKey:      apiKey,
		xrayManager: xrayManager,
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(s.authMiddleware())

	// Health check endpoint (no auth required)
	router.GET("/health", s.health)

	// Registration endpoint (no auth required, used for initial setup)
	router.POST("/api/v1/register", s.register)

	// API endpoints (require auth)
	api := router.Group("/api/v1")
	{
		api.POST("/apply-config", s.applyConfig)
		api.POST("/reload", s.reload)
		api.POST("/force-reload", s.forceReload)
		api.GET("/status", s.status)
		api.GET("/stats", s.stats)
		api.GET("/logs", s.getLogs)
		api.GET("/service-logs", s.getServiceLogs)
	}

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logger.Infof("API server listening on port %d", s.port)
	return s.httpServer.ListenAndServe()
}

// Stop stops the HTTP server.
func (s *Server) Stop() error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Close()
}

// authMiddleware validates API key from Authorization header.
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth for health and registration endpoints
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/api/v1/register" {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing Authorization header"})
			c.Abort()
			return
		}

		// Support both "Bearer <key>" and direct key
		apiKey := authHeader
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			apiKey = authHeader[7:]
		}

		if apiKey != s.apiKey {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// health returns the health status of the node.
func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"service": "3x-ui-node",
	})
}

// applyConfig applies a new XRAY configuration.
func (s *Server) applyConfig(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	// Try to parse as JSON with optional panelUrl field
	var requestData struct {
		Config   json.RawMessage `json:"config"`
		PanelURL string          `json:"panelUrl,omitempty"`
	}

	// First try to parse as new format with panelUrl
	if err := json.Unmarshal(body, &requestData); err == nil && requestData.PanelURL != "" {
		// New format: { "config": {...}, "panelUrl": "http://..." }
		body = requestData.Config
		// Set panel URL for log pusher
		nodeLogs.SetPanelURL(requestData.PanelURL)
	} else {
		// Old format: just JSON config, validate it
		var configJSON json.RawMessage
		if err := json.Unmarshal(body, &configJSON); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}
	}

	if err := s.xrayManager.ApplyConfig(body); err != nil {
		logger.Errorf("Failed to apply config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Configuration applied successfully"})
}

// reload reloads XRAY configuration.
func (s *Server) reload(c *gin.Context) {
	if err := s.xrayManager.Reload(); err != nil {
		logger.Errorf("Failed to reload: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "XRAY reloaded successfully"})
}

// forceReload forcefully reloads XRAY even if it's hung or not running.
func (s *Server) forceReload(c *gin.Context) {
	if err := s.xrayManager.ForceReload(); err != nil {
		logger.Errorf("Failed to force reload: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "XRAY force reloaded successfully"})
}

// status returns the current status of XRAY.
func (s *Server) status(c *gin.Context) {
	status := s.xrayManager.GetStatus()
	c.JSON(http.StatusOK, status)
}

// stats returns traffic and online clients statistics from XRAY.
func (s *Server) stats(c *gin.Context) {
	// Get reset parameter (default: false)
	reset := c.DefaultQuery("reset", "false") == "true"

	stats, err := s.xrayManager.GetStats(reset)
	if err != nil {
		logger.Errorf("Failed to get stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// getLogs returns XRAY access logs from the node.
func (s *Server) getLogs(c *gin.Context) {
	// Get query parameters
	countStr := c.DefaultQuery("count", "100")
	filter := c.DefaultQuery("filter", "")

	count, err := strconv.Atoi(countStr)
	if err != nil || count < 1 || count > 10000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid count parameter (must be 1-10000)"})
		return
	}

	logs, err := s.xrayManager.GetLogs(count, filter)
	if err != nil {
		logger.Errorf("Failed to get logs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

// getServiceLogs returns service application logs from the node (node service logs and XRAY core logs).
func (s *Server) getServiceLogs(c *gin.Context) {
	// Get query parameters
	countStr := c.DefaultQuery("count", "100")
	level := c.DefaultQuery("level", "debug")

	count, err := strconv.Atoi(countStr)
	if err != nil || count < 1 || count > 10000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid count parameter (must be 1-10000)"})
		return
	}

	// Get logs from logger buffer
	logs := logger.GetLogs(count, level)
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

// register handles node registration from the panel.
// This endpoint receives an API key from the panel and saves it persistently.
// No authentication required - this is the initial setup step.
func (s *Server) register(c *gin.Context) {
	type RegisterRequest struct {
		ApiKey      string `json:"apiKey" binding:"required"`      // API key generated by panel
		PanelURL    string `json:"panelUrl,omitempty"`              // Panel URL (optional)
		NodeAddress string `json:"nodeAddress,omitempty"`          // Node address (optional)
	}

	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Check if node is already registered
	existingConfig := nodeConfig.GetConfig()
	if existingConfig.ApiKey != "" {
		logger.Warningf("Node is already registered. Rejecting registration attempt to prevent overwriting existing API key")
		c.JSON(http.StatusConflict, gin.H{
			"error": "Node is already registered. API key cannot be overwritten",
			"message": "This node has already been registered. If you need to re-register, please remove the node-config.json file first",
		})
		return
	}

	// Save API key to config file (only if not already registered)
	if err := nodeConfig.SetApiKey(req.ApiKey, false); err != nil {
		logger.Errorf("Failed to save API key: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save API key: " + err.Error()})
		return
	}

	// Update API key in server (for immediate use)
	s.apiKey = req.ApiKey

	// Save panel URL if provided
	if req.PanelURL != "" {
		if err := nodeConfig.SetPanelURL(req.PanelURL); err != nil {
			logger.Warningf("Failed to save panel URL: %v", err)
		} else {
			// Update log pusher with new panel URL and API key
			nodeLogs.SetPanelURL(req.PanelURL)
			nodeLogs.UpdateApiKey(req.ApiKey) // Update API key in log pusher
		}
	} else {
		// Even if panel URL is not provided, update API key in log pusher
		nodeLogs.UpdateApiKey(req.ApiKey)
	}

	// Save node address if provided
	if req.NodeAddress != "" {
		if err := nodeConfig.SetNodeAddress(req.NodeAddress); err != nil {
			logger.Warningf("Failed to save node address: %v", err)
		}
	}

	logger.Infof("Node registered successfully with API key (length: %d)", len(req.ApiKey))
	c.JSON(http.StatusOK, gin.H{
		"message": "Node registered successfully",
		"apiKey":  req.ApiKey, // Return API key for confirmation
	})
}
