// AmneziaWG I1 (CPS signature packet) generators -- ported from the
// algorithms in vernette/warpscout's i1gen.go/quic.go (MIT), not the code
// itself. I1's whole point is DPI plausibility, not entropy: a chain that
// looks like a real DNS/STUN/SIP/QUIC packet (randomness only where real
// traffic of that kind would actually have it -- transaction IDs, nonces,
// padding, TLS Random, AEAD tags) is far harder to fingerprint than a blob
// of pure random bytes.

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

export type I1Profile = 'dns' | 'quic' | 'sip' | 'stun';
export type I1ProfileChoice = I1Profile | 'random';

export const I1_PROFILE_CHOICES: I1ProfileChoice[] = ['random', 'dns', 'quic', 'sip', 'stun'];

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
  const profiles: I1Profile[] = quicOk ? ['dns', 'quic', 'sip', 'stun'] : ['dns', 'sip', 'stun'];
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
  }
  if (chain == null) return null;
  return { chain, label: `${profile}(${resolvedHost})` };
}
