// Package websocket provides a WebSocket hub for real-time updates and notifications.
package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
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
	MessageTypeClientStats  MessageType = "client_stats"
	MessageTypeClients      MessageType = "clients"
	MessageTypeInvalidate   MessageType = "invalidate"
	maxMessageSize                      = 10 * 1024 * 1024 // 10MB

	enqueueTimeout       = 100 * time.Millisecond
	clientSendQueue      = 512  // ~50s of buffering for a momentarily slow browser.
	hubBroadcastQueue    = 2048 // Headroom for cron-storm + admin-mutation bursts.
	hubOpsQueue          = 128  // Backlog for register+unregister bursts (page reloads, disconnect storms).
	minBroadcastInterval = 250 * time.Millisecond
	hubRestartAttempts   = 3
)

type clientOpKind int

const (
	opRegister clientOpKind = iota
	opUnregister
)

type clientOp struct {
	kind clientOpKind
	c    *Client
}

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
	clients   map[*Client]struct{}
	broadcast chan []byte
	ops       chan clientOp
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc

	throttleMu    sync.Mutex
	lastBroadcast map[MessageType]time.Time
}

// NewHub creates a hub. Call Run in a goroutine to start its event loop.
func NewHub() *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		clients:       make(map[*Client]struct{}),
		broadcast:     make(chan []byte, hubBroadcastQueue),
		ops:           make(chan clientOp, hubOpsQueue),
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
	for attempt := range hubRestartAttempts {
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

		case op := <-h.ops:
			if op.c == nil {
				continue
			}
			switch op.kind {
			case opRegister:
				h.mu.Lock()
				h.clients[op.c] = struct{}{}
				n := len(h.clients)
				h.mu.Unlock()
				logger.Debugf("WebSocket client connected: %s (total: %d)", op.c.ID, n)
			case opUnregister:
				h.removeClient(op.c)
			}

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
	case h.ops <- clientOp{kind: opRegister, c: c}:
	case <-h.ctx.Done():
	}
}

// Unregister removes a client from the hub. Sends through the same ordered
// ops channel as Register so a register-then-unregister sequence from one
// goroutine is processed in program order — otherwise an unregister could
// land in the map before its register and silently no-op, leaking the entry.
//
// On a saturated ops channel (disconnect storm) we fall back to a bounded
// timeout drop rather than direct removal: a direct delete on a not-yet-
// registered client is precisely the ordering bug we fix here. Stragglers
// get evicted by fanout when their Send buffer fills.
func (h *Hub) Unregister(c *Client) {
	if h == nil || c == nil {
		return
	}
	select {
	case h.ops <- clientOp{kind: opUnregister, c: c}:
	case <-time.After(enqueueTimeout):
		logger.Warningf("WebSocket ops channel full, dropping unregister for %s", c.ID)
	case <-h.ctx.Done():
	}
}

// Stop signals the hub to shut down and close all client connections.
func (h *Hub) Stop() {
	if h != nil && h.cancel != nil {
		h.cancel()
	}
}
