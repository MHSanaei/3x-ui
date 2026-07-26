package job

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestSidecarTrafficPayload_UsesProtocolNamespacedKeys(t *testing.T) {
	traffics := []*xray.Traffic{{IsInbound: true, Tag: "amneziawg-1", Up: 10, Down: 20}}
	clientTraffics := []*xray.ClientTraffic{{Email: "a@b.c", Up: 10, Down: 20}}

	payload := sidecarTrafficPayload("amneziawg", traffics, clientTraffics)

	if _, ok := payload["amneziawgTraffics"]; !ok {
		t.Fatal("expected amneziawgTraffics key")
	}
	if _, ok := payload["amneziawgClientTraffics"]; !ok {
		t.Fatal("expected amneziawgClientTraffics key")
	}
	for _, wrongKey := range []string{"traffics", "clientTraffics", "mtprotoTraffics", "mtprotoClientTraffics"} {
		if _, ok := payload[wrongKey]; ok {
			t.Fatalf("payload must not contain %q -- it would collide with a different broadcast source", wrongKey)
		}
	}
}

func TestSidecarTrafficPayload_DistinctProtocolsNamespacedIndependently(t *testing.T) {
	amneziawgPayload := sidecarTrafficPayload("amneziawg", nil, nil)
	mtprotoPayload := sidecarTrafficPayload("mtproto", nil, nil)

	if _, ok := amneziawgPayload["mtprotoTraffics"]; ok {
		t.Fatal("amneziawg payload must not contain mtproto keys")
	}
	if _, ok := mtprotoPayload["amneziawgTraffics"]; ok {
		t.Fatal("mtproto payload must not contain amneziawg keys")
	}
}

func TestSidecarTrafficPayload_EmptyInputsStillProduceBothKeys(t *testing.T) {
	// The frontend clears a peer's speed when its tag/email is absent from
	// the payload's arrays -- but only if the key itself is present. If the
	// key vanished entirely for an idle poll, idle-clearing would never
	// trigger and the last nonzero speed would stick forever.
	payload := sidecarTrafficPayload("amneziawg", nil, nil)

	if _, ok := payload["amneziawgTraffics"]; !ok {
		t.Fatal("expected amneziawgTraffics key present even for nil input")
	}
	if _, ok := payload["amneziawgClientTraffics"]; !ok {
		t.Fatal("expected amneziawgClientTraffics key present even for nil input")
	}
}

func TestBroadcastSidecarTraffic_NoOpWithoutHub(t *testing.T) {
	// No global web server/hub is configured in this test binary, so
	// websocket.HasClients() is false -- this must return without panicking.
	broadcastSidecarTraffic("amneziawg", nil, nil)
}
