// Package websocket provides WebSocket hub for real-time updates and notifications.
package websocket

import (
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/global"
)

// GetHub returns the global WebSocket hub instance
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

// BroadcastStatus broadcasts server status update to all connected clients
func BroadcastStatus(status any) {
	hub := GetHub()
	if hub != nil {
		hub.Broadcast(MessageTypeStatus, status)
	}
}

// BroadcastTraffic broadcasts traffic statistics update to all connected clients
func BroadcastTraffic(traffic any) {
	hub := GetHub()
	if hub != nil {
		hub.Broadcast(MessageTypeTraffic, traffic)
	}
}

// BroadcastInbounds broadcasts inbounds list update to all connected clients
func BroadcastInbounds(inbounds any) {
	hub := GetHub()
	if hub != nil {
		hub.Broadcast(MessageTypeInbounds, inbounds)
	}
}

// BroadcastOutbounds broadcasts outbounds list update to all connected clients
func BroadcastOutbounds(outbounds any) {
	hub := GetHub()
	if hub != nil {
		hub.Broadcast(MessageTypeOutbounds, outbounds)
	}
}

// BroadcastNotification broadcasts a system notification to all connected clients
func BroadcastNotification(title, message, level string) {
	hub := GetHub()
	if hub != nil {
		notification := map[string]string{
			"title":   title,
			"message": message,
			"level":   level, // info, warning, error, success
		}
		hub.Broadcast(MessageTypeNotification, notification)
	}
}

// BroadcastXrayState broadcasts Xray state change to all connected clients
func BroadcastXrayState(state string, errorMsg string) {
	hub := GetHub()
	if hub != nil {
		stateUpdate := map[string]string{
			"state":    state,
			"errorMsg": errorMsg,
		}
		hub.Broadcast(MessageTypeXrayState, stateUpdate)
	}
}
