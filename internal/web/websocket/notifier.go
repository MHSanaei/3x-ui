// Package websocket provides WebSocket hub for real-time updates and notifications.
package websocket

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/global"
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

// BroadcastSidecarTraffic broadcasts an AmneziaWG/MTProto traffic delta under
// the same "traffic" message type BroadcastTraffic uses, but bypasses the
// hub's per-type throttle (see Hub.BroadcastUnthrottled) so the two sidecar
// jobs' independent ~10s broadcasts can never starve each other. The payload
// carries protocol-namespaced keys (see internal/web/job/sidecar_traffic.go),
// so no new frontend message-type wiring is needed -- applyTrafficEvent
// already receives every "traffic" message.
func BroadcastSidecarTraffic(traffic any) {
	if hub := GetHub(); hub != nil {
		hub.BroadcastUnthrottled(MessageTypeTraffic, traffic)
	}
}

// BroadcastClientStats broadcasts absolute per-client traffic counters. Small
// installs send the complete row set each cycle (payload key snapshot=true);
// above the traffic job's snapshot threshold only the rows active in the
// latest collection window are sent (snapshot=false), which keeps the payload
// under the hub's cap at any client count.
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
