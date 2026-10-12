package tuic

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	maxUdpAssociations        = 256
	udpAssociationIdleTimeout = 5 * time.Minute
)

var errUdpAssociationClosed = errors.New("tuic: UDP association is closed")

type udpAssociation struct {
	responseTransport uint8
	relay             *udpRelaySession
	lastActive        time.Time
}

type udpAssociationRegistry struct {
	mu                  sync.Mutex
	associations        map[uint16]*udpAssociation
	datagramReassembler *packetReassembler
	streamReassembler   *packetReassembler
	maxPacketSize       int
}

func newUdpAssociationRegistry(maxPacketSize int) *udpAssociationRegistry {
	return &udpAssociationRegistry{
		associations:        make(map[uint16]*udpAssociation),
		datagramReassembler: newPacketReassembler(maxPacketSize),
		streamReassembler:   newPacketReassembler(maxPacketSize),
		maxPacketSize:       maxPacketSize,
	}
}

func (r *udpAssociationRegistry) feed(transport uint8, hdr *PacketHeader, payload []byte) (*udpAssociation, *Address, []byte, bool) {
	if !validPacketFragment(hdr, payload, r.maxPacketSize) {
		return nil, nil, nil, false
	}

	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked(now)

	association := r.associations[hdr.AssocID]
	if association == nil {
		if len(r.associations) >= maxUdpAssociations {
			return nil, nil, nil, false
		}
		association = &udpAssociation{responseTransport: transport, lastActive: now}
		r.associations[hdr.AssocID] = association
	}
	association.lastActive = now

	reassembler := r.datagramReassembler
	if transport == packetTransportStream {
		reassembler = r.streamReassembler
	}
	addr, completePayload, complete := reassembler.feed(transport, hdr, payload)
	return association, addr, completePayload, complete
}

func validPacketFragment(hdr *PacketHeader, payload []byte, maxPacketSize int) bool {
	if hdr == nil || hdr.FragTotal == 0 || hdr.FragID >= hdr.FragTotal || int(hdr.Size) != len(payload) || len(payload) > maxPacketSize {
		return false
	}
	if hdr.FragTotal == 1 {
		return hdr.FragID == 0 && isPacketTarget(hdr.Addr)
	}
	return (hdr.FragID != 0 || isPacketTarget(hdr.Addr)) &&
		(hdr.FragID == 0 || hdr.Addr == nil || hdr.Addr.Type == AddrTypeNone)
}

func (r *udpAssociationRegistry) ensureRelay(ctx context.Context, assocID uint16, expected *udpAssociation, user *User, relay *SocksRelay) (*udpAssociation, bool, error) {
	r.mu.Lock()
	association := r.associations[assocID]
	if association == nil || association != expected {
		r.mu.Unlock()
		return nil, false, errUdpAssociationClosed
	}
	if association.relay != nil {
		association.lastActive = time.Now()
		r.mu.Unlock()
		return association, false, nil
	}
	r.mu.Unlock()

	socksSession, err := relay.DialUDP(ctx, user.Email)
	if err != nil {
		return nil, false, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	current := r.associations[assocID]
	if current != association {
		_ = socksSession.Close()
		return nil, false, errUdpAssociationClosed
	}
	if current.relay != nil {
		_ = socksSession.Close()
		current.lastActive = time.Now()
		return current, false, nil
	}
	current.relay = &udpRelaySession{relay: socksSession, responseTransport: current.responseTransport}
	current.lastActive = time.Now()
	return current, true, nil
}

func (r *udpAssociationRegistry) dissociate(assocID uint16) bool {
	r.mu.Lock()
	association := r.associations[assocID]
	delete(r.associations, assocID)
	r.datagramReassembler.clearAssociation(assocID)
	r.streamReassembler.clearAssociation(assocID)
	r.mu.Unlock()
	if association == nil {
		return false
	}
	if association.relay != nil {
		_ = association.relay.relay.Close()
	}
	return true
}

func (r *udpAssociationRegistry) touch(assocID uint16, expected *udpAssociation, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	current := r.associations[assocID]
	if current == nil || current != expected {
		return false
	}
	current.lastActive = now
	return true
}

func (r *udpAssociationRegistry) reapIdle(now time.Time) {
	r.mu.Lock()
	r.expireLocked(now)
	r.mu.Unlock()
}

func (r *udpAssociationRegistry) expireLocked(now time.Time) {
	for assocID, association := range r.associations {
		if now.Sub(association.lastActive) <= udpAssociationIdleTimeout {
			continue
		}
		delete(r.associations, assocID)
		r.datagramReassembler.clearAssociation(assocID)
		r.streamReassembler.clearAssociation(assocID)
		if association.relay != nil {
			_ = association.relay.relay.Close()
		}
	}
}

func (r *udpAssociationRegistry) closeAll() {
	r.mu.Lock()
	for _, association := range r.associations {
		if association.relay != nil {
			_ = association.relay.relay.Close()
		}
	}
	r.associations = make(map[uint16]*udpAssociation)
	r.datagramReassembler.clearAll()
	r.streamReassembler.clearAll()
	r.mu.Unlock()
}

func (r *udpAssociationRegistry) release(id uint16, expected *udpAssociation) {
	r.mu.Lock()
	if r.associations[id] == expected {
		delete(r.associations, id)
		r.datagramReassembler.clearAssociation(id)
		r.streamReassembler.clearAssociation(id)
	}
	r.mu.Unlock()
	if expected.relay != nil {
		_ = expected.relay.relay.Close()
	}
}
