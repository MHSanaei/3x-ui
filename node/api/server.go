// Package api provides REST API endpoints for the node service.
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mhsanaei/3x-ui/v2/logger"
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

	// API endpoints (require auth)
	api := router.Group("/api/v1")
	{
		api.POST("/apply-config", s.applyConfig)
		api.POST("/reload", s.reload)
		api.GET("/status", s.status)
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
		// Skip auth for health endpoint
		if c.Request.URL.Path == "/health" {
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

	// Validate JSON
	var configJSON json.RawMessage
	if err := json.Unmarshal(body, &configJSON); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
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

// status returns the current status of XRAY.
func (s *Server) status(c *gin.Context) {
	status := s.xrayManager.GetStatus()
	c.JSON(http.StatusOK, status)
}
