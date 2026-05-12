package controller

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/web/service"
	"github.com/mhsanaei/3x-ui/v3/web/session"

	"github.com/gin-gonic/gin"
	ws "github.com/gorilla/websocket"
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

// WebSocketController handles the HTTP→WebSocket upgrade for real-time updates.
// All per-connection lifecycle (pumps, hub registration) lives in
// service.WebSocketService — this controller is HTTP-layer only.
type WebSocketController struct {
	BaseController
	service *service.WebSocketService
}

// NewWebSocketController creates a controller wired to the given service.
func NewWebSocketController(svc *service.WebSocketService) *WebSocketController {
	return &WebSocketController{service: svc}
}

// HandleWebSocket authenticates the request, upgrades the HTTP connection, and
// hands ownership of the connection off to the service.
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

	w.service.HandleConnection(conn, getRemoteIp(c))
}
