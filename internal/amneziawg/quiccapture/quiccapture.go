// Package quiccapture builds a real QUIC v1 Initial ClientHello, sends it to
// an admin-supplied host on UDP/443, and turns the real server's Initial
// reply into an AmneziaWG CPS chain (the same <b 0x...>/<r N> grammar the
// frontend's i1Generators.ts profiles already produce) -- I2-I5 material
// built from an actual live handshake exchange, not synthesized locally.
//
// The client-side packet construction (HKDF-Expand-Label derivation, AES-GCM
// seal, AES header-protection mask) is a direct Go port of the already
// shipped and cross-checked frontend/src/lib/xray/i1Generators.ts genQuicI1:
// same constants, same formulas, same QUICv1 initial salt (RFC 9001 Appendix
// A). serverparse.go adds the harder new half genQuicI1 never needed: opening
// the *server's* Initial packet (deriving its "server in"-labeled keys from
// the same DCID we chose, which is exactly why this is possible without a
// full handshake -- QUIC Initial secrets are public-derivation, not secret)
// and locating the two genuinely random spans inside a real ServerHello
// (Random, the key_share ephemeral public key) so they become <r N> instead
// of frozen literals -- the same "randomness only where real traffic would
// have it" rule already applied to every other CPS profile in this fork,
// including the already-shipped browser TLS ClientHello profiles' own
// key_share handling. Freezing those bytes literally would turn one real
// capture into a static fingerprint reused on every future connection,
// actually worse than not capturing at all.
//
// Verified against real production QUIC endpoints, not just synthetic
// round-trip tests -- see tlsClientHelloMinimal's and findCryptoFrame's own
// doc comments for the two real interop gaps a live run against Google,
// Facebook, and both of Cloudflare's public endpoints (www.cloudflare.com
// and the dedicated cloudflare-quic.com interop-test domain) surfaced and
// fixed. All four now complete a full real capture.
package quiccapture

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
)

const (
	quicVersion1     = 1
	quicTagLen       = 16
	helloRandLen     = 32
	sampleLen        = 16 // AES block size; RFC 9001 5.4.2's header-protection sample width
	tlsHandshakeHead = 6  // msg_type(1) + length(3) + legacy_version(2), before Random
)

// QUICv1 initial salt, RFC 9001 Appendix A -- identical to i1Generators.ts's
// QUIC_INITIAL_SALT.
var quicInitialSalt = []byte{
	0x38, 0x76, 0x2c, 0xf7, 0xf5, 0x59, 0x34, 0xb3, 0x4d, 0x17,
	0x9a, 0xe6, 0xa4, 0xc8, 0x0c, 0xad, 0xcc, 0xbb, 0x7f, 0x0a,
}

// quicVarint encodes a QUIC variable-length integer (RFC 9000 S16). Every
// value used in this package fits in 1 or 2 bytes.
func quicVarint(x int) []byte {
	switch {
	case x < 0x40:
		return []byte{byte(x)}
	case x < 0x4000:
		return []byte{0x40 | byte(x>>8), byte(x)}
	default:
		return []byte{0x80 | byte(x>>24), byte(x >> 16), byte(x >> 8), byte(x)}
	}
}

// quicVarintDecode reads one QUIC varint starting at data[0], returning its
// value and encoded width. Only used when parsing the real server reply,
// where the actual width isn't known in advance.
func quicVarintDecode(data []byte) (value, width int, err error) {
	if len(data) == 0 {
		return 0, 0, fmt.Errorf("quiccapture: empty varint")
	}
	width = 1 << (data[0] >> 6)
	if len(data) < width {
		return 0, 0, fmt.Errorf("quiccapture: truncated varint")
	}
	value = int(data[0] & 0x3f)
	for i := 1; i < width; i++ {
		value = (value << 8) | int(data[i])
	}
	return value, width, nil
}

// str8 is a single-byte length prefix + bytes -- matches i1Generators.ts's
// quicStr8, used for the DCID field and HKDF label strings (both far under
// 255 bytes).
func str8(b []byte) []byte {
	return append([]byte{byte(len(b))}, b...)
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

// hkdfExpandLabel mirrors i1Generators.ts's quicExpandLabel: HKDF-Expand-Label
// (RFC 8446 S7.1) truncated to a single HMAC block, valid because every QUIC
// secret derived here is <=32 bytes (SHA-256's own output size).
func hkdfExpandLabel(secret []byte, label string, length int) []byte {
	info := []byte{byte(length >> 8), byte(length)}
	info = append(info, str8([]byte("tls13 "+label))...)
	info = append(info, str8(nil)...) // empty Context
	info = append(info, 1)            // HKDF-Expand's T(1) block counter
	return hmacSHA256(secret, info)[:length]
}

type quicKeySet struct {
	key, iv, hp []byte
}

// deriveInitialSecret is HKDF-Extract(initial_salt, dcid) -- HMAC-based
// extract is just HMAC(salt, ikm).
func deriveInitialSecret(dcid []byte) []byte {
	return hmacSHA256(quicInitialSalt, dcid)
}

// deriveKeys derives one side's (client or server) Initial key/iv/hp from the
// shared Initial secret, per RFC 9001 S5.1. label is "client in" or
// "server in".
func deriveKeys(initialSecret []byte, label string) quicKeySet {
	secret := hkdfExpandLabel(initialSecret, label, 32)
	return quicKeySet{
		key: hkdfExpandLabel(secret, "quic key", 16),
		iv:  hkdfExpandLabel(secret, "quic iv", 12),
		hp:  hkdfExpandLabel(secret, "quic hp", 16),
	}
}

func aesGCMSeal(key, iv, aad, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Seal(nil, iv, plaintext, aad), nil
}

func aesGCMOpen(key, iv, aad, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, iv, ciphertext, aad)
}

// hpMask is RFC 9001 S5.4.3's header-protection mask: one raw AES-ECB block
// encryption of a 16-byte ciphertext sample under the header-protection key.
// Go's cipher.Block.Encrypt is exactly one ECB block op, no CTR-equivalence
// trick needed (unlike the Web Crypto port, which has no ECB primitive).
func hpMask(hpKey, sample []byte) ([]byte, error) {
	block, err := aes.NewCipher(hpKey)
	if err != nil {
		return nil, err
	}
	if len(sample) != aes.BlockSize {
		return nil, fmt.Errorf("quiccapture: header-protection sample must be %d bytes, got %d", aes.BlockSize, len(sample))
	}
	mask := make([]byte, aes.BlockSize)
	block.Encrypt(mask, sample)
	return mask, nil
}

// xorPacketNumberIntoIV builds the per-packet AEAD nonce (RFC 9001 S5.3): the
// base IV XORed with the packet number in its low-order bytes.
func xorPacketNumberIntoIV(iv, pkn []byte) []byte {
	nonce := append([]byte(nil), iv...)
	off := len(nonce) - len(pkn)
	for i, b := range pkn {
		nonce[off+i] ^= b
	}
	return nonce
}

// cpsChain is a minimal Go mirror of the frontend's CpsChain builder: <b
// 0xHEX> for a literal byte span, <r N> for a backend-generated-fresh-every-
// send placeholder.
type cpsChain struct {
	parts []string
}

func (c *cpsChain) bytes(b []byte) {
	if len(b) == 0 {
		return
	}
	c.parts = append(c.parts, fmt.Sprintf("<b 0x%x>", b))
}

func (c *cpsChain) random(n int) {
	if n <= 0 {
		return
	}
	c.parts = append(c.parts, fmt.Sprintf("<r %d>", n))
}

func (c *cpsChain) String() string {
	out := ""
	for _, p := range c.parts {
		out += p
	}
	return out
}

func u16(n int) []byte { return []byte{byte(n >> 8), byte(n)} }

// tlsExtension wraps a 2-byte type + 2-byte length + body into one TLS
// extension entry (RFC 8446 4.2).
func tlsExtension(typ int, body []byte) []byte {
	return append(append(u16(typ), u16(len(body))...), body...)
}

// quicTransportParam encodes one QUIC transport parameter (RFC 9000 S18.2):
// varint id + varint length + value.
func quicTransportParam(id int, value []byte) []byte {
	out := quicVarint(id)
	out = append(out, quicVarint(len(value))...)
	return append(out, value...)
}

func quicTransportParamVarint(id, value int) []byte {
	return quicTransportParam(id, quicVarint(value))
}

// quicTransportParameters builds the quic_transport_parameters TLS extension
// (RFC 9001 S8.2: "endpoints MUST send... An endpoint that receives...
// without this extension MUST close the connection"). Not optional the way
// most of this ClientHello's other extensions might loosely seem --
// initial_source_connection_id in particular MUST match the packet header's
// own SCID field or a spec-compliant server closes with
// TRANSPORT_PARAMETER_ERROR. Values otherwise follow ordinary real-client
// defaults (RFC 9000 S18.2's own suggested ranges); this connection is
// discarded after one packet exchange, so their exact size doesn't matter
// beyond being plausible.
func quicTransportParameters(scid []byte) []byte {
	var out []byte
	out = append(out, quicTransportParamVarint(0x01, 30000)...)   // max_idle_timeout (ms)
	out = append(out, quicTransportParamVarint(0x03, 1350)...)    // max_udp_payload_size
	out = append(out, quicTransportParamVarint(0x04, 1048576)...) // initial_max_data
	out = append(out, quicTransportParamVarint(0x05, 262144)...)  // initial_max_stream_data_bidi_local
	out = append(out, quicTransportParamVarint(0x06, 262144)...)  // initial_max_stream_data_bidi_remote
	out = append(out, quicTransportParamVarint(0x07, 262144)...)  // initial_max_stream_data_uni
	out = append(out, quicTransportParamVarint(0x08, 100)...)     // initial_max_streams_bidi
	out = append(out, quicTransportParamVarint(0x09, 100)...)     // initial_max_streams_uni
	out = append(out, quicTransportParamVarint(0x0b, 25)...)      // max_ack_delay (ms)
	out = append(out, quicTransportParam(0x0f, scid)...)          // initial_source_connection_id
	return out
}

// tlsClientHelloMinimal builds a genuinely TLS-1.3-legal ClientHello: real
// cipher suites plus the extensions RFC 8446/9001 actually require for a
// server to proceed past parsing (supported_versions, supported_groups,
// key_share, signature_algorithms, alpn, quic_transport_parameters). Unlike
// i1Generators.ts's tlsClientHello (a deliberately non-spec-legal, DPI-
// shape-only packet that never touches a real network) or the browser-
// fingerprint profiles (which mimic a specific client), this one only needs
// to be *generic and acceptable*, not identify as anything -- its own bytes
// are discarded after sending; only the real server's reply becomes the
// eventual CPS chain. A first live-network run against real Google/
// Cloudflare/Facebook QUIC endpoints caught two real gaps the hard way: an
// empty cipher-suite list with no supported_versions/key_share (fine for a
// packet nothing ever parses) got an immediate Initial-level
// CONNECTION_CLOSE (frame type 0x1c) instead of a ServerHello: real TLS
// servers actually parse this one. Adding those got past parsing on a
// lenient test server (cloudflare-quic.com replied with an ACK, not a
// close) but production endpoints still closed -- because
// quic_transport_parameters, an RFC 9001-mandatory extension with no TLS
// equivalent, was still missing entirely.
func tlsClientHelloMinimal(sni string, random, scid []byte) []byte {
	name := []byte(sni)
	sniEntry := append(append([]byte{0}, u16(len(name))...), name...) // name_type=host_name
	sniExt := tlsExtension(0x0000, append(u16(len(sniEntry)), sniEntry...))

	groupsExt := tlsExtension(0x000a, append(u16(2), u16(29)...)) // supported_groups: x25519

	sigAlgs := []int{0x0804, 0x0403, 0x0401, 0x0501, 0x0503, 0x0806, 0x0805}
	sigBody := u16(len(sigAlgs) * 2)
	for _, s := range sigAlgs {
		sigBody = append(sigBody, u16(s)...)
	}
	sigAlgsExt := tlsExtension(0x000d, sigBody) // signature_algorithms

	versionsExt := tlsExtension(0x002b, []byte{2, 0x03, 0x04}) // supported_versions: TLS 1.3 only

	fakePubKey := make([]byte, 32) // x25519 public keys have no server-checkable validity beyond length
	_, _ = rand.Read(fakePubKey)
	keyShareEntry := append(append(u16(29), u16(len(fakePubKey))...), fakePubKey...)
	keyShareExt := tlsExtension(0x0033, append(u16(len(keyShareEntry)), keyShareEntry...))

	alpnProtos := [][]byte{[]byte("h3")}
	alpnBody := []byte{}
	for _, p := range alpnProtos {
		alpnBody = append(alpnBody, byte(len(p)))
		alpnBody = append(alpnBody, p...)
	}
	alpnExt := tlsExtension(0x0010, append(u16(len(alpnBody)), alpnBody...)) // application_layer_protocol_negotiation

	qtp := quicTransportParameters(scid)
	qtpExt := tlsExtension(0x0039, qtp) // quic_transport_parameters

	var exts []byte
	for _, e := range [][]byte{sniExt, groupsExt, sigAlgsExt, versionsExt, keyShareExt, alpnExt, qtpExt} {
		exts = append(exts, e...)
	}

	cipherSuites := []int{0x1301, 0x1302, 0x1303} // the 3 real TLS 1.3 AEAD suites
	cipherBody := u16(len(cipherSuites) * 2)
	for _, c := range cipherSuites {
		cipherBody = append(cipherBody, u16(c)...)
	}

	body := []byte{0x03, 0x03} // legacy_version = TLS 1.2 (always, even for TLS 1.3 -- real version is in supported_versions)
	body = append(body, random...)
	body = append(body, 0) // legacy_session_id: empty
	body = append(body, cipherBody...)
	body = append(body, 1, 0) // compression_methods: length=1, null
	body = append(body, u16(len(exts))...)
	body = append(body, exts...)

	hello := []byte{0x01, byte(len(body) >> 16), byte(len(body) >> 8), byte(len(body))}
	return append(hello, body...)
}

func cryptoFrame(data []byte) []byte {
	out := []byte{0x06}
	out = append(out, quicVarint(0)...)
	out = append(out, quicVarint(len(data))...)
	return append(out, data...)
}

// minInitialDatagramSize is RFC 9000 S14.1's mandatory floor: "a client MUST
// expand the payload of all UDP datagrams carrying Initial packets to at
// least the smallest allowed maximum datagram size (1200 bytes)". This is an
// anti-amplification defense -- a compliant server limits its own reply to
// at most 3x whatever it received from an unvalidated client address, so an
// undersized probe risks a truncated (or outright dropped) response. This
// doesn't apply to i1Generators.ts's genQuicI1, which only needs to clear a
// local 20-byte header-protection-sample minimum since it never talks to a
// real server.
const minInitialDatagramSize = 1200

// clientProbe is the ready-to-send wire bytes of our own client Initial
// packet, plus everything needed to make sense of the server's reply to it.
type clientProbe struct {
	packet    []byte
	dcid      []byte
	pkn       []byte
	headerLen int
}

// buildClientProbe constructs a real QUICv1 Initial packet carrying a
// minimal TLS ClientHello for host. Diverges from i1Generators.ts's
// quicInitialPacket in the two places that only matter once a real server is
// on the other end: dcid must be RFC 9000 S7.2's mandatory >=8 bytes (a
// 1-byte DCID, fine for a locally-synthesized packet nothing ever validates,
// gets silently dropped by real implementations), scid is a realistic
// non-empty 8 bytes rather than empty (real clients don't send empty SCIDs;
// staying unremarkable here is also just better camouflage), and the payload
// is padded with zero bytes (QUIC PADDING frames, type 0x00) to satisfy the
// minInitialDatagramSize floor above.
func buildClientProbe(host string, dcid, scid, random []byte) (*clientProbe, error) {
	if len(dcid) < 8 {
		return nil, fmt.Errorf("quiccapture: dcid must be >= 8 bytes (RFC 9000 S7.2), got %d", len(dcid))
	}
	pkn := []byte{0}
	hello := tlsClientHelloMinimal(host, random, scid)
	crypto := cryptoFrame(hello)

	// Two passes: the header's own Length field varint-encodes the
	// post-padding payload size, so its byte width (1 vs 2 bytes) isn't
	// known until padding is decided -- build once with zero padding purely
	// to measure the real header length, then compute exact padding from
	// that (never an under-estimate, unlike guessing the varint width
	// up front).
	buildHeader := func(payloadLen int) []byte {
		h := []byte{0xc0 | byte(len(pkn)-1)}
		h = append(h, 0, 0, 0, quicVersion1)
		h = append(h, str8(dcid)...)
		h = append(h, str8(scid)...)
		h = append(h, 0) // empty Token length (varint(0))
		h = append(h, quicVarint(len(pkn)+payloadLen+quicTagLen)...)
		h = append(h, pkn...)
		return h
	}
	measuredHeaderLen := len(buildHeader(len(crypto)))
	paddingLen := minInitialDatagramSize - measuredHeaderLen - len(crypto) - quicTagLen
	if paddingLen < 0 {
		paddingLen = 0
	}
	payload := append(crypto, make([]byte, paddingLen)...)

	header := buildHeader(len(payload))
	headerLen := len(header)

	initialSecret := deriveInitialSecret(dcid)
	keys := deriveKeys(initialSecret, "client in")

	nonce := xorPacketNumberIntoIV(keys.iv, pkn)
	ciphertext, err := aesGCMSeal(keys.key, nonce, header, payload)
	if err != nil {
		return nil, fmt.Errorf("quiccapture: seal client probe: %w", err)
	}

	sampleStart := 4 - len(pkn)
	sample := ciphertext[sampleStart : sampleStart+sampleLen]
	mask, err := hpMask(keys.hp, sample)
	if err != nil {
		return nil, fmt.Errorf("quiccapture: mask client probe: %w", err)
	}
	header[0] ^= mask[0] & 0x0f
	for i := range pkn {
		header[headerLen-len(pkn)+i] ^= mask[1+i]
	}

	return &clientProbe{
		packet:    append(header, ciphertext...),
		dcid:      dcid,
		pkn:       pkn,
		headerLen: headerLen,
	}, nil
}
