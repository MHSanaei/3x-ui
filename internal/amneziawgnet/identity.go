package amneziawgnet

import (
	"net/netip"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
)

// PeerIndex resolves a decapsulated connection's tunnel-internal source
// address back to the peer it belongs to, the same role Xray-core's own
// wireguard proxy's GetUserByAddr plays -- sourced here from an
// amneziawg.Instance's own Peers (already carries Email per peer, no new
// data needed) rather than a separate user table.
type PeerIndex struct {
	entries []peerIndexEntry
}

type peerIndexEntry struct {
	prefix netip.Prefix
	peer   amneziawg.Peer
}

// NewPeerIndex builds a lookup index from peers' AllowedIPs. Entries with an
// unparseable AllowedIPs value are skipped rather than failing the whole
// index -- by the time an Instance reaches this package, AllowedIPs has
// already been accepted at save time (see internal/amneziawg's own
// validation), so a bad entry here would only mean stale/manually-edited
// data, not something worth refusing to serve the rest of the peers over.
func NewPeerIndex(peers []amneziawg.Peer) *PeerIndex {
	idx := &PeerIndex{}
	for _, p := range peers {
		for _, a := range p.AllowedIPs {
			prefix, err := netip.ParsePrefix(a)
			if err != nil {
				continue
			}
			idx.entries = append(idx.entries, peerIndexEntry{prefix: prefix, peer: p})
		}
	}
	return idx
}

// Lookup returns the peer whose AllowedIPs most specifically contains addr --
// the same longest-prefix-match rule a real AmneziaWG interface's own
// AllowedIPs routing table uses for outbound packets, applied here in
// reverse to attribute an inbound (tunnel-internal-source) packet back to
// its owning peer.
func (idx *PeerIndex) Lookup(addr netip.Addr) (amneziawg.Peer, bool) {
	bestBits := -1
	var bestPeer amneziawg.Peer
	for _, e := range idx.entries {
		if e.prefix.Bits() <= bestBits || !e.prefix.Contains(addr) {
			continue
		}
		bestBits = e.prefix.Bits()
		bestPeer = e.peer
	}
	if bestBits < 0 {
		return amneziawg.Peer{}, false
	}
	return bestPeer, true
}
