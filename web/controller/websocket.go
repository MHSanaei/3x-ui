package controller

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/common"
	"github.com/mhsanaei/3x-ui/v2/web/session"
	"github.com/mhsanaei/3x-ui/v2/web/websocket"

	"github.com/gin-gonic/gin"
	ws "github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512
)

var upgrader = ws.Upgrader{
	ReadBufferSize:  4096, // Increased from 1024 for better performance
	WriteBufferSize: 4096, // Increased from 1024 for better performance
	CheckOrigin: func(r *http.Request) bool {
		// Check origin for security
		origin := r.Header.Get("Origin")
		if origin == "" {
			// Allow connections without Origin header (same-origin requests)
			return true
		}
		// Get the host from the request
		host := r.Host
		// Extract scheme and host from origin
		originURL := origin
		// Simple check: origin should match the request host
		// This prevents cross-origin WebSocket hijacking
		if strings.HasPrefix(originURL, "http://") || strings.HasPrefix(originURL, "https://") {
			// Extract host from origin
			originHost := strings.TrimPrefix(strings.TrimPrefix(originURL, "http://"), "https://")
			if idx := strings.Index(originHost, "/"); idx != -1 {
				originHost = originHost[:idx]
			}
			if idx := strings.Index(originHost, ":"); idx != -1 {
				originHost = originHost[:idx]
			}
			// Compare hosts (without port)
			requestHost := host
			if idx := strings.Index(requestHost, ":"); idx != -1 {
				requestHost = requestHost[:idx]
			}
			return originHost == requestHost || originHost == "" || requestHost == ""
		}
		return false
	},
}

// WebSocketController handles WebSocket connections for real-time updates
type WebSocketController struct {
	BaseController
	hub *websocket.Hub
}

// NewWebSocketController creates a new WebSocket controller
func NewWebSocketController(hub *websocket.Hub) *WebSocketController {
	return &WebSocketController{
		hub: hub,
	}
}

// HandleWebSocket handles WebSocket connections
func (w *WebSocketController) HandleWebSocket(c *gin.Context) {
	// Check authentication
	if !session.IsLogin(c) {
		logger.Warningf("Unauthorized WebSocket connection attempt from %s", getRemoteIp(c))
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// Upgrade connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("Failed to upgrade WebSocket connection:", err)
		return
	}

	// Create client
	clientID := uuid.New().String()
	client := &websocket.Client{
		ID:     clientID,
		Hub:    w.hub,
		Send:   make(chan []byte, 512), // Increased from 256 to 512 to prevent overflow
		Topics: make(map[websocket.MessageType]bool),
	}

	// Register client
	w.hub.Register(client)
	logger.Debugf("WebSocket client %s registered from %s", clientID, getRemoteIp(c))

	// Start goroutines for reading and writing
	go w.writePump(client, conn)
	go w.readPump(client, conn)
}

// readPump pumps messages from the WebSocket connection to the hub
func (w *WebSocketController) readPump(client *websocket.Client, conn *ws.Conn) {
	defer func() {
		if r := common.Recover("WebSocket readPump panic"); r != nil {
			logger.Error("WebSocket readPump panic recovered:", r)
		}
		w.hub.Unregister(client)
		conn.Close()
	}()

	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	conn.SetReadLimit(maxMessageSize)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if ws.IsUnexpectedCloseError(err, ws.CloseGoingAway, ws.CloseAbnormalClosure) {
				logger.Debugf("WebSocket read error for client %s: %v", client.ID, err)
			}
			break
		}

		// Validate message size
		if len(message) > maxMessageSize {
			logger.Warningf("WebSocket message from client %s exceeds max size: %d bytes", client.ID, len(message))
			continue
		}

		// Handle incoming messages (e.g., subscription requests)
		// For now, we'll just log them
		logger.Debugf("Received WebSocket message from client %s: %s", client.ID, string(message))
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (w *WebSocketController) writePump(client *websocket.Client, conn *ws.Conn) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		if r := common.Recover("WebSocket writePump panic"); r != nil {
			logger.Error("WebSocket writePump panic recovered:", r)
		}
		ticker.Stop()
		conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				conn.WriteMessage(ws.CloseMessage, []byte{})
				return
			}

			// Send each message individually (no batching)
			// This ensures each JSON message is sent separately and can be parsed correctly
			if err := conn.WriteMessage(ws.TextMessage, message); err != nil {
				logger.Debugf("WebSocket write error for client %s: %v", client.ID, err)
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(ws.PingMessage, nil); err != nil {
				logger.Debugf("WebSocket ping error for client %s: %v", client.ID, err)
				return
			}
		}
	}
}
