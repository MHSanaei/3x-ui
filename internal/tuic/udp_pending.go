package tuic

import (
	"context"
	"errors"
	"sync/atomic"
)

const (
	maxPendingUdpBytesPerInbound = 1 << 20
	maxPendingUdpBytesPerConn    = 128 << 10
	maxPendingUdpPackets         = 16
	udpPacketOverhead            = 256
)

var errUdpQueueFull = errors.New("tuic: pending UDP packet limit reached")

type pendingUdpBudget struct{ bytes atomic.Int64 }

func (b *pendingUdpBudget) reserve(size int) bool {
	if b == nil {
		return true
	}
	for {
		used := b.bytes.Load()
		if used+int64(size) > maxPendingUdpBytesPerInbound {
			return false
		}
		if b.bytes.CompareAndSwap(used, used+int64(size)) {
			return true
		}
	}
}

type pendingUdpPacket struct {
	target  *Address
	payload []byte
}

func (r *udpAssociationRegistry) queuePacket(ctx context.Context, id uint16, expected *udpAssociation, target *Address, payload []byte) (*udpRelaySession, context.Context, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.associations[id] != expected {
		return nil, nil, errUdpAssociationClosed
	}
	if expected.relay != nil && !expected.opening {
		return expected.relay, nil, nil
	}
	charge := len(payload) + udpPacketOverhead
	if len(expected.pending) >= maxPendingUdpPackets || r.pendingBytes+charge > maxPendingUdpBytesPerConn || !r.pendingBudget.reserve(charge) {
		return nil, nil, errUdpQueueFull
	}
	r.pendingBytes += charge
	addr := *target
	addr.IP = append([]byte(nil), target.IP...)
	expected.pending = append(expected.pending, pendingUdpPacket{target: &addr, payload: append([]byte(nil), payload...)})
	if expected.opening {
		return nil, nil, nil
	}
	expected.opening = true
	opening, cancel := context.WithCancel(ctx)
	expected.openCancel = cancel
	return nil, opening, nil
}

func (r *udpAssociationRegistry) nextPending(id uint16, expected *udpAssociation) (pendingUdpPacket, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.associations[id] != expected || len(expected.pending) == 0 {
		expected.opening = false
		if expected.openCancel != nil {
			expected.openCancel()
			expected.openCancel = nil
		}
		return pendingUdpPacket{}, false
	}
	packet := expected.pending[0]
	expected.pending[0] = pendingUdpPacket{}
	expected.pending = expected.pending[1:]
	return packet, true
}

func (r *udpAssociationRegistry) releasePendingPacketLocked(packet pendingUdpPacket) {
	charge := len(packet.payload) + udpPacketOverhead
	r.pendingBytes -= charge
	if r.pendingBudget != nil {
		r.pendingBudget.bytes.Add(-int64(charge))
	}
}

func (r *udpAssociationRegistry) releasePendingPacket(packet pendingUdpPacket) {
	r.mu.Lock()
	r.releasePendingPacketLocked(packet)
	r.mu.Unlock()
}

func (r *udpAssociationRegistry) clearPendingLocked(association *udpAssociation) {
	for _, packet := range association.pending {
		r.releasePendingPacketLocked(packet)
	}
	association.pending = nil
	association.opening = false
	if association.openCancel != nil {
		association.openCancel()
		association.openCancel = nil
	}
}

func (r *udpAssociationRegistry) failPending(expected *udpAssociation) {
	r.mu.Lock()
	r.clearPendingLocked(expected)
	r.mu.Unlock()
}
