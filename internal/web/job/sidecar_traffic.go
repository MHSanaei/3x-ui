package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/web/websocket"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// sidecarTrafficPayload builds the websocket broadcast map for one poll of a
// sidecar protocol's traffic delta. "Sidecar" means a protocol that never
// runs inside xray-core's own inbounds -- AmneziaWG's kernel tunnel and
// MTProto's mtg process are the only two today (both explicitly skipped from
// claiming an xray-core tag in GenXrayInboundConfig). XrayTrafficJob's own 5s
// broadcast (xray_traffic_job.go) carries "traffics"/"clientTraffics" for
// xray-native inbounds/clients and never mentions sidecar tags, so reusing
// those keys would let each side's broadcast clobber the other's speed on
// its very next, unrelated tick. protocol namespaces the keys instead:
// "amneziawg" produces "amneziawgTraffics"/"amneziawgClientTraffics",
// "mtproto" produces "mtprotoTraffics"/"mtprotoClientTraffics" -- distinct
// pairs per protocol, so the frontend tracks each independently and never
// has to reconcile two protocols within one map (see useInbounds.ts /
// useClients.ts).
func sidecarTrafficPayload(protocol string, traffics []*xray.Traffic, clientTraffics []*xray.ClientTraffic) map[string]any {
	return map[string]any{
		protocol + "Traffics":       traffics,
		protocol + "ClientTraffics": clientTraffics,
	}
}

// broadcastSidecarTraffic pushes this poll's live-speed snapshot for a
// sidecar protocol ("amneziawg" or "mtproto") over the websocket, so its
// inbound/client rows show a live Speed value the same way xray-native rows
// already do. Call this every tick -- even with empty slices -- so a peer
// that just went idle clears to no-speed on the frontend instead of sticking
// at its last nonzero reading; only the AddTraffic/RefreshLocalOnlineClients
// calls around it are conditioned on non-empty data, since cumulative-totals
// accounting doesn't need this signal. Uses BroadcastSidecarTraffic (not
// BroadcastTraffic) -- see its doc comment for why.
func broadcastSidecarTraffic(protocol string, traffics []*xray.Traffic, clientTraffics []*xray.ClientTraffic) {
	if !websocket.HasClients() {
		return
	}
	websocket.BroadcastSidecarTraffic(sidecarTrafficPayload(protocol, traffics, clientTraffics))
}
