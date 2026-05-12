// Package websocket provides a WebSocket hub for real-time updates and notifications.
package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/logger"
)

// MessageType identifies the kind of WebSocket message.
type MessageType string

const (
	MessageTypeStatus       MessageType = "status"
	MessageTypeTraffic      MessageType = "traffic"
	MessageTypeInbounds     MessageType = "inbounds"
	MessageTypeOutbounds    MessageType = "outbounds"
	MessageTypeNodes        MessageType = "nodes"
	MessageTypeNotification MessageType = "notification"
	MessageTypeXrayState    MessageType = "xray_state"
	// MessageTypeClientStats carries absolute traffic counters for the clients
	// that had activity in the latest collection window. Frontend applies these
	// in-place — far smaller than re-broadcasting the full inbound list and
	// scales to 10k+ clients without falling back to REST.
	MessageTypeClientStats MessageType = "client_stats"
	MessageTypeInvalidate  MessageType = "invalidate" // Tells frontend to re-fetch via REST (last-resort).

	// maxMessageSize caps the WebSocket payload. Beyond this the hub sends a
	// lightweight invalidate signal and the frontend re-fetches via REST.
	// 10MB lets typical 2k–8k-client deployments push directly via WS (low
	// latency); larger installs fall back to invalidate.
	maxMessageSize = 10 * 1024 * 1024 // 10MB

	enqueueTimeout    = 100 * time.Millisecond
	clientSendQueue   = 512  // ~50s of buffering for a momentarily slow browser.
	hubBroadcastQueue = 2048 // Headroom for cron-storm + admin-mutation bursts.
	hubControlQueue   = 64   // Backlog for register/unregister bursts (page reloads, disconnect storms).

	// minBroadcastInterval throttles per-type broadcasts so cron storms or
	// rapid mutations cannot drown the hub. Bursts within the interval are
	// dropped (not coalesced); the next broadcast outside the window delivers
	// the latest state. Only message types in throttledMessageTypes are gated —
	// heartbeat and one-shot signals (status, notification, xray_state,
	// invalidate) bypass this so they are never delayed.
	minBroadcastInterval = 250 * time.Millisecond

	// hubRestartAttempts caps panic-recovery restarts. After this many
	// consecutive failures we stop trying and log; the panel keeps running
	// (frontend falls back to REST polling) and the operator can investigate.
	hubRestartAttempts = 3
)

// NewClient builds a Client ready for hub registration.
func NewClient(id string) *Client {
	return &Client{
		ID:   id,
		Send: make(chan []byte, clientSendQueue),
	}
}

// Message is the wire format sent to clients.
type Message struct {
	Type    MessageType `json:"type"`
	Payload any         `json:"payload"`
	Time    int64       `json:"time"`
}

// Client represents a single WebSocket connection.
type Client struct {
	ID        string
	Send      chan []byte
	closeOnce sync.Once
}

// Hub fan-outs messages to all connected clients.
type Hub struct {
	clients    map[*Client]struct{}
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc

	throttleMu    sync.Mutex
	lastBroadcast map[MessageType]time.Time
}

// NewHub creates a hub. Call Run in a goroutine to start its event loop.
func NewHub() *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		clients:       make(map[*Client]struct{}),
		broadcast:     make(chan []byte, hubBroadcastQueue),
		register:      make(chan *Client, hubControlQueue),
		unregister:    make(chan *Client, hubControlQueue),
		ctx:           ctx,
		cancel:        cancel,
		lastBroadcast: make(map[MessageType]time.Time),
	}
}

var throttledMessageTypes = map[MessageType]struct{}{
	MessageTypeInbounds:    {},
	MessageTypeOutbounds:   {},
	MessageTypeTraffic:     {},
	MessageTypeClientStats: {},
}

func (h *Hub) shouldThrottle(msgType MessageType) bool {
	if _, gated := throttledMessageTypes[msgType]; !gated {
		return false
	}
	h.throttleMu.Lock()
	defer h.throttleMu.Unlock()
	now := time.Now()
	if last, ok := h.lastBroadcast[msgType]; ok && now.Sub(last) < minBroadcastInterval {
		return true
	}
	h.lastBroadcast[msgType] = now
	return false
}

// Run drives the hub. The inner loop is wrapped in a panic-recovery harness
// that retries up to hubRestartAttempts times with backoff so a transient
// panic doesn't permanently kill real-time updates for commercial deployments.
// After the cap, the hub stays down and the frontend falls back to REST polling.
func (h *Hub) Run() {
	for attempt := 0; attempt < hubRestartAttempts; attempt++ {
		stopped := h.runOnce()
		if stopped {
			return
		}
		if attempt < hubRestartAttempts-1 {
			wait := time.Duration(1<<attempt) * time.Second // 1s, 2s, 4s
			logger.Errorf("WebSocket hub crashed, restarting in %s (%d/%d)", wait, attempt+1, hubRestartAttempts-1)
			select {
			case <-time.After(wait):
			case <-h.ctx.Done():
				return
			}
		}
	}
	logger.Error("WebSocket hub stopped after exhausting restart attempts")
}

// runOnce drives the event loop once and returns true if the hub stopped
// cleanly (context cancelled). On panic, recover logs and returns false so
// Run can decide whether to retry.
func (h *Hub) runOnce() (stopped bool) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("WebSocket hub panic recovered: %v", r)
			stopped = false
		}
	}()

	for {
		select {
		case <-h.ctx.Done():
			h.shutdown()
			return true

		case c := <-h.register:
			if c == nil {
				continue
			}
			h.mu.Lock()
			h.clients[c] = struct{}{}
			n := len(h.clients)
			h.mu.Unlock()
			logger.Debugf("WebSocket client connected: %s (total: %d)", c.ID, n)

		case c := <-h.unregister:
			if c == nil {
				continue
			}
			h.removeClient(c)

		case msg := <-h.broadcast:
			h.fanout(msg)
		}
	}
}

// shutdown closes all client send channels and clears the registry.
func (h *Hub) shutdown() {
	h.mu.Lock()
	for c := range h.clients {
		c.closeOnce.Do(func() { close(c.Send) })
	}
	h.clients = make(map[*Client]struct{})
	h.mu.Unlock()
	logger.Info("WebSocket hub stopped")
}

// removeClient deletes a client and closes its send channel exactly once.
func (h *Hub) removeClient(c *Client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		c.closeOnce.Do(func() { close(c.Send) })
	}
	n := len(h.clients)
	h.mu.Unlock()
	logger.Debugf("WebSocket client disconnected: %s (total: %d)", c.ID, n)
}

// fanout delivers msg to every client. Each send is non-blocking — a client
// whose buffer is full is collected for direct removal at the end. We do NOT
// route slow-client unregistrations through the unregister channel: under
// burst load (panel restart, network blip) that channel can fill up while the
// hub itself is the consumer, causing a self-deadlock.
func (h *Hub) fanout(msg []byte) {
	if msg == nil {
		return
	}
	h.mu.RLock()
	if len(h.clients) == 0 {
		h.mu.RUnlock()
		return
	}
	targets := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	var dead []*Client
	for _, c := range targets {
		if !trySend(c, msg) {
			dead = append(dead, c)
		}
	}

	if len(dead) == 0 {
		return
	}
	h.mu.Lock()
	for _, c := range dead {
		if _, ok := h.clients[c]; ok {
			delete(h.clients, c)
			c.closeOnce.Do(func() { close(c.Send) })
			logger.Debugf("WebSocket client %s send buffer full, disconnected", c.ID)
		}
	}
	h.mu.Unlock()
}

// trySend performs a non-blocking write to the client's Send channel.
// Returns false if the client should be evicted (full buffer or closed channel).
// A defer-recover guards against the rare race where the channel was closed
// concurrently — sending on a closed channel always panics, even with select+default.
func trySend(c *Client, msg []byte) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			ok = false
		}
	}()
	select {
	case c.Send <- msg:
		return true
	default:
		return false
	}
}

// Broadcast serializes payload and queues it for delivery to all clients.
// If the serialized message exceeds maxMessageSize, an invalidate signal is
// queued instead so the frontend re-fetches via REST. Broadcasts of throttled
// message types (see throttledMessageTypes) within minBroadcastInterval of
// the previous one are dropped — the next legitimate mutation will push the
// fresh state.
func (h *Hub) Broadcast(messageType MessageType, payload any) {
	if h == nil || payload == nil || h.GetClientCount() == 0 {
		return
	}
	if h.shouldThrottle(messageType) {
		return
	}
	data, err := json.Marshal(Message{
		Type:    messageType,
		Payload: payload,
		Time:    time.Now().UnixMilli(),
	})
	if err != nil {
		logger.Error("WebSocket marshal failed:", err)
		return
	}
	if len(data) > maxMessageSize {
		logger.Debugf("WebSocket payload %d bytes exceeds limit, sending invalidate for %s", len(data), messageType)
		h.broadcastInvalidate(messageType)
		return
	}
	h.enqueue(data)
}

// broadcastInvalidate queues a lightweight signal telling clients to re-fetch
// the named data type via REST.
func (h *Hub) broadcastInvalidate(originalType MessageType) {
	data, err := json.Marshal(Message{
		Type:    MessageTypeInvalidate,
		Payload: map[string]string{"type": string(originalType)},
		Time:    time.Now().UnixMilli(),
	})
	if err != nil {
		logger.Error("WebSocket invalidate marshal failed:", err)
		return
	}
	h.enqueue(data)
}

// enqueue submits raw bytes to the broadcast channel. Dropped on backpressure
// (channel full for >100ms) or shutdown.
func (h *Hub) enqueue(data []byte) {
	select {
	case h.broadcast <- data:
	case <-time.After(enqueueTimeout):
		logger.Warning("WebSocket broadcast channel full, dropping message")
	case <-h.ctx.Done():
	}
}

// GetClientCount returns the number of connected clients.
func (h *Hub) GetClientCount() int {
	if h == nil {
		return 0
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Register adds a client to the hub.
func (h *Hub) Register(c *Client) {
	if h == nil || c == nil {
		return
	}
	select {
	case h.register <- c:
	case <-h.ctx.Done():
	}
}

// Unregister removes a client from the hub. Fast path queues for the hub
// goroutine; if the channel is saturated (disconnect storm) we fall back
// to a direct removal under the write lock so dead clients aren't left in
// the registry waiting for their Send buffer to fill (minutes of wasted
// fanout work at low broadcast rates).
//
// Direct removal is safe from any caller: external goroutines (read/write
// pumps) hold no hub locks, and the hub goroutine itself never holds h.mu
// when it calls Unregister — fanout releases its RLock before per-client
// sends, so we can't self-deadlock here.
func (h *Hub) Unregister(c *Client) {
	if h == nil || c == nil {
		return
	}
	select {
	case h.unregister <- c:
	default:
		h.removeClient(c)
	}
}

// Stop signals the hub to shut down and close all client connections.
func (h *Hub) Stop() {
	if h != nil && h.cancel != nil {
		h.cancel()
	}
}
