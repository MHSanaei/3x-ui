package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func vkTurnPortSettings(port int) string {
	return fmt.Sprintf(`{"vkTurn":{"enabled":true,"callLink":"https://vk.ru/call/join/test","port":%d}}`, port)
}

func TestVKTurnPortConflictsWithUDPListenersInBothDirections(t *testing.T) {
	for _, sidecarFirst := range []bool{false, true} {
		t.Run(fmt.Sprint(sidecarFirst), func(t *testing.T) {
			setupConflictDB(t)
			ordinary := &model.Inbound{Tag: "ordinary", Enable: true, Listen: "127.0.0.1", Port: 56000, Protocol: model.WireGuard, Settings: `{}`}
			tunnel := &model.Inbound{Tag: "vk-wg", Enable: true, Listen: "192.0.2.1", Port: 51820, Protocol: model.WireGuard, Settings: vkTurnPortSettings(56000)}
			existing, candidate := ordinary, tunnel
			if sidecarFirst {
				existing, candidate = tunnel, ordinary
			}
			if err := database.GetDB().Create(existing).Error; err != nil {
				t.Fatal(err)
			}
			hit, err := (&InboundService{}).checkPortConflict(candidate, 0)
			if err != nil {
				t.Fatal(err)
			}
			if hit == nil || hit.InboundID != existing.Id || hit.Port != 56000 || hit.Transports != transportUDP {
				t.Fatalf("conflict = %+v, want inbound %d UDP port 56000", hit, existing.Id)
			}
			if sidecarFirst && !strings.Contains(hit.String(), "VK TURN port 56000") {
				t.Fatalf("conflict does not name the VK TURN listener: %s", hit)
			}
		})
	}
}

func TestVKTurnPortReservationsRespectNodesTCPAndDisabledRows(t *testing.T) {
	for _, tc := range []struct {
		name         string
		protocol     model.Protocol
		nodeID       *int
		wantConflict bool
	}{
		{"same host UDP", model.WireGuard, nil, true},
		{"same host TCP", model.VLESS, nil, false},
		{"different host UDP", model.WireGuard, new(2), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupConflictDB(t)
			seedInboundConflict(t, "vk-wg", "0.0.0.0", 51820, model.WireGuard, "", vkTurnPortSettings(56000))
			if err := database.GetDB().Model(&model.Inbound{}).Where("tag = ?", "vk-wg").Update("enable", false).Error; err != nil {
				t.Fatal(err)
			}
			hit, err := (&InboundService{}).checkPortConflict(&model.Inbound{Port: 56000, Protocol: tc.protocol, NodeID: tc.nodeID}, 0)
			if err != nil {
				t.Fatal(err)
			}
			if (hit != nil) != tc.wantConflict {
				t.Fatalf("conflict = %+v, want conflict %v", hit, tc.wantConflict)
			}
		})
	}
}

func TestVKTurnPortConflictsWithAnotherVKTurnPort(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "vk-first", "0.0.0.0", 51820, model.WireGuard, "", vkTurnPortSettings(56000))
	hit, err := (&InboundService{}).checkPortConflict(&model.Inbound{Port: 51821, Protocol: model.WireGuard, Settings: vkTurnPortSettings(56000)}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if hit == nil || hit.Tag != "vk-first" || hit.Port != 56000 {
		t.Fatalf("conflict = %+v, want vk-first on 56000", hit)
	}
}

func TestVKTurnPortEditIgnoresOwnReservation(t *testing.T) {
	setupConflictDB(t)
	inbound := &model.Inbound{Tag: "vk-wg", Port: 51820, Protocol: model.WireGuard, Settings: vkTurnPortSettings(56000)}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatal(err)
	}
	hit, err := (&InboundService{}).checkPortConflict(inbound, inbound.Id)
	if err != nil || hit != nil {
		t.Fatalf("own reservation rejected: conflict %+v, error %v", hit, err)
	}
}

func TestVKTurnPortAndAmneziaWGForwardConflictInBothDirections(t *testing.T) {
	setupConflictDB(t)
	seedInboundConflict(t, "awg-forward", "0.0.0.0", 51821, model.AmneziaWG, "", awgRelayWindowSettingsWithForward(t, "awg-forward", "56000"))
	hit, err := (&InboundService{}).checkPortConflict(&model.Inbound{Port: 51820, Protocol: model.WireGuard, Settings: vkTurnPortSettings(56000)}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if hit == nil || hit.Port != 56000 || hit.ForwardedBy != "awg-forward@relay-window" {
		t.Fatalf("conflict = %+v, want forwarded port 56000", hit)
	}

	seedInboundConflict(t, "vk-wg", "0.0.0.0", 51822, model.WireGuard, "", vkTurnPortSettings(56001))
	ctx, err := (&InboundService{}).loadPortConflictContext(database.GetDB(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := (&InboundService{}).checkForwardedPortsConflict(ctx, "56001"); !strings.Contains(got, "VK TURN") || !strings.Contains(got, "56001") {
		t.Fatalf("forward conflict = %q, want VK TURN port 56001", got)
	}
}
