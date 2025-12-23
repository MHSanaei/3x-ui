package service

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // In production, validate origin
	},
}

// WebSocketService handles WebSocket connections for real-time updates
type WebSocketService struct {
	xrayService XrayService
	clients     map[*websocket.Conn]bool
	broadcast   chan []byte
	register    chan *websocket.Conn
	unregister  chan *websocket.Conn
	mu          sync.RWMutex
	running     bool
}

// NewWebSocketService creates a new WebSocket service
func NewWebSocketService(xrayService XrayService) *WebSocketService {
	return &WebSocketService{
		xrayService: xrayService,
		clients:     make(map[*websocket.Conn]bool),
		broadcast:   make(chan []byte, 256),
		register:    make(chan *websocket.Conn),
		unregister:  make(chan *websocket.Conn),
		running:     false,
	}
}

// Run starts the WebSocket service
func (s *WebSocketService) Run() {
	if s.running {
		return
	}
	s.running = true
	defer func() { s.running = false }()

	for {
		select {
		case conn := <-s.register:
			s.mu.Lock()
			s.clients[conn] = true
			s.mu.Unlock()
			logger.Debugf("WebSocket client connected (total: %d)", len(s.clients))

			// Send initial data
			s.sendToClient(conn, DashboardData{
				Type:      "connected",
				Timestamp: time.Now(),
				Data:      map[string]interface{}{"message": "Connected to real-time updates"},
			})

		case conn := <-s.unregister:
			s.mu.Lock()
			if _, ok := s.clients[conn]; ok {
				delete(s.clients, conn)
				conn.Close()
				logger.Debugf("WebSocket client disconnected (total: %d)", len(s.clients))
			}
			s.mu.Unlock()

		case message := <-s.broadcast:
			s.mu.RLock()
			clients := make([]*websocket.Conn, 0, len(s.clients))
			for conn := range s.clients {
				clients = append(clients, conn)
			}
			s.mu.RUnlock()

			// Send to all clients with timeout
			for _, conn := range clients {
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
					logger.Debug("WebSocket write error:", err)
					select {
					case s.unregister <- conn:
					default:
					}
				}
			}
		}
	}
}

// sendToClient sends a message to a specific client
func (s *WebSocketService) sendToClient(conn *websocket.Conn, data DashboardData) {
	message, err := json.Marshal(data)
	if err != nil {
		logger.Warning("Failed to marshal WebSocket message:", err)
		return
	}

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
		logger.Debug("WebSocket write error:", err)
		select {
		case s.unregister <- conn:
		default:
		}
	}
}

// BroadcastMessage broadcasts a message to all connected clients
func (s *WebSocketService) BroadcastMessage(data interface{}) {
	message, err := json.Marshal(data)
	if err != nil {
		logger.Warning("Failed to marshal WebSocket message:", err)
		return
	}

	select {
	case s.broadcast <- message:
	default:
		logger.Warning("WebSocket broadcast channel full, dropping message")
	}
}

// RegisterClient registers a new WebSocket client
func (s *WebSocketService) RegisterClient(conn *websocket.Conn) {
	select {
	case s.register <- conn:
	default:
		logger.Warning("WebSocket register channel full")
	}
}

// UnregisterClient unregisters a WebSocket client
func (s *WebSocketService) UnregisterClient(conn *websocket.Conn) {
	select {
	case s.unregister <- conn:
	default:
		logger.Warning("WebSocket unregister channel full")
	}
}

// GetClientCount returns the number of connected clients
func (s *WebSocketService) GetClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

// DashboardData represents real-time dashboard data
type DashboardData struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// SendTrafficUpdate sends traffic update to clients
func (s *WebSocketService) SendTrafficUpdate(traffics []*xray.Traffic, clientTraffics []*xray.ClientTraffic) {
	data := DashboardData{
		Type:      "traffic",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"inbound_traffics": traffics,
			"client_traffics":  clientTraffics,
		},
	}
	s.BroadcastMessage(data)
}

// SendSystemUpdate sends system metrics update
func (s *WebSocketService) SendSystemUpdate(cpu, memory float64) {
	data := DashboardData{
		Type:      "system",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"cpu":    cpu,
			"memory": memory,
		},
	}
	s.BroadcastMessage(data)
}

// SendMetricsUpdate sends Prometheus metrics update
func (s *WebSocketService) SendMetricsUpdate(metrics map[string]interface{}) {
	data := DashboardData{
		Type:      "metrics",
		Timestamp: time.Now(),
		Data:      metrics,
	}
	s.BroadcastMessage(data)
}

// Stop stops the WebSocket service gracefully
func (s *WebSocketService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for conn := range s.clients {
		conn.Close()
		delete(s.clients, conn)
	}
	s.running = false
}
