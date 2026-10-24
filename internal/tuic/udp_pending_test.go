package tuic

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func pendingTestAssociation(t *testing.T, registry *udpAssociationRegistry, id uint16, size int) (*udpAssociation, *Address, []byte) {
	t.Helper()
	target := &Address{Type: AddrTypeIPv4, IP: net.IPv4(127, 0, 0, 1), Port: 53}
	payload := make([]byte, size)
	association, _, _, complete := registry.feed(packetTransportDatagram, &PacketHeader{
		AssocID: id, PktID: 1, FragTotal: 1, Size: uint16(size), Addr: target,
	}, payload)
	if association == nil || !complete {
		t.Fatal("valid UDP packet did not create association")
	}
	return association, target, payload
}

func TestPendingUdpPacketLimitsReleaseCapacityAfterDissociate(t *testing.T) {
	registry := newUdpAssociationRegistry(1500)
	registry.pendingBudget = &pendingUdpBudget{}
	association, target, payload := pendingTestAssociation(t, registry, 1, 1000)
	for range 16 {
		if _, _, err := registry.queuePacket(context.Background(), 1, association, target, payload); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := registry.queuePacket(context.Background(), 1, association, target, payload); !errors.Is(err, errUdpQueueFull) {
		t.Fatalf("association overflow error = %v, want %v", err, errUdpQueueFull)
	}
	registry.dissociate(1)
	if used := registry.pendingBudget.bytes.Load(); used != 0 {
		t.Fatalf("dissociation retained %d queued bytes", used)
	}
	replacement, _, _ := pendingTestAssociation(t, registry, 1, 1000)
	if _, _, err := registry.queuePacket(context.Background(), 1, association, target, payload); !errors.Is(err, errUdpAssociationClosed) {
		t.Fatalf("stale worker queued into reused association: %v", err)
	}
	if _, opening, err := registry.queuePacket(context.Background(), 1, replacement, target, payload); err != nil || opening == nil {
		t.Fatalf("released capacity unavailable to replacement: opening=%v err=%v", opening, err)
	}
	registry.closeAll()
}

func TestPendingUdpByteBudgetsBoundMultipleConnections(t *testing.T) {
	budget := &pendingUdpBudget{}
	var registries []*udpAssociationRegistry
	defer func() {
		for _, registry := range registries {
			registry.closeAll()
		}
	}()
	blockedInbound := false
	for connID := 0; connID < 12; connID++ {
		registry := newUdpAssociationRegistry(1500)
		registry.pendingBudget = budget
		registries = append(registries, registry)
		blockedConn := false
		for id := uint16(1); id < 256; id++ {
			association, target, payload := pendingTestAssociation(t, registry, id, 1500)
			_, _, err := registry.queuePacket(context.Background(), id, association, target, payload)
			if err == nil {
				continue
			}
			if !errors.Is(err, errUdpQueueFull) {
				t.Fatal(err)
			}
			if connID == 0 && registry.pendingBytes+1756 <= 128<<10 {
				t.Fatal("first connection hit a limit before filling its 128 KiB allowance")
			}
			blockedConn = registry.pendingBytes+1756 > 128<<10
			blockedInbound = budget.bytes.Load()+1756 > 1<<20
			break
		}
		if registry.pendingBytes > 128<<10 || budget.bytes.Load() > 1<<20 {
			t.Fatal("pending UDP payloads exceeded their memory budgets")
		}
		if blockedInbound {
			break
		}
		if !blockedConn {
			t.Fatal("connection queue admitted all associations without enforcing byte budget")
		}
	}
	if !blockedInbound {
		t.Fatal("separate QUIC connections bypassed the shared inbound byte budget")
	}
	for _, registry := range registries {
		registry.closeAll()
	}
	if used := budget.bytes.Load(); used != 0 {
		t.Fatalf("closing connections retained %d queued bytes", used)
	}
}

func TestPendingUdpRemovalCancelsDialAndReleasesQueuedAndInFlightPackets(t *testing.T) {
	for _, remove := range []string{"dissociate", "expire", "shutdown", "reader-failure", "dial-failure"} {
		t.Run(remove, func(t *testing.T) {
			registry := newUdpAssociationRegistry(1500)
			registry.pendingBudget = &pendingUdpBudget{}
			association, target, payload := pendingTestAssociation(t, registry, 1, 100)
			_, opening, err := registry.queuePacket(context.Background(), 1, association, target, payload)
			if err != nil || opening == nil {
				t.Fatalf("queue first UDP packet: opening=%v err=%v", opening, err)
			}
			if _, _, err := registry.queuePacket(context.Background(), 1, association, target, payload); err != nil {
				t.Fatal(err)
			}
			inFlight, ok := registry.nextPending(1, association)
			if !ok {
				t.Fatal("first pending packet disappeared before removal")
			}
			switch remove {
			case "dissociate":
				registry.dissociate(1)
			case "expire":
				registry.reapIdle(time.Now().Add(udpAssociationIdleTimeout + time.Second))
			case "shutdown":
				registry.closeAll()
			case "reader-failure":
				registry.release(1, association)
			case "dial-failure":
				registry.failPending(association)
			}
			if !errors.Is(opening.Err(), context.Canceled) {
				t.Fatal("association removal left its SOCKS dial alive")
			}
			registry.releasePendingPacket(inFlight)
			if used := registry.pendingBudget.bytes.Load(); used != 0 || registry.pendingBytes != 0 {
				t.Fatalf("removal leaked or double released packet budget: inbound=%d connection=%d", used, registry.pendingBytes)
			}
		})
	}
}
