package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// WebSocketController handles WebSocket connections
type WebSocketController struct {
	wsService *service.WebSocketService
}

// NewWebSocketController creates a new WebSocket controller
func NewWebSocketController(g *gin.RouterGroup, wsService *service.WebSocketService) *WebSocketController {
	w := &WebSocketController{
		wsService: wsService,
	}
	w.initRouter(g)
	return w
}

func (w *WebSocketController) initRouter(g *gin.RouterGroup) {
	g.GET("/ws", w.handleWebSocket)
}

// handleWebSocket handles WebSocket connections
func (w *WebSocketController) handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	w.wsService.RegisterClient(conn)
	defer w.wsService.UnregisterClient(conn)

	// Keep connection alive
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
