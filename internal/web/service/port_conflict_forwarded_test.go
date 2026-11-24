package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
)

// checkForwardedPortsConflict only ran from the AmneziaWG save path, so an
// ordinary inbound could take a port a peer forwards on every interface.
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

// A peer the forward supervisor opens no listener for holds no port: it has no
// email, or no address the tunnel can route to, and Reconcile skips it either way.
func TestAddInboundAllowsAPortNoPeerCanActuallyForward(t *testing.T) {
	cases := []struct {
		name     string
		settings func(t *testing.T) string
	}{
		{
			name: "a peer with no email",
			settings: func(t *testing.T) string {
				t.Helper()
				return replaceFirst(t, awgRelayWindowSettingsWithForward(t, "awg-forward", "8443"),
					`"email":"awg-forward@relay-window"`, `"email":""`)
			},
		},
		{
			name: "an IPv6-only peer on a row without IPv6",
			settings: func(t *testing.T) string {
				t.Helper()
				return replaceFirst(t, awgRelayWindowSettingsWithForward(t, "awg-forward", "8443"),
					`"allowedIPs":["10.8.1.2/32"]`, `"allowedIPs":["fd00::2/128"]`)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupConflictDB(t)
			seedInboundConflict(t, "awg-forward", "0.0.0.0", 51820, model.AmneziaWG, ``, tc.settings(t))

			if _, _, err := (&InboundService{}).AddInbound(&model.Inbound{
				Tag: "user-inbound", Enable: true, Listen: "0.0.0.0", Port: 8443,
				Protocol: model.VLESS, StreamSettings: `{"network":"tcp"}`, Settings: `{"clients":[]}`,
			}); err != nil {
				t.Fatalf("nothing binds 8443 for this peer; the create must be allowed: %v", err)
			}
		})
	}
}

// The refusal has to point at where the socket really is: the forward listens on
// every interface, so repeating the candidate's requested address asserts a lie.
func TestForwardedPortRefusalNamesTheWildcardBind(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "awg-forward", "0.0.0.0", 51820, model.AmneziaWG, ``,
		awgRelayWindowSettingsWithForward(t, "awg-forward", "8443"))

	_, _, err := (&InboundService{}).AddInbound(&model.Inbound{
		Tag: "user-inbound", Enable: true, Listen: "10.0.0.5", Port: 8443,
		Protocol: model.VLESS, StreamSettings: `{"network":"tcp"}`, Settings: `{"clients":[]}`,
	})
	if err == nil {
		t.Fatal("the port is forwarded on every interface, including 10.0.0.5; the create must be refused")
	}
	if !strings.Contains(err.Error(), " on * by its client ") {
		t.Fatalf("the refusal must place the forward on every interface, got %v", err)
	}
}

func replaceFirst(t *testing.T, s, old, new string) string {
	t.Helper()
	if !strings.Contains(s, old) {
		t.Fatalf("fixture no longer contains %s", old)
	}
	return strings.Replace(s, old, new, 1)
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

// A forward bound on every interface holds a relay port no port column shows, so the
// inbound whose id derives that relay port must not be created onto it.
func TestAddInboundRefusesARelayPortAnAmneziaWGPeerForwards(t *testing.T) {
	cases := []struct {
		name      string
		relayPort func(id int) int
		inbound   func(t *testing.T) *model.Inbound
	}{
		{"amneziawg", amneziawgnet.SOCKSPortForInbound, func(t *testing.T) *model.Inbound {
			return &model.Inbound{
				Tag: "awg-next", Enable: true, Listen: "0.0.0.0", Port: 51821,
				Protocol: model.AmneziaWG, Settings: awgRelayWindowSettings(t, "awg-next"),
			}
		}},
		{"tuic", tuic.SOCKSPortForInbound, func(t *testing.T) *model.Inbound {
			return &model.Inbound{
				Tag: "tuic-next", Enable: true, Listen: "0.0.0.0", Port: 8443,
				Protocol: model.TUIC, Settings: `{"clients":[]}`,
			}
		}},
	}

	for _, tc := range cases {
		for _, held := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s held=%v", tc.name, held), func(t *testing.T) {
				setupConflictDB(t)
				// The holder takes id 1, so the create below derives its port from id 2.
				forwarded := tc.relayPort(2)
				if !held {
					forwarded = tc.relayPort(3)
				}
				seedInboundConflict(t, "awg-forward", "0.0.0.0", 51820, model.AmneziaWG, ``,
					awgRelayWindowSettingsWithForward(t, "awg-forward", fmt.Sprint(forwarded)))

				_, _, err := (&InboundService{}).AddInbound(tc.inbound(t))
				if !held {
					if err != nil {
						t.Fatalf("relay port %d is free; the create must be allowed: %v", tc.relayPort(2), err)
					}
					return
				}
				if err == nil {
					t.Fatalf("relay port %d is forwarded by a peer of another inbound; the create must be refused", forwarded)
				}
				if !strings.Contains(err.Error(), "relay port") || !strings.Contains(err.Error(), "awg-forward@relay-window") {
					t.Fatalf("the refusal must name the relay port and the peer holding it, got %v", err)
				}
			})
		}
	}
}
