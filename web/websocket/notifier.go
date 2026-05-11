// Package websocket provides WebSocket hub for real-time updates and notifications.
package websocket

import (
	"github.com/mhsanaei/3x-ui/v3/logger"
	"github.com/mhsanaei/3x-ui/v3/web/global"
)

// GetHub returns the global WebSocket hub instance.
func GetHub() *Hub {
	webServer := global.GetWebServer()
	if webServer == nil {
		return nil
	}
	hub := webServer.GetWSHub()
	if hub == nil {
		return nil
	}
	wsHub, ok := hub.(*Hub)
	if !ok {
		logger.Warning("WebSocket hub type assertion failed")
		return nil
	}
	return wsHub
}

// HasClients returns true if any WebSocket client is connected.
// Use this to skip expensive work (DB queries, serialization) when no browser is open.
func HasClients() bool {
	hub := GetHub()
	return hub != nil && hub.GetClientCount() > 0
}

// BroadcastStatus broadcasts server status update to all connected clients.
func BroadcastStatus(status any) {
	if hub := GetHub(); hub != nil {
		hub.Broadcast(MessageTypeStatus, status)
	}
}

// BroadcastTraffic broadcasts traffic statistics update to all connected clients.
func BroadcastTraffic(traffic any) {
	if hub := GetHub(); hub != nil {
		hub.Broadcast(MessageTypeTraffic, traffic)
	}
}

// BroadcastClientStats broadcasts absolute per-client traffic counters for the
// clients that had activity in the latest collection window. Use this instead
// of re-broadcasting the full inbound list — it scales to 10k+ clients because
// the payload only includes active rows (typically a fraction of total).
func BroadcastClientStats(stats any) {
	if hub := GetHub(); hub != nil {
		hub.Broadcast(MessageTypeClientStats, stats)
	}
}

// BroadcastInbounds broadcasts inbounds list update to all connected clients.
func BroadcastInbounds(inbounds any) {
	if hub := GetHub(); hub != nil {
		hub.Broadcast(MessageTypeInbounds, inbounds)
	}
}

// BroadcastNodes broadcasts the fresh node list to all connected clients.
// Pushed by NodeHeartbeatJob at the end of each 10s tick so the Nodes page
// reflects status / latency / cpu / mem updates without polling.
func BroadcastNodes(nodes any) {
	if hub := GetHub(); hub != nil {
		hub.Broadcast(MessageTypeNodes, nodes)
	}
}

// BroadcastOutbounds broadcasts outbounds list update to all connected clients.
func BroadcastOutbounds(outbounds any) {
	if hub := GetHub(); hub != nil {
		hub.Broadcast(MessageTypeOutbounds, outbounds)
	}
}

// BroadcastNotification broadcasts a system notification to all connected clients.
func BroadcastNotification(title, message, level string) {
	hub := GetHub()
	if hub == nil {
		return
	}
	hub.Broadcast(MessageTypeNotification, map[string]string{
		"title":   title,
		"message": message,
		"level":   level,
	})
}

// BroadcastXrayState broadcasts Xray state change to all connected clients.
func BroadcastXrayState(state string, errorMsg string) {
	hub := GetHub()
	if hub == nil {
		return
	}
	hub.Broadcast(MessageTypeXrayState, map[string]string{
		"state":    state,
		"errorMsg": errorMsg,
	})
}

// BroadcastInvalidate sends a lightweight signal telling clients to re-fetch
// the named data type via REST. Use this when the caller already knows the
// payload is too large to push directly (e.g., 10k+ clients) to skip the
// JSON-marshal cost on the hot path.
func BroadcastInvalidate(dataType MessageType) {
	if hub := GetHub(); hub != nil {
		hub.broadcastInvalidate(dataType)
	}
}
