package controller

import (
	"net"
	"net/http"
	"net/url"
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
	writeWait       = 10 * time.Second
	pongWait        = 60 * time.Second
	pingPeriod      = (pongWait * 9) / 10
	clientReadLimit = 512
)

var upgrader = ws.Upgrader{
	ReadBufferSize:    32768,
	WriteBufferSize:   32768,
	EnableCompression: true,
	CheckOrigin:       checkSameOrigin,
}

// checkSameOrigin allows requests with no Origin header (same-origin or non-browser
// clients) and otherwise requires the Origin hostname to match the request hostname.
// Comparison is case-insensitive (RFC 7230 §2.7.3) and ignores port differences
// (the panel often sits behind a reverse proxy on a different port).
func checkSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Hostname() == "" {
		return false
	}
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		// IPv6 literals without a port arrive as "[::1]"; net.SplitHostPort
		// fails in that case while url.Hostname() returns the address without
		// brackets. Strip them so same-origin checks pass for bare IPv6 hosts.
		host = r.Host
		if len(host) >= 2 && host[0] == '[' && host[len(host)-1] == ']' {
			host = host[1 : len(host)-1]
		}
	}
	return strings.EqualFold(u.Hostname(), host)
}

// WebSocketController handles WebSocket connections for real-time updates.
type WebSocketController struct {
	BaseController
	hub *websocket.Hub
}

// NewWebSocketController creates a new WebSocket controller.
func NewWebSocketController(hub *websocket.Hub) *WebSocketController {
	return &WebSocketController{hub: hub}
}

// HandleWebSocket upgrades the HTTP connection and starts the read/write pumps.
func (w *WebSocketController) HandleWebSocket(c *gin.Context) {
	if !session.IsLogin(c) {
		logger.Warningf("Unauthorized WebSocket connection attempt from %s", getRemoteIp(c))
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("Failed to upgrade WebSocket connection:", err)
		return
	}

	client := websocket.NewClient(uuid.New().String())
	w.hub.Register(client)
	logger.Debugf("WebSocket client %s registered from %s", client.ID, getRemoteIp(c))

	go w.writePump(client, conn)
	go w.readPump(client, conn)
}

// readPump consumes inbound frames so the gorilla deadline/pong machinery keeps
// running. Clients send no commands today; frames are discarded.
func (w *WebSocketController) readPump(client *websocket.Client, conn *ws.Conn) {
	defer func() {
		if r := common.Recover("WebSocket readPump panic"); r != nil {
			logger.Error("WebSocket readPump panic recovered:", r)
		}
		w.hub.Unregister(client)
		conn.Close()
	}()

	conn.SetReadLimit(clientReadLimit)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			if ws.IsUnexpectedCloseError(err, ws.CloseGoingAway, ws.CloseAbnormalClosure) {
				logger.Debugf("WebSocket read error for client %s: %v", client.ID, err)
			}
			return
		}
	}
}

// writePump pushes hub messages to the connection and emits keepalive pings.
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
		case msg, ok := <-client.Send:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				conn.WriteMessage(ws.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(ws.TextMessage, msg); err != nil {
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
