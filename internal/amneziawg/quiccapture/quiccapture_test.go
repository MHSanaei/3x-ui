package quiccapture

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func fill(n int, b byte) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

// buildTestServerInitial builds a real, correctly-encrypted server Initial
// packet (using the package's own "server in"-derived keys, the same
// primitives buildClientProbe uses for the client side) carrying a
// hand-crafted but wire-accurate ServerHello -- so ParseServerReply is
// exercised against genuinely valid QUIC bytes, not a hand-waved fixture.
// randomFill/keyShareFill let the caller vary just the two connection-fresh
// fields while keeping everything else (cipher suite, extension shape)
// identical -- see TestParseServerReplyRandomizesOnlyTheRealRandomSpans.
func buildTestServerInitial(t *testing.T, dcid, serverSCID []byte, randomFill, keyShareFill byte) []byte {
	t.Helper()

	randomField := fill(helloRandLen, randomFill)
	keyShareField := fill(32, keyShareFill) // x25519 public key length

	cipherSuite := []byte{0x13, 0x01} // TLS_AES_128_GCM_SHA256

	supportedVersions := []byte{0x00, 0x2b, 0x00, 0x02, 0x03, 0x04} // type, len=2, TLS 1.3
	keyShareEntry := append([]byte{0x00, 0x1d}, byte(len(keyShareField)>>8), byte(len(keyShareField)))
	keyShareEntry = append(keyShareEntry, keyShareField...)
	keyShareExt := append([]byte{0x00, 0x33}, byte(len(keyShareEntry)>>8), byte(len(keyShareEntry)))
	keyShareExt = append(keyShareExt, keyShareEntry...)

	exts := append(append([]byte{}, supportedVersions...), keyShareExt...)

	body := []byte{0x03, 0x03} // legacy_version
	body = append(body, randomField...)
	body = append(body, 0) // legacy_session_id_echo length = 0
	body = append(body, cipherSuite...)
	body = append(body, 0) // legacy_compression_method
	body = append(body, byte(len(exts)>>8), byte(len(exts)))
	body = append(body, exts...)

	serverHello := append([]byte{0x02, byte(len(body) >> 16), byte(len(body) >> 8), byte(len(body))}, body...)
	payload := cryptoFrame(serverHello)
	// Pad like a real server's first flight would (well under 1200 bytes is
	// fine here -- ParseServerReply never enforces a minimum on replies).
	payload = append(payload, fill(50, 0)...)

	pkn := []byte{0, 1} // exercise a 2-byte server PN, deliberately different from the client's 1-byte PN
	header := []byte{0xc0 | byte(len(pkn)-1)}
	header = append(header, 0, 0, 0, quicVersion1)
	header = append(header, str8(dcid)...)       // server's DCID = echo of our SCID; irrelevant to decryption
	header = append(header, str8(serverSCID)...) // server's own fresh SCID; irrelevant to decryption
	header = append(header, 0)                   // token length = 0
	header = append(header, quicVarint(len(pkn)+len(payload)+quicTagLen)...)
	header = append(header, pkn...)
	headerLen := len(header)

	initialSecret := deriveInitialSecret(dcid)
	keys := deriveKeys(initialSecret, "server in")
	nonce := xorPacketNumberIntoIV(keys.iv, pkn)
	ciphertext, err := aesGCMSeal(keys.key, nonce, header, payload)
	if err != nil {
		t.Fatalf("seal test server packet: %v", err)
	}

	sampleStart := 4 - len(pkn)
	mask, err := hpMask(keys.hp, ciphertext[sampleStart:sampleStart+sampleLen])
	if err != nil {
		t.Fatalf("mask test server packet: %v", err)
	}
	header[0] ^= mask[0] & 0x0f
	for i := range pkn {
		header[headerLen-len(pkn)+i] ^= mask[1+i]
	}

	return append(header, ciphertext...)
}

func TestBuildClientProbe(t *testing.T) {
	dcid := fill(8, 0x11)
	scid := fill(8, 0x22)
	random := fill(32, 0x33)

	probe, err := buildClientProbe("example.com", dcid, scid, random)
	if err != nil {
		t.Fatalf("buildClientProbe: %v", err)
	}
	if len(probe.packet) < minInitialDatagramSize {
		t.Errorf("client probe datagram is %d bytes, want >= %d (RFC 9000 S14.1)", len(probe.packet), minInitialDatagramSize)
	}
	if probe.packet[0]&0xf0 != 0xc0 {
		t.Errorf("first byte %#x doesn't look like a long-header Initial packet", probe.packet[0])
	}
	if got := probe.packet[1:5]; !bytes.Equal(got, []byte{0, 0, 0, quicVersion1}) {
		t.Errorf("version bytes = %x, want QUICv1", got)
	}
}

func TestBuildClientProbeRejectsShortDCID(t *testing.T) {
	_, err := buildClientProbe("example.com", fill(4, 0), fill(8, 0), fill(32, 0))
	if err == nil {
		t.Fatal("expected an error for a < 8 byte DCID (RFC 9000 S7.2)")
	}
}

var cpsTokenRe = regexp.MustCompile(`<b 0x([0-9a-f]+)>|<r (\d+)>`)

type cpsToken struct {
	hex string // set for a <b> block (the literal wire bytes, as hex)
	n   int    // set (>0) for an <r> tag
}

func tokenizeCps(t *testing.T, chain string) []cpsToken {
	t.Helper()
	matches := cpsTokenRe.FindAllStringSubmatch(chain, -1)
	if matches == nil {
		t.Fatalf("chain has no recognizable tokens: %q", chain)
	}
	tokens := make([]cpsToken, 0, len(matches))
	for _, m := range matches {
		if m[1] != "" {
			tokens = append(tokens, cpsToken{hex: m[1]})
		} else {
			n, err := strconv.Atoi(m[2])
			if err != nil {
				t.Fatalf("bad <r N> tag %q: %v", m[0], err)
			}
			tokens = append(tokens, cpsToken{n: n})
		}
	}
	return tokens
}

func TestParseServerReplyStructure(t *testing.T) {
	dcid := fill(8, 0x11)
	resp := buildTestServerInitial(t, dcid, fill(8, 0x44), 0xaa, 0xbb)

	chain, err := ParseServerReply(resp, dcid)
	if err != nil {
		t.Fatalf("ParseServerReply: %v", err)
	}
	if !strings.HasPrefix(chain, "<b 0x") {
		t.Fatalf("chain should open on a literal block (the header), got: %q", chain)
	}
	if !strings.HasSuffix(chain, "<r 16>") {
		t.Fatalf("chain should end on the AEAD tag's <r 16>, got: %q", chain)
	}

	tokens := tokenizeCps(t, chain)
	var randomTagCount int
	for _, tok := range tokens {
		if tok.n > 0 {
			randomTagCount++
		}
	}
	if randomTagCount != 3 {
		t.Errorf("expected exactly 3 <r N> tags (Random, key_share, tag), got %d in %q", randomTagCount, chain)
	}
	if len(tokens) != 6 {
		t.Errorf("expected literal/random/literal/random/literal/random (6 tokens), got %d: %q", len(tokens), chain)
	}
}

// TestParseServerReplyRandomizesOnlyTheRealRandomSpans is the load-bearing
// correctness test: AES-GCM's underlying CTR-mode confidentiality means
// changing one plaintext byte changes exactly the ciphertext byte at that
// same position (and, since GCM's tag covers everything, the trailing tag)
// -- nothing else. So two otherwise-identical server replies that differ
// only in their real Random/key_share plaintext content must produce chains
// that agree everywhere except: (a) the <r N> spans themselves, and (b) up
// to the first few bytes of Random that the header-protection sample window
// forces to stay literal (RFC 9001 S5.4.2 always samples starting 4 bytes
// into the PN field, regardless of where Random happens to fall -- the same
// tension i1Generators.ts's quicCutParts already has for the client's own
// Random field, not new to this parser). Token 1's own <r N> length reveals
// exactly how many of Random's 32 bytes survived that overlap, so the
// allowed divergence at the tail of token 0 is computed from the chain
// itself, not re-derived from the production formula (which would risk
// sharing the same bug instead of catching it). Anywhere else a literal
// token differs, or a real difference falls outside a marked span, the
// offset math mislabeled a connection-fresh field as permanently frozen (or
// vice versa).
func TestParseServerReplyRandomizesOnlyTheRealRandomSpans(t *testing.T) {
	dcid := fill(8, 0x11)
	scid := fill(8, 0x44)
	respA := buildTestServerInitial(t, dcid, scid, 0xaa, 0xbb)
	respB := buildTestServerInitial(t, dcid, scid, 0x55, 0xcc)

	chainA, err := ParseServerReply(respA, dcid)
	if err != nil {
		t.Fatalf("ParseServerReply(A): %v", err)
	}
	chainB, err := ParseServerReply(respB, dcid)
	if err != nil {
		t.Fatalf("ParseServerReply(B): %v", err)
	}

	tokensA := tokenizeCps(t, chainA)
	tokensB := tokenizeCps(t, chainB)
	if len(tokensA) != len(tokensB) {
		t.Fatalf("token count differs between two structurally-identical captures: %d vs %d", len(tokensA), len(tokensB))
	}
	if len(tokensA) < 2 || tokensA[1].hex != "" {
		t.Fatalf("expected token 1 to be Random's <r N> tag")
	}
	if tokensA[1].n != tokensB[1].n {
		t.Fatalf("Random's randomized length differs between captures: %d vs %d", tokensA[1].n, tokensB[1].n)
	}
	sampleWindowOverlap := helloRandLen - tokensA[1].n // bytes of Random forced literal by the sample window

	for i := range tokensA {
		a, b := tokensA[i], tokensB[i]
		isLiteral := a.hex != ""
		if isLiteral != (b.hex != "") {
			t.Fatalf("token %d is literal in one capture and random in the other", i)
		}
		if !isLiteral {
			if a.n != b.n {
				t.Errorf("random token %d has different lengths between captures: %d vs %d", i, a.n, b.n)
			}
			continue
		}
		if i == 0 && sampleWindowOverlap > 0 {
			cut := len(a.hex) - 2*sampleWindowOverlap // exclude the sample-window-forced tail (2 hex chars/byte)
			if cut < 0 {
				t.Fatalf("token 0 (%d hex chars) is shorter than the computed sample-window overlap (%d bytes)", len(a.hex), sampleWindowOverlap)
			}
			if a.hex[:cut] != b.hex[:cut] {
				t.Errorf("token 0's structural prefix (excluding the %d-byte sample-window overlap) differs between captures:\n  A: %s\n  B: %s", sampleWindowOverlap, a.hex, b.hex)
			}
			continue
		}
		if a.hex != b.hex {
			t.Errorf("literal token %d differs between captures that only changed Random/key_share content:\n  A: %s\n  B: %s", i, a.hex, b.hex)
		}
	}
}

func TestParseServerReplyRejectsVersionNegotiation(t *testing.T) {
	pkt := []byte{0xc0, 0, 0, 0, 0} // version = 0 marks a Version Negotiation packet
	pkt = append(pkt, fill(30, 0)...)
	if _, err := ParseServerReply(pkt, fill(8, 0x11)); err == nil {
		t.Fatal("expected an error for a Version Negotiation reply")
	}
}

func TestParseServerReplyRejectsGarbage(t *testing.T) {
	if _, err := ParseServerReply(fill(40, 0xff), fill(8, 0x11)); err == nil {
		t.Fatal("expected an error for a reply that isn't a real Initial packet")
	}
}
