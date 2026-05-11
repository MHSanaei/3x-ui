// Package service: WebSocketService owns the per-connection pump goroutines
// and bridges the HTTP-layer controller to the broadcast hub. The controller
// handles the upgrade handshake and authentication, then hands the raw
// connection to this service which takes ownership of its lifecycle.
package service

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/util/common"
	"github.com/mhsanaei/3x-ui/v3/web/websocket"

	"github.com/google/uuid"
	ws "github.com/gorilla/websocket"
)

const (
	wsWriteWait       = 10 * time.Second
	wsPongWait        = 60 * time.Second
	wsPingPeriod      = (wsPongWait * 9) / 10
	wsClientReadLimit = 512
)

// WebSocketService manages WebSocket client connections. It owns the
// read/write pumps for each accepted connection and registers/unregisters
// clients with the hub.
type WebSocketService struct {
	hub *websocket.Hub
}

// NewWebSocketService creates a service backed by the given hub.
func NewWebSocketService(hub *websocket.Hub) *WebSocketService {
	return &WebSocketService{hub: hub}
}

// HandleConnection takes ownership of an upgraded WebSocket connection:
// registers a new client, starts the read/write pumps, and returns
// immediately. The connection is closed when both pumps exit.
func (s *WebSocketService) HandleConnection(conn *ws.Conn, remoteIP string) {
	if s == nil || s.hub == nil || conn == nil {
		if conn != nil {
			conn.Close()
		}
		return
	}

	client := websocket.NewClient(uuid.New().String())
	s.hub.Register(client)
	logger.Debugf("WebSocket client %s registered from %s", client.ID, remoteIP)

	go s.writePump(client, conn)
	go s.readPump(client, conn)
}

// readPump consumes inbound frames so the gorilla deadline/pong machinery keeps
// running. Clients send no commands today; frames are discarded.
func (s *WebSocketService) readPump(client *websocket.Client, conn *ws.Conn) {
	defer func() {
		if r := common.Recover("WebSocket readPump panic"); r != nil {
			logger.Error("WebSocket readPump panic recovered:", r)
		}
		s.hub.Unregister(client)
		conn.Close()
	}()

	conn.SetReadLimit(wsClientReadLimit)
	conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
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
func (s *WebSocketService) writePump(client *websocket.Client, conn *ws.Conn) {
	ticker := time.NewTicker(wsPingPeriod)
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
			conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if !ok {
				conn.WriteMessage(ws.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(ws.TextMessage, msg); err != nil {
				logger.Debugf("WebSocket write error for client %s: %v", client.ID, err)
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := conn.WriteMessage(ws.PingMessage, nil); err != nil {
				logger.Debugf("WebSocket ping error for client %s: %v", client.ID, err)
				return
			}
		}
	}
}
