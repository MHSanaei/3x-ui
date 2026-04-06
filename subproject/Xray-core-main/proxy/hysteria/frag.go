package hysteria

func FragUDPMessage(m *UDPMessage, maxSize int) []UDPMessage {
	if m.Size() <= maxSize {
		return []UDPMessage{*m}
	}
	fullPayload := m.Data
	maxPayloadSize := maxSize - m.HeaderSize()
	off := 0
	fragID := uint8(0)
	fragCount := uint8((len(fullPayload) + maxPayloadSize - 1) / maxPayloadSize) // round up
	frags := make([]UDPMessage, fragCount)
	for off < len(fullPayload) {
		payloadSize := len(fullPayload) - off
		if payloadSize > maxPayloadSize {
			payloadSize = maxPayloadSize
		}
		frag := *m
		frag.FragID = fragID
		frag.FragCount = fragCount
		frag.Data = fullPayload[off : off+payloadSize]
		frags[fragID] = frag
		off += payloadSize
		fragID++
	}
	return frags
}

// Defragger handles the defragmentation of UDP messages.
// The current implementation can only handle one packet ID at a time.
// If another packet arrives before a packet has received all fragments
// in their entirety, any previous state is discarded.
type Defragger struct {
	pktID uint16
	frags []*UDPMessage
	count uint8
	size  int // data size
}

func (d *Defragger) Feed(m *UDPMessage) *UDPMessage {
	if m.FragCount <= 1 {
		return m
	}
	if m.FragID >= m.FragCount {
		// wtf is this?
		return nil
	}
	if m.PacketID != d.pktID || m.FragCount != uint8(len(d.frags)) {
		// new message, clear previous state
		d.pktID = m.PacketID
		d.frags = make([]*UDPMessage, m.FragCount)
		d.frags[m.FragID] = m
		d.count = 1
		d.size = len(m.Data)
	} else if d.frags[m.FragID] == nil {
		d.frags[m.FragID] = m
		d.count++
		d.size += len(m.Data)
		if int(d.count) == len(d.frags) {
			// all fragments received, assemble
			data := make([]byte, d.size)
			off := 0
			for _, frag := range d.frags {
				off += copy(data[off:], frag.Data)
			}
			m.Data = data
			m.FragID = 0
			m.FragCount = 1
			return m
		}
	}
	return nil
}
