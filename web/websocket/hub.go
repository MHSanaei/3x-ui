// Package websocket provides WebSocket hub for real-time updates and notifications.
package websocket

import (
	"context"
	"encoding/json"
	"runtime"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v2/logger"
)

// MessageType represents the type of WebSocket message
type MessageType string

const (
	MessageTypeStatus       MessageType = "status"       // Server status update
	MessageTypeTraffic      MessageType = "traffic"      // Traffic statistics update
	MessageTypeInbounds     MessageType = "inbounds"     // Inbounds list update
	MessageTypeNotification MessageType = "notification" // System notification
	MessageTypeXrayState    MessageType = "xray_state"   // Xray state change
	MessageTypeOutbounds    MessageType = "outbounds"    // Outbounds list update
)

// Message represents a WebSocket message
type Message struct {
	Type    MessageType `json:"type"`
	Payload any         `json:"payload"`
	Time    int64       `json:"time"`
}

// Client represents a WebSocket client connection
type Client struct {
	ID     string
	Send   chan []byte
	Hub    *Hub
	Topics map[MessageType]bool // Subscribed topics
}

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Inbound messages from clients
	broadcast chan []byte

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Mutex for thread-safe operations
	mu sync.RWMutex

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc

	// Worker pool for parallel broadcasting
	workerPoolSize int
	broadcastWg    sync.WaitGroup
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	ctx, cancel := context.WithCancel(context.Background())

	// Calculate optimal worker pool size (CPU cores * 2, but max 100)
	workerPoolSize := runtime.NumCPU() * 2
	if workerPoolSize > 100 {
		workerPoolSize = 100
	}
	if workerPoolSize < 10 {
		workerPoolSize = 10
	}

	return &Hub{
		clients:        make(map[*Client]bool),
		broadcast:      make(chan []byte, 2048), // Increased from 256 to 2048 for high load
		register:       make(chan *Client, 100), // Buffered channel for fast registration
		unregister:     make(chan *Client, 100), // Buffered channel for fast unregistration
		ctx:            ctx,
		cancel:         cancel,
		workerPoolSize: workerPoolSize,
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("WebSocket hub panic recovered:", r)
			// Restart the hub loop
			go h.Run()
		}
	}()

	for {
		select {
		case <-h.ctx.Done():
			// Graceful shutdown: close all clients
			h.mu.Lock()
			for client := range h.clients {
				// Safely close channel (avoid double close panic)
				select {
				case _, stillOpen := <-client.Send:
					if stillOpen {
						close(client.Send)
					}
				default:
					close(client.Send)
				}
			}
			h.clients = make(map[*Client]bool)
			h.mu.Unlock()
			// Wait for all broadcast workers to finish
			h.broadcastWg.Wait()
			logger.Info("WebSocket hub stopped gracefully")
			return

		case client := <-h.register:
			if client == nil {
				continue
			}
			h.mu.Lock()
			h.clients[client] = true
			count := len(h.clients)
			h.mu.Unlock()
			logger.Debugf("WebSocket client connected: %s (total: %d)", client.ID, count)

		case client := <-h.unregister:
			if client == nil {
				continue
			}
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				// Safely close channel (avoid double close panic)
				// Check if channel is already closed by trying to read from it
				select {
				case _, stillOpen := <-client.Send:
					if stillOpen {
						// Channel was open and had data, now it's empty, safe to close
						close(client.Send)
					}
					// If stillOpen is false, channel was already closed, do nothing
				default:
					// Channel is empty and open, safe to close
					close(client.Send)
				}
			}
			count := len(h.clients)
			h.mu.Unlock()
			logger.Debugf("WebSocket client disconnected: %s (total: %d)", client.ID, count)

		case message := <-h.broadcast:
			if message == nil {
				continue
			}
			// Optimization: quickly copy client list and release lock
			h.mu.RLock()
			clientCount := len(h.clients)
			if clientCount == 0 {
				h.mu.RUnlock()
				continue
			}

			// Pre-allocate memory for client list
			clients := make([]*Client, 0, clientCount)
			for client := range h.clients {
				clients = append(clients, client)
			}
			h.mu.RUnlock()

			// Parallel broadcast using worker pool
			h.broadcastParallel(clients, message)
		}
	}
}

// broadcastParallel sends message to all clients in parallel for maximum performance
func (h *Hub) broadcastParallel(clients []*Client, message []byte) {
	if len(clients) == 0 {
		return
	}

	// For small number of clients, use simple parallel sending
	if len(clients) < h.workerPoolSize {
		var wg sync.WaitGroup
		for _, client := range clients {
			wg.Add(1)
			go func(c *Client) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						// Channel may be closed, safely ignore
						logger.Debugf("WebSocket broadcast panic recovered for client %s: %v", c.ID, r)
					}
				}()
				select {
				case c.Send <- message:
				default:
					// Client's send buffer is full, disconnect
					logger.Debugf("WebSocket client %s send buffer full, disconnecting", c.ID)
					h.Unregister(c)
				}
			}(client)
		}
		wg.Wait()
		return
	}

	// For large number of clients, use worker pool for optimal performance
	clientChan := make(chan *Client, len(clients))
	for _, client := range clients {
		clientChan <- client
	}
	close(clientChan)

	// Start workers for parallel processing
	h.broadcastWg.Add(h.workerPoolSize)
	for i := 0; i < h.workerPoolSize; i++ {
		go func() {
			defer h.broadcastWg.Done()
			for client := range clientChan {
				func() {
					defer func() {
						if r := recover(); r != nil {
							// Channel may be closed, safely ignore
							logger.Debugf("WebSocket broadcast panic recovered for client %s: %v", client.ID, r)
						}
					}()
					select {
					case client.Send <- message:
					default:
						// Client's send buffer is full, disconnect
						logger.Debugf("WebSocket client %s send buffer full, disconnecting", client.ID)
						h.Unregister(client)
					}
				}()
			}
		}()
	}

	// Wait for all workers to finish
	h.broadcastWg.Wait()
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(messageType MessageType, payload any) {
	if h == nil {
		return
	}
	if payload == nil {
		logger.Warning("Attempted to broadcast nil payload")
		return
	}

	msg := Message{
		Type:    messageType,
		Payload: payload,
		Time:    getCurrentTimestamp(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		logger.Error("Failed to marshal WebSocket message:", err)
		return
	}

	// Limit message size to prevent memory issues
	const maxMessageSize = 1024 * 1024 // 1MB
	if len(data) > maxMessageSize {
		logger.Warningf("WebSocket message too large: %d bytes, dropping", len(data))
		return
	}

	// Non-blocking send with timeout to prevent delays
	select {
	case h.broadcast <- data:
	case <-time.After(100 * time.Millisecond):
		logger.Warning("WebSocket broadcast channel is full, dropping message")
	case <-h.ctx.Done():
		// Hub is shutting down
	}
}

// BroadcastToTopic sends a message only to clients subscribed to the specific topic
func (h *Hub) BroadcastToTopic(messageType MessageType, payload any) {
	if h == nil {
		return
	}
	if payload == nil {
		logger.Warning("Attempted to broadcast nil payload to topic")
		return
	}

	msg := Message{
		Type:    messageType,
		Payload: payload,
		Time:    getCurrentTimestamp(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		logger.Error("Failed to marshal WebSocket message:", err)
		return
	}

	// Limit message size to prevent memory issues
	const maxMessageSize = 1024 * 1024 // 1MB
	if len(data) > maxMessageSize {
		logger.Warningf("WebSocket message too large: %d bytes, dropping", len(data))
		return
	}

	h.mu.RLock()
	// Filter clients by topics and quickly release lock
	subscribedClients := make([]*Client, 0)
	for client := range h.clients {
		if len(client.Topics) == 0 || client.Topics[messageType] {
			subscribedClients = append(subscribedClients, client)
		}
	}
	h.mu.RUnlock()

	// Parallel send to subscribed clients
	if len(subscribedClients) > 0 {
		h.broadcastParallel(subscribedClients, data)
	}
}

// GetClientCount returns the number of connected clients
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Register registers a new client with the hub
func (h *Hub) Register(client *Client) {
	if h == nil || client == nil {
		return
	}
	select {
	case h.register <- client:
	case <-h.ctx.Done():
		// Hub is shutting down
	}
}

// Unregister unregisters a client from the hub
func (h *Hub) Unregister(client *Client) {
	if h == nil || client == nil {
		return
	}
	select {
	case h.unregister <- client:
	case <-h.ctx.Done():
		// Hub is shutting down
	}
}

// Stop gracefully stops the hub and closes all connections
func (h *Hub) Stop() {
	if h == nil {
		return
	}
	if h.cancel != nil {
		h.cancel()
	}
}

// getCurrentTimestamp returns current Unix timestamp in milliseconds
func getCurrentTimestamp() int64 {
	return time.Now().UnixMilli()
}
