package quiccapture

import (
	"errors"
	"fmt"
)

// TLS 1.3 extension type for key_share (RFC 8446 4.2.8), as it appears in a
// ServerHello's own extension list.
const extTypeKeyShare = 0x0033

// ErrNoCryptoFrameYet means this specific datagram's Initial packet held
// only ACK/PADDING/PING frames -- a real, observed-in-the-wild server
// behavior (cloudflare-quic.com splits its ACK and CRYPTO frames across two
// separate datagrams, unlike Google/Facebook/Cloudflare's main site, which
// coalesce them into one). Capture treats this as "read one more datagram
// and retry," not a hard failure.
var ErrNoCryptoFrameYet = errors.New("quiccapture: server's Initial packet had no CRYPTO frame yet (ACK/PADDING only)")

// ParseServerReply turns the server's raw UDP reply to a probe built by
// buildClientProbe into a CPS chain: the literal ("this genuinely happened")
// bytes stay a <b 0x...> block, and exactly the two spans a real client-
// server exchange would show fresh on every connection -- the ServerHello's
// Random field and its key_share ephemeral public key, plus the trailing
// AEAD tag -- become <r N> placeholders. Everything else (cipher suite
// chosen, extension order/lengths, the header-protection sample window)
// stays frozen, matching real traffic: a real server also repeats the same
// negotiated shape on every connection, only the crypto material varies.
//
// resp is the raw datagram as received (may contain a coalesced
// Handshake-level packet after this Initial packet; that trailing data is
// simply never reached, since Length says exactly where this packet ends
// and there's no key to decrypt anything past it anyway).
func ParseServerReply(resp []byte, probeDCID []byte) (string, error) {
	if len(resp) < 20 || resp[0]&0xf0 != 0xc0 {
		return "", fmt.Errorf("quiccapture: reply is not a QUICv1 long-header Initial packet")
	}
	version := uint32(resp[1])<<24 | uint32(resp[2])<<16 | uint32(resp[3])<<8 | uint32(resp[4])
	if version == 0 {
		return "", fmt.Errorf("quiccapture: server sent a Version Negotiation packet, not an Initial reply")
	}
	if version != quicVersion1 {
		return "", fmt.Errorf("quiccapture: server replied with QUIC version 0x%08x, not v1", version)
	}

	cursor := 5
	if cursor >= len(resp) {
		return "", fmt.Errorf("quiccapture: truncated reply (DCID length)")
	}
	dcidLen := int(resp[cursor])
	cursor++
	cursor += dcidLen // the server's DCID here is just an echo of our own SCID; not needed for decryption

	if cursor >= len(resp) {
		return "", fmt.Errorf("quiccapture: truncated reply (SCID length)")
	}
	scidLen := int(resp[cursor])
	cursor++
	cursor += scidLen

	if cursor >= len(resp) {
		return "", fmt.Errorf("quiccapture: truncated reply (token length)")
	}
	tokenLen, width, err := quicVarintDecode(resp[cursor:])
	if err != nil {
		return "", fmt.Errorf("quiccapture: token length: %w", err)
	}
	cursor += width + tokenLen

	if cursor >= len(resp) {
		return "", fmt.Errorf("quiccapture: truncated reply (packet length)")
	}
	pktLen, width, err := quicVarintDecode(resp[cursor:])
	if err != nil {
		return "", fmt.Errorf("quiccapture: packet length: %w", err)
	}
	cursor += width
	pnFieldOffset := cursor
	if pnFieldOffset+pktLen > len(resp) {
		return "", fmt.Errorf("quiccapture: reply's own Length field claims more bytes than were received")
	}

	// Header protection: RFC 9001 S5.4.2 always samples 16 bytes starting 4
	// bytes into the (still masked) Packet Number field, regardless of the
	// real PN length -- that's precisely so a receiver can find the sample
	// before knowing the real PN length.
	if pnFieldOffset+4+sampleLen > len(resp) {
		return "", fmt.Errorf("quiccapture: reply too short to contain a header-protection sample")
	}
	sample := resp[pnFieldOffset+4 : pnFieldOffset+4+sampleLen]

	initialSecret := deriveInitialSecret(probeDCID)
	serverKeys := deriveKeys(initialSecret, "server in")

	mask, err := hpMask(serverKeys.hp, sample)
	if err != nil {
		return "", fmt.Errorf("quiccapture: header-protection mask: %w", err)
	}

	unmasked := append([]byte(nil), resp[:pnFieldOffset+pktLen]...)
	unmasked[0] ^= mask[0] & 0x0f
	pnLen := int(unmasked[0]&0x03) + 1
	if pnFieldOffset+pnLen > len(unmasked) {
		return "", fmt.Errorf("quiccapture: packet number field runs past the declared packet length")
	}
	for i := 0; i < pnLen; i++ {
		unmasked[pnFieldOffset+i] ^= mask[1+i]
	}
	pkn := unmasked[pnFieldOffset : pnFieldOffset+pnLen]

	ciphertextStart := pnFieldOffset + pnLen
	ciphertextWithTag := unmasked[ciphertextStart : pnFieldOffset+pktLen]
	if len(ciphertextWithTag) < quicTagLen {
		return "", fmt.Errorf("quiccapture: reply's encrypted region is shorter than one AEAD tag")
	}
	aad := unmasked[:ciphertextStart]

	nonce := xorPacketNumberIntoIV(serverKeys.iv, pkn)
	plaintext, err := aesGCMOpen(serverKeys.key, nonce, aad, ciphertextWithTag)
	if err != nil {
		return "", fmt.Errorf("quiccapture: could not decrypt server reply (try again or a different host): %w", err)
	}

	frameDataOffset, helloLen, err := findCryptoFrame(plaintext)
	if err != nil {
		return "", err
	}
	serverHello := plaintext[frameDataOffset : frameDataOffset+helloLen]
	randOffsetInHello, keyShareOffsetInHello, keyShareLen, err := locateServerHelloRandomSpans(serverHello)
	if err != nil {
		return "", err
	}

	randOffset := ciphertextStart + frameDataOffset + randOffsetInHello
	keyShareOffset := ciphertextStart + frameDataOffset + keyShareOffsetInHello
	tagOffset := ciphertextStart + len(plaintext)

	// The header-protection sample window (RFC 9001 S5.4.2, same rule
	// buildClientProbe's own comment cites) must never fall inside a span
	// amneziawg-go will re-randomize at send time -- otherwise a receiver
	// that tries to unmask this packet as real QUIC finds inconsistent
	// reserved bits, a detectable tell. If Random would naturally start
	// before that window ends, push the literal prefix out and shrink
	// Random's own randomized length by the same amount (mirroring
	// i1Generators.ts's quicCutParts).
	literalEnd := randOffset
	if sampleWindowEnd := ciphertextStart + (sampleLen + 4 - pnLen); sampleWindowEnd > literalEnd {
		literalEnd = sampleWindowEnd
	}
	randLen := (randOffset + helloRandLen) - literalEnd

	c := &cpsChain{}
	c.bytes(unmasked[:literalEnd])
	c.random(randLen)
	c.bytes(unmasked[randOffset+helloRandLen : keyShareOffset])
	c.random(keyShareLen)
	c.bytes(unmasked[keyShareOffset+keyShareLen : tagOffset])
	c.random(quicTagLen)
	return c.String(), nil
}

// findCryptoFrame walks the AEAD-opened Initial payload's frame sequence
// (RFC 9000 12.4: a packet's payload is a *sequence* of frames, not
// necessarily just one) looking for the CRYPTO frame (RFC 9000 19.6)
// carrying the ServerHello. A live run against real Google/Cloudflare/
// Facebook endpoints showed every one of them leads with an ACK frame
// acknowledging the probe's own Initial packet before their CRYPTO frame,
// in the same packet -- expecting CRYPTO at offset 0 (true only for the
// simplest possible responder) missed all of them. PADDING/PING/ACK/ACK_ECN
// are the only frame types skipped; anything else (a different frame type,
// a nonzero CRYPTO offset suggesting fragmentation/reordering) is treated
// as an unsupported capture rather than guessed at -- the ServerHello
// (~100-300 bytes) never legitimately needs to span multiple Initial-level
// packets or arrive out of order in a single-round-trip probe like this.
func findCryptoFrame(plaintext []byte) (dataOffset, dataLen int, err error) {
	cursor := 0
	for cursor < len(plaintext) {
		switch plaintext[cursor] {
		case 0x00: // PADDING: zero-length, consume just this one byte
			cursor++
		case 0x01: // PING: zero-length
			cursor++
		case 0x02, 0x03: // ACK / ACK_ECN
			n, err := skipAckFrame(plaintext[cursor:])
			if err != nil {
				return 0, 0, fmt.Errorf("quiccapture: skipping server's ACK frame: %w", err)
			}
			cursor += n
		case 0x06: // CRYPTO -- what we came for
			c := cursor + 1
			offset, w, err := quicVarintDecode(plaintext[c:])
			if err != nil {
				return 0, 0, fmt.Errorf("quiccapture: CRYPTO frame offset: %w", err)
			}
			c += w
			if offset != 0 {
				return 0, 0, fmt.Errorf("quiccapture: server's ServerHello arrived fragmented across frames; try again")
			}
			length, w, err := quicVarintDecode(plaintext[c:])
			if err != nil {
				return 0, 0, fmt.Errorf("quiccapture: CRYPTO frame length: %w", err)
			}
			c += w
			if c+length > len(plaintext) {
				return 0, 0, fmt.Errorf("quiccapture: CRYPTO frame claims more bytes than the packet contains")
			}
			return c, length, nil
		default:
			return 0, 0, fmt.Errorf("quiccapture: unexpected frame type 0x%02x before any CRYPTO frame; try again", plaintext[cursor])
		}
	}
	return 0, 0, ErrNoCryptoFrameYet
}

// skipAckFrame returns the byte length of one ACK/ACK_ECN frame (RFC 9000
// 19.3) starting at data[0], without needing to interpret its actual
// acknowledged ranges -- findCryptoFrame only needs to know where it ends.
// Layout: type(1) + largest_acked(varint) + ack_delay(varint) +
// ack_range_count(varint) + first_ack_range(varint) + ack_range_count
// pairs of {gap(varint), ack_range_length(varint)}, plus (ACK_ECN only)
// three trailing ECN counts.
func skipAckFrame(data []byte) (int, error) {
	isECN := data[0] == 0x03
	cursor := 1

	readVarint := func(name string) (int, error) {
		v, w, err := quicVarintDecode(data[cursor:])
		if err != nil {
			return 0, fmt.Errorf("%s: %w", name, err)
		}
		cursor += w
		return v, nil
	}

	if _, err := readVarint("largest_acked"); err != nil {
		return 0, err
	}
	if _, err := readVarint("ack_delay"); err != nil {
		return 0, err
	}
	ackRangeCount, err := readVarint("ack_range_count")
	if err != nil {
		return 0, err
	}
	if _, err := readVarint("first_ack_range"); err != nil {
		return 0, err
	}
	for i := 0; i < ackRangeCount; i++ {
		if _, err := readVarint(fmt.Sprintf("range %d gap", i)); err != nil {
			return 0, err
		}
		if _, err := readVarint(fmt.Sprintf("range %d ack_range_length", i)); err != nil {
			return 0, err
		}
	}
	if isECN {
		for _, name := range []string{"ect0_count", "ect1_count", "ecn_ce_count"} {
			if _, err := readVarint(name); err != nil {
				return 0, err
			}
		}
	}
	return cursor, nil
}

// locateServerHelloRandomSpans parses just enough of a real ServerHello
// (RFC 8446 4.1.3) to find the two connection-fresh spans: the Random field
// (fixed offset, right after the 4-byte handshake header + 2-byte
// legacy_version) and the key_share extension's ephemeral public key
// (position varies -- extension order isn't guaranteed by spec, so the
// extension list is walked to find it).
func locateServerHelloRandomSpans(hello []byte) (randOffset, keyShareOffset, keyShareLen int, err error) {
	if len(hello) < tlsHandshakeHead+helloRandLen+1 || hello[0] != 0x02 {
		return 0, 0, 0, fmt.Errorf("quiccapture: CRYPTO frame doesn't start with a ServerHello")
	}
	randOffset = tlsHandshakeHead

	cursor := tlsHandshakeHead + helloRandLen
	sessIDLen := int(hello[cursor])
	cursor += 1 + sessIDLen // legacy_session_id_echo length + bytes

	cursor += 2 // cipher_suite
	cursor += 1 // legacy_compression_method

	if cursor+2 > len(hello) {
		return 0, 0, 0, fmt.Errorf("quiccapture: ServerHello truncated before its extensions length")
	}
	extListLen := int(hello[cursor])<<8 | int(hello[cursor+1])
	cursor += 2
	extListEnd := cursor + extListLen
	if extListEnd > len(hello) {
		return 0, 0, 0, fmt.Errorf("quiccapture: ServerHello's extensions length runs past the message")
	}

	for cursor < extListEnd {
		if cursor+4 > extListEnd {
			return 0, 0, 0, fmt.Errorf("quiccapture: ServerHello extension list truncated")
		}
		extType := int(hello[cursor])<<8 | int(hello[cursor+1])
		extLen := int(hello[cursor+2])<<8 | int(hello[cursor+3])
		extDataStart := cursor + 4
		if extDataStart+extLen > extListEnd {
			return 0, 0, 0, fmt.Errorf("quiccapture: ServerHello extension %d overruns the extension list", extType)
		}
		if extType == extTypeKeyShare {
			if extLen < 4 {
				return 0, 0, 0, fmt.Errorf("quiccapture: key_share extension too short to contain a key")
			}
			keyLen := int(hello[extDataStart+2])<<8 | int(hello[extDataStart+3])
			if 4+keyLen != extLen {
				return 0, 0, 0, fmt.Errorf("quiccapture: key_share key length doesn't match the extension's own length")
			}
			return randOffset, extDataStart + 4, keyLen, nil
		}
		cursor = extDataStart + extLen
	}
	return 0, 0, 0, fmt.Errorf("quiccapture: ServerHello had no key_share extension")
}
