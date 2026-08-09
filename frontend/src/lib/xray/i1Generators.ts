// AmneziaWG CPS signature-packet generators -- ported from the algorithms in
// vernette/warpscout's i1gen.go/quic.go (MIT), not the code itself. The real
// protocol has five independent CPS slots (i1-i5, confirmed against
// amneziawg-go v3.0.3's device/uapi.go: five separate UAPI setters,
// device.ipackets[0..4]) -- genI1 (kept that name; it shipped first, before
// i2-i5 existed) generates a chain for any of them, the caller decides which
// UAPI field the result is stored into. A generated chain's whole point is
// DPI plausibility, not entropy: one that looks like a real DNS/STUN/SIP/
// QUIC packet (randomness only where real traffic of that kind would
// actually have it -- transaction IDs, nonces, padding, TLS Random, AEAD
// tags) is far harder to fingerprint than a blob of pure random bytes.

const DNS_LABEL_MAX = 63;
const DNS_PAD_MIN = 60;
const DNS_PAD_MAX = 180;
const STUN_SOFTWARE_MIN = 16;
const STUN_SOFTWARE_MAX = 32;

// A mix of globally-reachable front domains and popular Russian services --
// ported from lucx-ui's RU domain pool (cps/domains.go), which the traffic
// this fork's admins actually see skews toward more than a purely global
// list would.
const I1_HOSTS = [
  'www.apple.com', 'www.google.com', 'www.microsoft.com', 'cdn.jsdelivr.net',
  'yandex.ru', 'vk.com', 'mail.ru', 'ozon.ru', 'wildberries.ru', 'rutube.ru', 'gosuslugi.ru', 'sberbank.ru', 'tbank.ru',
];

export type I1Profile = 'dns' | 'quic' | 'sip' | 'stun' | 'chrome' | 'firefox' | 'safari';
export type I1ProfileChoice = I1Profile | 'random';

export const I1_PROFILE_CHOICES: I1ProfileChoice[] = ['random', 'dns', 'quic', 'sip', 'stun', 'chrome', 'firefox', 'safari'];

// The 5 real CPS UAPI slots, in order -- shared by amneziawg.tsx (renders
// one identically-shaped generate row per slot) and InboundFormModal.tsx
// (keys the per-slot profile-choice state and regen handler).
export const CPS_SLOTS = ['i1', 'i2', 'i3', 'i4', 'i5'] as const;
export type CpsSlot = (typeof CPS_SLOTS)[number];

function randRange(min: number, max: number): number {
  return min + Math.floor(Math.random() * (max - min + 1));
}

// Builds a CPS chain string: <b 0xHEX> for literal bytes (hex must be
// lowercase, no separators, to match the panel backend's own
// GenerateObfuscation20-adjacent <r N>/<b ...> grammar), <tag N> for a
// backend-generated placeholder (r = N random bytes, rc = N random
// printable characters, rd = N random digits).
class CpsChain {
  private parts: string[] = [];
  private static encoder = new TextEncoder();

  bytes(b: Uint8Array): void {
    if (b.length === 0) return;
    let hex = '';
    for (const byte of b) hex += byte.toString(16).padStart(2, '0');
    this.parts.push(`<b 0x${hex}>`);
  }

  text(s: string): void {
    this.bytes(CpsChain.encoder.encode(s));
  }

  tag(name: string, n: number): void {
    this.parts.push(`<${name} ${n}>`);
  }

  toString(): string {
    return this.parts.join('');
  }
}

function u16be(n: number): [number, number] {
  return [(n >> 8) & 0xff, n & 0xff];
}

function u32be(n: number): [number, number, number, number] {
  return [(n >>> 24) & 0xff, (n >>> 16) & 0xff, (n >>> 8) & 0xff, n & 0xff];
}

// Length-prefixed DNS labels (one byte length + label bytes, per '.'),
// terminated by a zero byte. Returns null for a label that's empty or
// over the 63-byte DNS limit -- defensive only, none of I1_HOSTS can hit
// this today.
function dnsName(host: string): Uint8Array | null {
  const bytes: number[] = [];
  const encoder = new TextEncoder();
  for (const label of host.replace(/^\.+|\.+$/g, '').split('.')) {
    const labelBytes = encoder.encode(label);
    if (labelBytes.length === 0 || labelBytes.length > DNS_LABEL_MAX) return null;
    bytes.push(labelBytes.length, ...labelBytes);
  }
  bytes.push(0);
  return new Uint8Array(bytes);
}

// A standard A-record query plus an EDNS0 OPT record whose padding option
// (RFC 7830) carries the random bytes, so the packet stays a well-formed
// DNS query throughout.
function genDnsI1(host: string): string | null {
  const qname = dnsName(host);
  if (!qname) return null;
  const question = new Uint8Array([...qname, 0, 1, 0, 1]); // QTYPE=A, QCLASS=IN

  const padLen = randRange(DNS_PAD_MIN, DNS_PAD_MAX);
  const opt: number[] = [0]; // root name
  opt.push(...u16be(41)); // OPT
  opt.push(...u16be(4096)); // UDP payload size
  opt.push(0, 0, 0, 0); // extended rcode + flags
  opt.push(...u16be(padLen + 4)); // rdlength
  opt.push(...u16be(12)); // padding option code, RFC 7830
  opt.push(...u16be(padLen));

  const c = new CpsChain();
  c.tag('r', 2); // transaction id
  // flags=0x0100 (standard query, recursion desired), QDCOUNT=1,
  // ANCOUNT=0, NSCOUNT=0, ARCOUNT=1 (the OPT record below)
  c.bytes(new Uint8Array([0x01, 0x00, 0, 1, 0, 0, 0, 0, 0, 1]));
  c.bytes(question);
  c.bytes(new Uint8Array(opt));
  c.tag('r', padLen);
  return c.toString();
}

// STUN Binding Request with a SOFTWARE attribute whose value is random
// printable characters -- a real STUN client legitimately sends a
// human-readable software string here, so this stays plausible.
function genStunI1(): string {
  const softwareLen = randRange(STUN_SOFTWARE_MIN / 4, STUN_SOFTWARE_MAX / 4) * 4;
  const attrLen = 4 + softwareLen;

  const c = new CpsChain();
  c.bytes(new Uint8Array([
    ...u16be(0x0001), // Binding Request
    ...u16be(attrLen),
    ...u32be(0x2112a442), // magic cookie
  ]));
  c.tag('r', 12); // transaction id
  c.bytes(new Uint8Array([...u16be(0x8022), ...u16be(softwareLen)])); // SOFTWARE attribute header
  c.tag('rc', softwareLen);
  return c.toString();
}

// A SIP OPTIONS request (the standard SIP keepalive/capability probe) with
// randomized Via branch, From tag, Call-ID, and CSeq -- every field a real
// SIP client would vary between requests anyway.
function genSipI1(host: string): string {
  const c = new CpsChain();
  c.text(`OPTIONS sip:${host} SIP/2.0\r\n`);
  c.text(`Via: SIP/2.0/UDP 192.168.${randRange(0, 255)}.${randRange(2, 254)}:5060;branch=z9hG4bK`);
  c.tag('rc', 10);
  c.text('\r\nFrom: <sip:');
  c.tag('rc', 8);
  c.text(`@${host}>;tag=`);
  c.tag('rd', 6);
  c.text(`\r\nTo: <sip:${host}>\r\nCall-ID: `);
  c.tag('rc', 16);
  c.text(`@${host}\r\nCSeq: `);
  c.tag('rd', 4);
  c.text(' OPTIONS\r\nMax-Forwards: 70\r\nUser-Agent: PJSIP/2.13\r\nContent-Length: 0\r\n\r\n');
  return c.toString();
}

// ---- QUIC v1 Initial packet mimicry (RFC 9000/9001) ----
//
// Unlike DNS/SIP/STUN (pure byte-packing), this builds a real QUIC v1
// Initial packet: a TLS 1.3 ClientHello carrying the host as SNI, wrapped
// in a CRYPTO frame, AES-128-GCM-sealed under QUIC's own Initial secrets
// (HKDF-Extract via HMAC-SHA256, then HKDF-Expand-Label truncated to a
// single HMAC block -- every secret needed here is <=32 bytes, so one
// block is always enough), then AES header-protection masking (AES-ECB
// single-block, obtained through the standard AES-CTR equivalence: CTR
// with counter=sample and an all-zero plaintext block yields exactly
// AES_Encrypt(key, sample), since Web Crypto has no raw ECB mode). Only
// the TLS ClientHello's Random field and the GCM tag -- the two spans
// real traffic would vary on every single packet regardless -- become
// <r N> placeholders; everything else (header, framing, lengths, the
// otherwise-fixed extension bytes) stays a literal <b ...> block. This
// pipeline was cross-checked byte-for-byte against a fixed-test-vector
// port of the real upstream Go source before shipping (two hostnames,
// exercising both the single- and double-byte QUIC-varint-length paths).
//
// Needs crypto.subtle, i.e. a secure context (HTTPS or localhost) --
// isQuicI1Supported() guards this; the caller should check it before
// offering an explicit "quic" choice.

const QUIC_VERSION_1 = 1;
const QUIC_TAG_LEN = 16;
const QUIC_HELLO_RAND_LEN = 32;
const QUIC_SAMPLE_END = 20;
const TLS_HANDSHAKE_HEAD = 6;

const QUIC_INITIAL_SALT = new Uint8Array([
  0x38, 0x76, 0x2c, 0xf7, 0xf5, 0x59, 0x34, 0xb3, 0x4d, 0x17,
  0x9a, 0xe6, 0xa4, 0xc8, 0x0c, 0xad, 0xcc, 0xbb, 0x7f, 0x0a,
]);

export function isQuicI1Supported(): boolean {
  return typeof crypto !== 'undefined' && typeof crypto.subtle !== 'undefined';
}

// TS's bare `Uint8Array` defaults to `Uint8Array<ArrayBufferLike>`, which
// (as of TS 6, whose lib.dom.d.ts tightened `BufferSource`) is no longer
// assignable to crypto.subtle's parameter types even though every array
// here is genuinely ArrayBuffer-backed at runtime (plain `new Uint8Array`/
// `TextEncoder.encode`/`.slice()` never produce a SharedArrayBuffer one).
// This alias just makes the type annotations match that real guarantee.
type Bytes = Uint8Array<ArrayBuffer>;

function concatBytes(...arrays: Bytes[]): Bytes {
  const total = arrays.reduce((sum, a) => sum + a.length, 0);
  const out = new Uint8Array(total);
  let offset = 0;
  for (const a of arrays) {
    out.set(a, offset);
    offset += a.length;
  }
  return out;
}

function randomBytesSync(n: number): Bytes {
  const b = new Uint8Array(n);
  crypto.getRandomValues(b);
  return b;
}

// QUIC variable-length integer encoding (RFC 9000 S16): top 2 bits pick a
// 1/2/4-byte width by value range; every value used here fits in 1 or 2.
function quicVarint(x: number): Bytes {
  if (x < 0x40) return new Uint8Array([x]);
  if (x < 0x4000) return new Uint8Array([0x40 | (x >> 8), x & 0xff]);
  return new Uint8Array([0x80 | ((x >> 24) & 0xff), (x >> 16) & 0xff, (x >> 8) & 0xff, x & 0xff]);
}

// Single-byte length prefix + bytes -- safe here since every use (a 1-byte
// DCID, an HKDF label string under ~20 bytes) is far under 255.
function quicStr8(b: Bytes): Bytes {
  return concatBytes(new Uint8Array([b.length]), b);
}

async function hmacSha256(keyBytes: Bytes, data: Bytes): Promise<Bytes> {
  const key = await crypto.subtle.importKey('raw', keyBytes, { name: 'HMAC', hash: 'SHA-256' }, false, ['sign']);
  const sig = await crypto.subtle.sign('HMAC', key, data);
  return new Uint8Array(sig);
}

// HKDF-Expand-Label truncated to a single HMAC block -- every QUIC secret
// needed here (<=32 bytes) fits in one. The trailing [0, 1] is the empty
// Context field's own 1-byte length prefix, followed by HKDF-Expand's
// block-counter byte (0x01, since only T(1) is ever needed).
async function quicExpandLabel(secret: Bytes, label: string, length: number): Promise<Bytes> {
  const info = concatBytes(
    new Uint8Array([(length >> 8) & 0xff, length & 0xff]),
    quicStr8(new TextEncoder().encode(`tls13 ${label}`)),
    new Uint8Array([0, 1]),
  );
  return (await hmacSha256(secret, info)).slice(0, length);
}

async function quicInitialKeys(dcid: Bytes): Promise<{ key: Bytes; iv: Bytes; hp: Bytes }> {
  const initial = await hmacSha256(QUIC_INITIAL_SALT, dcid);
  const client = await quicExpandLabel(initial, 'client in', 32);
  const [key, iv, hp] = await Promise.all([
    quicExpandLabel(client, 'quic key', 16),
    quicExpandLabel(client, 'quic iv', 12),
    quicExpandLabel(client, 'quic hp', 16),
  ]);
  return { key, iv, hp };
}

async function aesGcmSeal(keyBytes: Bytes, iv: Bytes, aad: Bytes, plaintext: Bytes): Promise<Bytes> {
  const key = await crypto.subtle.importKey('raw', keyBytes, { name: 'AES-GCM' }, false, ['encrypt']);
  const sealed = await crypto.subtle.encrypt(
    { name: 'AES-GCM', iv, additionalData: aad, tagLength: 128 },
    key,
    plaintext,
  );
  // Web Crypto appends the 16-byte tag to the ciphertext, matching Go's
  // own cipher.NewGCM().Seal convention -- no reassembly needed.
  return new Uint8Array(sealed);
}

// AES-ECB single-block encrypt via the standard AES-CTR equivalence (Web
// Crypto exposes no raw ECB mode): encrypting an all-zero plaintext block
// under AES-CTR with counter=sample yields exactly the keystream, i.e.
// AES_Encrypt(key, sample) -- precisely the header-protection mask RFC
// 9001 S5.4.3 defines.
async function aesEcbSingleBlock(keyBytes: Bytes, block: Bytes): Promise<Bytes> {
  const key = await crypto.subtle.importKey('raw', keyBytes, { name: 'AES-CTR' }, false, ['encrypt']);
  const mask = await crypto.subtle.encrypt({ name: 'AES-CTR', counter: block, length: 128 }, key, new Uint8Array(16));
  return new Uint8Array(mask);
}

// A minimal TLS 1.3 ClientHello: legacy_version + Random + empty
// session_id/cipher_suites/compression, plus a single server_name (SNI)
// extension. Not a spec-legal ClientHello (empty cipher suite list) --
// doesn't need to be, since nothing ever parses it as real TLS; it only
// needs to *look* length-consistent to a passive DPI heuristic.
function tlsClientHello(sni: string): Bytes {
  const name = new TextEncoder().encode(sni);
  const serverNameEntry = concatBytes(
    new Uint8Array([0]), // name_type = host_name
    new Uint8Array([(name.length >> 8) & 0xff, name.length & 0xff]),
    name,
  );
  const serverNameList = concatBytes(
    new Uint8Array([(serverNameEntry.length >> 8) & 0xff, serverNameEntry.length & 0xff]),
    serverNameEntry,
  );
  const ext = concatBytes(
    new Uint8Array([0, 0]), // extension type = server_name
    new Uint8Array([(serverNameList.length >> 8) & 0xff, serverNameList.length & 0xff]),
    serverNameList,
  );

  const body = concatBytes(
    new Uint8Array([0x03, 0x03]), // legacy_version = TLS 1.2
    randomBytesSync(QUIC_HELLO_RAND_LEN), // Random -- becomes <r 32>-ish below
    new Uint8Array([0, 0, 0, 0]), // empty session_id / cipher_suites / compression
    new Uint8Array([(ext.length >> 8) & 0xff, ext.length & 0xff]),
    ext,
  );
  return concatBytes(
    new Uint8Array([0x01, (body.length >> 16) & 0xff, (body.length >> 8) & 0xff, body.length & 0xff]),
    body,
  );
}

function quicCryptoFrame(data: Bytes): Bytes {
  return concatBytes(new Uint8Array([0x06]), quicVarint(0), quicVarint(data.length), data);
}

async function quicInitialPacket(
  dcid: Bytes,
  pkn: Bytes,
  payload: Bytes,
): Promise<{ packet: Bytes; headerLen: number }> {
  // No PADDING frame: the ClientHello already puts the packet well over
  // the 20-byte minimum the header-protection sample needs.
  const header = concatBytes(
    new Uint8Array([0xc0 | (pkn.length - 1)]),
    new Uint8Array([0, 0, 0, QUIC_VERSION_1]),
    quicStr8(dcid),
    new Uint8Array([0, 0]), // empty SCID length, empty Token length (varint(0))
    quicVarint(pkn.length + payload.length + QUIC_TAG_LEN),
    pkn,
  );
  const headerLen = header.length;

  const { key, iv, hp } = await quicInitialKeys(dcid);
  for (let i = 0; i < pkn.length; i++) {
    iv[iv.length - pkn.length + i] ^= pkn[i];
  }

  const ciphertext = await aesGcmSeal(key, iv, header, payload);

  const sampleStart = 4 - pkn.length;
  const sample = ciphertext.slice(sampleStart, QUIC_SAMPLE_END - pkn.length);
  const mask = await aesEcbSingleBlock(hp, sample);

  header[0] ^= mask[0] & 0x0f;
  for (let i = 0; i < pkn.length; i++) {
    header[headerLen - pkn.length + i] ^= mask[1 + i];
  }
  return { packet: concatBytes(header, ciphertext), headerLen };
}

// Alternating keep/randomize byte counts over the finished packet: the
// static prefix must cover the header-protection sample, so the first cut
// never starts before ciphertext byte (20 - len(pkn)).
function quicCutParts(frameHeaderLen: number, helloLen: number, pknLen: number, headerLen: number): number[] {
  let keep = frameHeaderLen + TLS_HANDSHAKE_HEAD;
  let randomized = QUIC_HELLO_RAND_LEN;
  const short = QUIC_SAMPLE_END - pknLen - keep;
  if (short > 0) {
    keep += short;
    randomized -= short;
  }
  return [headerLen + keep, randomized, helloLen - TLS_HANDSHAKE_HEAD - QUIC_HELLO_RAND_LEN, QUIC_TAG_LEN];
}

function quicToCps(packet: Bytes, parts: number[]): string {
  const c = new CpsChain();
  let offset = 0;
  let keepLiteral = true;
  for (const n of parts) {
    if (n > 0 && keepLiteral) {
      c.bytes(packet.slice(offset, offset + n));
    } else if (n > 0) {
      c.tag('r', n);
    }
    offset += n;
    keepLiteral = !keepLiteral;
  }
  return c.toString();
}

async function genQuicI1(host: string): Promise<string> {
  const dcid = randomBytesSync(1);
  const pkn = new Uint8Array([0]);
  const hello = tlsClientHello(host);
  const payload = quicCryptoFrame(hello);

  const { packet, headerLen } = await quicInitialPacket(dcid, pkn, payload);
  const parts = quicCutParts(payload.length - hello.length, hello.length, pkn.length, headerLen);
  return quicToCps(packet, parts);
}

// ---- Browser TLS 1.3 ClientHello fingerprint mimicry (Chrome/Firefox/Safari) ----
//
// A real TCP-style TLS record carrying a ClientHello, built to match one of
// three specific real browsers byte-for-byte: extension order, cipher-suite
// list, and GREASE placement all come from refraction-networking/utls
// (BSD-3-Clause, a fork of Go's own crypto/tls) -- not the code itself, a
// from-scratch port informed by reading the real Go source. Two kinds of
// utls source were used: its own testdata/ClientHello-JSON-{Chrome102,
// Firefox105,iOS14}.json fixtures (real captured ClientHellos, its own
// golden test vectors, not just documentation) for the logical extension
// order/content, and dicttls + u_tls_extensions.go + handshake_messages.go
// for the exact wire-level numeric IDs and per-extension byte layouts. A
// subtly wrong browser fingerprint reads as *more* suspicious to a
// fingerprint-aware DPI than an honest non-browser packet (see this
// section's own doc comment history for why this warranted going to the
// primary source instead of trusting a paraphrase), so every extension
// here matches one of those three real captures exactly, including quirks
// that look like they could be transcription mistakes but are genuinely
// present in the real capture (e.g. iOS14's signature_algorithms lists
// rsa_pss_rsae_sha384 twice in a row).
//
// Real per-connection material (TLS Random, the legacy session_id, and a
// real key_share's ephemeral public key) becomes <r 32> placeholders, the
// same "randomness only where real traffic would have it" rule the other
// profiles follow. GREASE values (RFC 8701: 0xωaωa for any nibble ω) are
// the one exception worth calling out: real browsers pick a value fresh
// per *connection*, but amneziawg-go's CPS grammar has no "one of these 16
// shapes" placeholder, only <r N> (uniformly random, which would almost
// never happen to land on a GREASE-shaped value by chance) -- so, matching
// this file's existing precedent for other *shape* values that get fixed
// once per "regenerate" click rather than re-randomized per packet (e.g.
// STUN's own softwareLen), each GREASE value here is computed once in
// TypeScript, via the same algorithm BoringSSL itself uses (confirmed
// against utls's GetBoringGREASEValue), and baked in as a literal.
// Confirmed via utls's own ssl_grease_* constants that a real ClientHello
// uses UP TO 5 independently-random GREASE values in one connection, not
// one value reused everywhere: one for the cipher-suite entry, one shared
// between the supported_groups list and the key_share group field, one
// each for the first/last GREASE extension markers, and one for the
// supported_versions entry.

const TLS_EXT = {
  serverName: 0,
  statusRequest: 5,
  supportedGroups: 10,
  ecPointFormats: 11,
  signatureAlgorithms: 13,
  alpn: 16,
  sct: 18,
  padding: 21,
  extendedMasterSecret: 23,
  compressCertificate: 27,
  recordSizeLimit: 28,
  delegatedCredentials: 34,
  sessionTicket: 35,
  supportedVersions: 43,
  pskKeyExchangeModes: 45,
  keyShare: 51,
  renegotiationInfo: 0xff01,
  applicationSettings: 0x4469, // Chrome-only (ALPS), not IANA-assigned
} as const;

// TLS 1.3/1.2 cipher suite IDs actually used by the 3 mimicked browsers'
// real captures (confirmed against utls's dicttls/cipher_suites.go).
const TLS_CIPHER = {
  AES_128_GCM_SHA256: 0x1301,
  AES_256_GCM_SHA384: 0x1302,
  CHACHA20_POLY1305_SHA256: 0x1303,
  ECDHE_ECDSA_AES_128_GCM_SHA256: 0xc02b,
  ECDHE_ECDSA_AES_256_GCM_SHA384: 0xc02c,
  ECDHE_RSA_AES_128_GCM_SHA256: 0xc02f,
  ECDHE_RSA_AES_256_GCM_SHA384: 0xc030,
  ECDHE_ECDSA_CHACHA20_POLY1305: 0xcca9,
  ECDHE_RSA_CHACHA20_POLY1305: 0xcca8,
  ECDHE_RSA_AES_128_CBC_SHA: 0xc013,
  ECDHE_RSA_AES_256_CBC_SHA: 0xc014,
  RSA_AES_128_GCM_SHA256: 0x009c,
  RSA_AES_256_GCM_SHA384: 0x009d,
  RSA_AES_128_CBC_SHA: 0x002f,
  RSA_AES_256_CBC_SHA: 0x0035,
  ECDHE_ECDSA_AES_256_CBC_SHA: 0xc00a,
  ECDHE_ECDSA_AES_128_CBC_SHA: 0xc009,
  ECDHE_ECDSA_AES_256_CBC_SHA384: 0xc024,
  ECDHE_ECDSA_AES_128_CBC_SHA256: 0xc023,
  ECDHE_RSA_AES_256_CBC_SHA384: 0xc028,
  ECDHE_RSA_AES_128_CBC_SHA256: 0xc027,
  RSA_AES_256_CBC_SHA256: 0x003d,
  RSA_AES_128_CBC_SHA256: 0x003c,
  ECDHE_ECDSA_3DES_EDE_CBC_SHA: 0xc008,
  ECDHE_RSA_3DES_EDE_CBC_SHA: 0xc012,
  RSA_3DES_EDE_CBC_SHA: 0x000a,
} as const;

const NAMED_GROUP = { x25519: 29, secp256r1: 23, secp384r1: 24, secp521r1: 25, ffdhe2048: 256, ffdhe3072: 257 } as const;

const SIG_ALG = {
  ecdsa_secp256r1_sha256: 0x0403,
  ecdsa_secp384r1_sha384: 0x0503,
  ecdsa_secp521r1_sha512: 0x0603,
  ecdsa_sha1: 0x0203,
  rsa_pkcs1_sha1: 0x0201,
  rsa_pkcs1_sha256: 0x0401,
  rsa_pkcs1_sha384: 0x0501,
  rsa_pkcs1_sha512: 0x0601,
  rsa_pss_rsae_sha256: 0x0804,
  rsa_pss_rsae_sha384: 0x0805,
  rsa_pss_rsae_sha512: 0x0806,
} as const;

const TLS10_VERSION = 0x0301;
const TLS11_VERSION = 0x0302;
const TLS12_VERSION = 0x0303;
const TLS13_VERSION = 0x0304;
const CERT_COMPRESSION_BROTLI = 2;

// This generates a random value of the form 0xωaωa, for all 0 <= ω < 16 --
// ported directly from utls's GetBoringGREASEValue, itself a port of
// BoringSSL's own algorithm (see this section's own doc comment).
function randomGreaseValue(): number {
  const b = (Math.floor(Math.random() * 16) << 4) | 0x0a;
  return (b << 8) | b;
}

// One TLS extension's fully-built wire bytes, plus its own exact length --
// tracked together (not re-derived by measuring output later) so the
// extensions-list length prefix, computed before any extension is emitted,
// can never drift from what actually gets written.
interface ExtPart {
  len: number;
  emit: (c: CpsChain) => void;
}

function litExt(type: number, body: number[]): ExtPart {
  const bytes = new Uint8Array([...u16be(type), ...u16be(body.length), ...body]);
  return { len: bytes.length, emit: (c) => c.bytes(bytes) };
}

function emptyExt(type: number): ExtPart {
  return litExt(type, []);
}

function u8ListExt(type: number, values: number[]): ExtPart {
  return litExt(type, [values.length, ...values]); // ec_point_formats
}

function pskModesExt(modes: number[]): ExtPart {
  return litExt(TLS_EXT.pskKeyExchangeModes, [modes.length, ...modes]);
}

// supported_versions: 1-byte list-length (in bytes) + 2 bytes per version.
function u16ListExt1(type: number, values: number[]): ExtPart {
  const flat: number[] = [];
  for (const v of values) flat.push(...u16be(v));
  return litExt(type, [flat.length, ...flat]);
}

// supported_groups/signature_algorithms: 2-byte list-length (in bytes) + 2
// bytes per value.
function u16ListExt2(type: number, values: number[]): ExtPart {
  const flat: number[] = [];
  for (const v of values) flat.push(...u16be(v));
  return litExt(type, [...u16be(flat.length), ...flat]);
}

// alpn/application_settings share the identical body shape: 2-byte
// list-length + repeated {1-byte string-length, string}.
function alpnLikeBody(protocols: string[]): number[] {
  const flat: number[] = [];
  for (const p of protocols) {
    const b = Array.from(new TextEncoder().encode(p));
    flat.push(b.length, ...b);
  }
  return [...u16be(flat.length), ...flat];
}
function alpnExt(protocols: string[]): ExtPart {
  return litExt(TLS_EXT.alpn, alpnLikeBody(protocols));
}
function alpsExt(protocols: string[]): ExtPart {
  return litExt(TLS_EXT.applicationSettings, alpnLikeBody(protocols));
}

function compressCertExt(algos: number[]): ExtPart {
  const flat: number[] = [];
  for (const a of algos) flat.push(...u16be(a));
  return litExt(TLS_EXT.compressCertificate, [flat.length, ...flat]);
}

function recordSizeLimitExt(limit: number): ExtPart {
  return litExt(TLS_EXT.recordSizeLimit, [...u16be(limit)]);
}

function renegotiationInfoExt(): ExtPart {
  return litExt(TLS_EXT.renegotiationInfo, [0]); // initial handshake: zero-length
}

function statusRequestExt(): ExtPart {
  return litExt(TLS_EXT.statusRequest, [1, 0, 0, 0, 0]); // OCSP type + two zero-length uint16s
}

function serverNameExt(host: string): ExtPart {
  const name = Array.from(new TextEncoder().encode(host));
  const entry = [0, ...u16be(name.length), ...name]; // name_type = host_name
  return litExt(TLS_EXT.serverName, [...u16be(entry.length), ...entry]);
}

// key_share is the one extension that genuinely mixes literal bytes (a
// GREASE group's near-empty placeholder) with real per-connection material
// (a real group's ephemeral public key) -- so unlike every other extension
// above, its length can't be read off a plain literal-body array; each
// share contributes 4 (group id + data-length header) + its data length
// (1 for a GREASE placeholder, 32 for a real x25519/secp256r1 key).
function keyShareExt(shares: { group: number; greaseBody?: number[] }[]): ExtPart {
  let listLen = 0;
  for (const s of shares) listLen += 4 + (s.greaseBody ? s.greaseBody.length : 32);
  return {
    len: 4 + 2 + listLen,
    emit: (c) => {
      c.bytes(new Uint8Array([...u16be(TLS_EXT.keyShare), ...u16be(2 + listLen), ...u16be(listLen)]));
      for (const s of shares) {
        if (s.greaseBody) {
          c.bytes(new Uint8Array([...u16be(s.group), ...u16be(s.greaseBody.length), ...s.greaseBody]));
        } else {
          c.bytes(new Uint8Array([...u16be(s.group), ...u16be(32)]));
          c.tag('r', 32);
        }
      }
    },
  };
}

// Assembles a full TLS record carrying a ClientHello handshake message --
// shared wrapper (record header, handshake header, legacy_version, Random,
// session_id, cipher_suites, compression_methods, extensions-list length
// prefix) for all 3 browser profiles; confirmed against utls's own
// (unmodified-from-Go) handshake_messages.go marshal code.
function buildTlsClientHello(cipherSuites: number[], exts: ExtPart[]): string {
  const c = new CpsChain();
  const extsLen = exts.reduce((sum, e) => sum + e.len, 0);
  const cipherLen = cipherSuites.length * 2;
  const bodyLen = 2 /* legacy_version */ + 32 /* Random */ + 1 + 32 /* session_id */
    + 2 + cipherLen /* cipher_suites */ + 1 + 1 /* compression_methods */
    + 2 + extsLen; /* extensions */

  // content type = handshake(22); legacy record version is always 3.1, a
  // real, well-known quirk even for a TLS 1.3 ClientHello (middlebox
  // compatibility -- the real version only ever appears in supported_versions).
  c.bytes(new Uint8Array([0x16, 0x03, 0x01, ...u16be(bodyLen + 4)]));
  c.bytes(new Uint8Array([0x01, (bodyLen >> 16) & 0xff, (bodyLen >> 8) & 0xff, bodyLen & 0xff])); // ClientHello(1) + 3-byte length
  c.bytes(new Uint8Array([0x03, 0x03])); // legacy_version: always TLS 1.2 on the wire
  c.tag('r', 32); // Random
  c.bytes(new Uint8Array([32]));
  c.tag('r', 32); // legacy session_id: real browsers send 32 random bytes (RFC 8446 Appendix D.4)
  c.bytes(new Uint8Array([...u16be(cipherLen), ...cipherSuites.flatMap((s) => u16be(s))]));
  c.bytes(new Uint8Array([1, 0])); // compression_methods: length=1, null
  c.bytes(new Uint8Array(u16be(extsLen)));
  for (const e of exts) e.emit(c);
  return c.toString();
}

function genChromeI1(host: string): string {
  const cipherGrease = randomGreaseValue();
  const groupGrease = randomGreaseValue();
  const ext1Grease = randomGreaseValue();
  const ext2Grease = randomGreaseValue();
  const versionGrease = randomGreaseValue();

  const cipherSuites = [
    cipherGrease,
    TLS_CIPHER.AES_128_GCM_SHA256, TLS_CIPHER.AES_256_GCM_SHA384, TLS_CIPHER.CHACHA20_POLY1305_SHA256,
    TLS_CIPHER.ECDHE_ECDSA_AES_128_GCM_SHA256, TLS_CIPHER.ECDHE_RSA_AES_128_GCM_SHA256,
    TLS_CIPHER.ECDHE_ECDSA_AES_256_GCM_SHA384, TLS_CIPHER.ECDHE_RSA_AES_256_GCM_SHA384,
    TLS_CIPHER.ECDHE_ECDSA_CHACHA20_POLY1305, TLS_CIPHER.ECDHE_RSA_CHACHA20_POLY1305,
    TLS_CIPHER.ECDHE_RSA_AES_128_CBC_SHA, TLS_CIPHER.ECDHE_RSA_AES_256_CBC_SHA,
    TLS_CIPHER.RSA_AES_128_GCM_SHA256, TLS_CIPHER.RSA_AES_256_GCM_SHA384,
    TLS_CIPHER.RSA_AES_128_CBC_SHA, TLS_CIPHER.RSA_AES_256_CBC_SHA,
  ];

  const exts: ExtPart[] = [
    litExt(ext1Grease, []),
    serverNameExt(host),
    emptyExt(TLS_EXT.extendedMasterSecret),
    renegotiationInfoExt(),
    u16ListExt2(TLS_EXT.supportedGroups, [groupGrease, NAMED_GROUP.x25519, NAMED_GROUP.secp256r1, NAMED_GROUP.secp384r1]),
    u8ListExt(TLS_EXT.ecPointFormats, [0]), // uncompressed
    emptyExt(TLS_EXT.sessionTicket),
    alpnExt(['h2', 'http/1.1']),
    statusRequestExt(),
    u16ListExt2(TLS_EXT.signatureAlgorithms, [
      SIG_ALG.ecdsa_secp256r1_sha256, SIG_ALG.rsa_pss_rsae_sha256, SIG_ALG.rsa_pkcs1_sha256,
      SIG_ALG.ecdsa_secp384r1_sha384, SIG_ALG.rsa_pss_rsae_sha384, SIG_ALG.rsa_pkcs1_sha384,
      SIG_ALG.rsa_pss_rsae_sha512, SIG_ALG.rsa_pkcs1_sha512,
    ]),
    emptyExt(TLS_EXT.sct),
    keyShareExt([{ group: groupGrease, greaseBody: [0] }, { group: NAMED_GROUP.x25519 }]),
    pskModesExt([1]), // psk_dhe_ke
    u16ListExt1(TLS_EXT.supportedVersions, [versionGrease, TLS13_VERSION, TLS12_VERSION]),
    compressCertExt([CERT_COMPRESSION_BROTLI]),
    alpsExt(['h2']),
    litExt(ext2Grease, [0]),
    emptyExt(TLS_EXT.padding),
  ];

  return buildTlsClientHello(cipherSuites, exts);
}

function genFirefoxI1(host: string): string {
  const cipherSuites = [
    TLS_CIPHER.AES_128_GCM_SHA256, TLS_CIPHER.CHACHA20_POLY1305_SHA256, TLS_CIPHER.AES_256_GCM_SHA384,
    TLS_CIPHER.ECDHE_ECDSA_AES_128_GCM_SHA256, TLS_CIPHER.ECDHE_RSA_AES_128_GCM_SHA256,
    TLS_CIPHER.ECDHE_ECDSA_CHACHA20_POLY1305, TLS_CIPHER.ECDHE_RSA_CHACHA20_POLY1305,
    TLS_CIPHER.ECDHE_ECDSA_AES_256_GCM_SHA384, TLS_CIPHER.ECDHE_RSA_AES_256_GCM_SHA384,
    TLS_CIPHER.ECDHE_ECDSA_AES_256_CBC_SHA, TLS_CIPHER.ECDHE_ECDSA_AES_128_CBC_SHA,
    TLS_CIPHER.ECDHE_RSA_AES_128_CBC_SHA, TLS_CIPHER.ECDHE_RSA_AES_256_CBC_SHA,
    TLS_CIPHER.RSA_AES_128_GCM_SHA256, TLS_CIPHER.RSA_AES_256_GCM_SHA384,
    TLS_CIPHER.RSA_AES_128_CBC_SHA, TLS_CIPHER.RSA_AES_256_CBC_SHA,
  ];

  const exts: ExtPart[] = [
    serverNameExt(host),
    emptyExt(TLS_EXT.extendedMasterSecret),
    renegotiationInfoExt(),
    u16ListExt2(TLS_EXT.supportedGroups, [
      NAMED_GROUP.x25519, NAMED_GROUP.secp256r1, NAMED_GROUP.secp384r1, NAMED_GROUP.secp521r1,
      NAMED_GROUP.ffdhe2048, NAMED_GROUP.ffdhe3072,
    ]),
    u8ListExt(TLS_EXT.ecPointFormats, [0]),
    emptyExt(TLS_EXT.sessionTicket),
    alpnExt(['h2', 'http/1.1']),
    statusRequestExt(),
    u16ListExt2(TLS_EXT.delegatedCredentials, [
      SIG_ALG.ecdsa_secp256r1_sha256, SIG_ALG.ecdsa_secp384r1_sha384, SIG_ALG.ecdsa_secp521r1_sha512, SIG_ALG.ecdsa_sha1,
    ]),
    keyShareExt([{ group: NAMED_GROUP.x25519 }, { group: NAMED_GROUP.secp256r1 }]),
    u16ListExt1(TLS_EXT.supportedVersions, [TLS13_VERSION, TLS12_VERSION]),
    u16ListExt2(TLS_EXT.signatureAlgorithms, [
      SIG_ALG.ecdsa_secp256r1_sha256, SIG_ALG.ecdsa_secp384r1_sha384, SIG_ALG.ecdsa_secp521r1_sha512,
      SIG_ALG.rsa_pss_rsae_sha256, SIG_ALG.rsa_pss_rsae_sha384, SIG_ALG.rsa_pss_rsae_sha512,
      SIG_ALG.rsa_pkcs1_sha256, SIG_ALG.rsa_pkcs1_sha384, SIG_ALG.rsa_pkcs1_sha512,
      SIG_ALG.ecdsa_sha1, SIG_ALG.rsa_pkcs1_sha1,
    ]),
    pskModesExt([1]),
    recordSizeLimitExt(16385),
    emptyExt(TLS_EXT.padding),
  ];

  return buildTlsClientHello(cipherSuites, exts);
}

function genSafariI1(host: string): string {
  const cipherGrease = randomGreaseValue();
  const groupGrease = randomGreaseValue();
  const ext1Grease = randomGreaseValue();
  const ext2Grease = randomGreaseValue();
  const versionGrease = randomGreaseValue();

  const cipherSuites = [
    cipherGrease,
    TLS_CIPHER.AES_128_GCM_SHA256, TLS_CIPHER.AES_256_GCM_SHA384, TLS_CIPHER.CHACHA20_POLY1305_SHA256,
    TLS_CIPHER.ECDHE_ECDSA_AES_256_GCM_SHA384, TLS_CIPHER.ECDHE_ECDSA_AES_128_GCM_SHA256, TLS_CIPHER.ECDHE_ECDSA_CHACHA20_POLY1305,
    TLS_CIPHER.ECDHE_RSA_AES_256_GCM_SHA384, TLS_CIPHER.ECDHE_RSA_AES_128_GCM_SHA256, TLS_CIPHER.ECDHE_RSA_CHACHA20_POLY1305,
    TLS_CIPHER.ECDHE_ECDSA_AES_256_CBC_SHA384, TLS_CIPHER.ECDHE_ECDSA_AES_128_CBC_SHA256,
    TLS_CIPHER.ECDHE_ECDSA_AES_256_CBC_SHA, TLS_CIPHER.ECDHE_ECDSA_AES_128_CBC_SHA,
    TLS_CIPHER.ECDHE_RSA_AES_256_CBC_SHA384, TLS_CIPHER.ECDHE_RSA_AES_128_CBC_SHA256,
    TLS_CIPHER.ECDHE_RSA_AES_256_CBC_SHA, TLS_CIPHER.ECDHE_RSA_AES_128_CBC_SHA,
    TLS_CIPHER.RSA_AES_256_GCM_SHA384, TLS_CIPHER.RSA_AES_128_GCM_SHA256,
    TLS_CIPHER.RSA_AES_256_CBC_SHA256, TLS_CIPHER.RSA_AES_128_CBC_SHA256,
    TLS_CIPHER.RSA_AES_256_CBC_SHA, TLS_CIPHER.RSA_AES_128_CBC_SHA,
    TLS_CIPHER.ECDHE_ECDSA_3DES_EDE_CBC_SHA, TLS_CIPHER.ECDHE_RSA_3DES_EDE_CBC_SHA,
    TLS_CIPHER.RSA_3DES_EDE_CBC_SHA,
  ];

  const exts: ExtPart[] = [
    litExt(ext1Grease, []),
    serverNameExt(host),
    emptyExt(TLS_EXT.extendedMasterSecret),
    renegotiationInfoExt(),
    u16ListExt2(TLS_EXT.supportedGroups, [
      groupGrease, NAMED_GROUP.x25519, NAMED_GROUP.secp256r1, NAMED_GROUP.secp384r1, NAMED_GROUP.secp521r1,
    ]),
    u8ListExt(TLS_EXT.ecPointFormats, [0]),
    alpnExt(['h2', 'http/1.1']),
    statusRequestExt(),
    // Real iOS14 capture lists rsa_pss_rsae_sha384 twice in a row -- kept
    // exactly as captured (see this section's own doc comment on why).
    u16ListExt2(TLS_EXT.signatureAlgorithms, [
      SIG_ALG.ecdsa_secp256r1_sha256, SIG_ALG.rsa_pss_rsae_sha256, SIG_ALG.rsa_pkcs1_sha256,
      SIG_ALG.ecdsa_secp384r1_sha384, SIG_ALG.ecdsa_sha1, SIG_ALG.rsa_pss_rsae_sha384, SIG_ALG.rsa_pss_rsae_sha384,
      SIG_ALG.rsa_pkcs1_sha384, SIG_ALG.rsa_pss_rsae_sha512, SIG_ALG.rsa_pkcs1_sha512, SIG_ALG.rsa_pkcs1_sha1,
    ]),
    emptyExt(TLS_EXT.sct),
    keyShareExt([{ group: groupGrease, greaseBody: [0] }, { group: NAMED_GROUP.x25519 }]),
    pskModesExt([1]),
    u16ListExt1(TLS_EXT.supportedVersions, [versionGrease, TLS13_VERSION, TLS12_VERSION, TLS11_VERSION, TLS10_VERSION]),
    litExt(ext2Grease, [0]),
    emptyExt(TLS_EXT.padding),
  ];

  return buildTlsClientHello(cipherSuites, exts);
}

export interface I1GenResult {
  chain: string;
  label: string;
}

// Generates an I1 CPS chain for the given profile (or a uniformly random
// one when profileChoice is 'random' -- quic is excluded from that pool
// when crypto.subtle isn't available, so 'random' always succeeds), and a
// human-readable "profile(host)" label so a 'random' pick is visible to
// the admin. host defaults to a uniformly random pick from I1_HOSTS.
// Returns null when generation can't be fulfilled: a malformed host (see
// dnsName -- not reachable with the built-in host pool) or an explicit
// 'quic' choice without crypto.subtle (see isQuicI1Supported; the caller
// should check this before offering 'quic' at all, so this is a backstop,
// not the primary UX).
export async function genI1(profileChoice: I1ProfileChoice, host = ''): Promise<I1GenResult | null> {
  const quicOk = isQuicI1Supported();
  const profiles: I1Profile[] = quicOk
    ? ['dns', 'quic', 'sip', 'stun', 'chrome', 'firefox', 'safari']
    : ['dns', 'sip', 'stun', 'chrome', 'firefox', 'safari'];
  const profile: I1Profile = profileChoice === 'random'
    ? profiles[randRange(0, profiles.length - 1)]
    : profileChoice;
  if (profile === 'quic' && !quicOk) return null;
  const resolvedHost = host || I1_HOSTS[randRange(0, I1_HOSTS.length - 1)];

  let chain: string | null;
  switch (profile) {
    case 'dns':
      chain = genDnsI1(resolvedHost);
      break;
    case 'quic':
      chain = await genQuicI1(resolvedHost);
      break;
    case 'sip':
      chain = genSipI1(resolvedHost);
      break;
    case 'stun':
      chain = genStunI1();
      break;
    case 'chrome':
      chain = genChromeI1(resolvedHost);
      break;
    case 'firefox':
      chain = genFirefoxI1(resolvedHost);
      break;
    case 'safari':
      chain = genSafariI1(resolvedHost);
      break;
  }
  if (chain == null) return null;
  return { chain, label: `${profile}(${resolvedHost})` };
}
