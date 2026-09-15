package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// checkForwardedPortsConflict only ever ran from the AmneziaWG save path, so an
// ordinary inbound could take a port a peer forwards -- a bind on every
// interface from the AmneziaWG side, which leaves one of the two listeners dead.
func TestAddInboundRefusesAPortAnAmneziaWGPeerForwards(t *testing.T) {
	const forwarded = 8443
	cases := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{"the forwarded port", forwarded, true},
		{"a free port", forwarded + 1, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupConflictDB(t)
			seedInboundConflict(t, "awg-forward", "0.0.0.0", 51820, model.AmneziaWG, ``,
				awgRelayWindowSettingsWithForward(t, "awg-forward", "8443"))

			_, _, err := (&InboundService{}).AddInbound(&model.Inbound{
				Tag: "user-inbound", Enable: true, Listen: "0.0.0.0", Port: tc.port,
				Protocol: model.VLESS, StreamSettings: `{"network":"tcp"}`, Settings: `{"clients":[]}`,
			})
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("port %d is free; the create must be allowed: %v", tc.port, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("port %d is forwarded by a peer of another inbound; the create must be refused", tc.port)
			}
			if !strings.Contains(err.Error(), "awg-forward@relay-window") {
				t.Fatalf("the refusal must name the peer holding the port, got %v", err)
			}
		})
	}
}

// The forward listener runs where the AmneziaWG row runs, so a node row sharing
// a local peer's port stays legal -- the scoping every other guard here uses.
func TestAddInboundAllowsANodeRowOnALocallyForwardedPort(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "awg-forward", "0.0.0.0", 51820, model.AmneziaWG, ``,
		awgRelayWindowSettingsWithForward(t, "awg-forward", "8443"))

	node := &model.Node{Name: "n1", Address: "127.0.0.1", Port: 2096, Scheme: "https", Enable: true, Status: "online"}
	if err := database.GetDB().Create(node).Error; err != nil {
		t.Fatalf("seed node: %v", err)
	}
	if _, _, err := (&InboundService{}).AddInbound(&model.Inbound{
		Tag: "node-inbound", Enable: true, Listen: "0.0.0.0", Port: 8443,
		Protocol: model.VLESS, StreamSettings: `{"network":"tcp"}`, Settings: `{"clients":[]}`,
		NodeID: &node.Id,
	}); err != nil {
		t.Fatalf("a node row does not bind here; the create must be allowed: %v", err)
	}
}
