// AmneziaWG I1 (CPS signature packet) generators -- ported from the
// algorithms in vernette/warpscout's i1gen.go (MIT), not the code itself.
// I1's whole point is DPI plausibility, not entropy: a chain that looks
// like a real DNS/STUN/SIP packet (randomness only where real traffic of
// that kind would actually have it -- transaction IDs, nonces, padding) is
// far harder to fingerprint than a blob of pure random bytes. QUIC (a 4th
// profile in the reference implementation) needs real AES-GCM/HKDF/header-
// protection crypto and is deliberately not ported here -- see the plan's
// own Phase 3.8 section for why it's a separate follow-up.

const DNS_LABEL_MAX = 63;
const DNS_PAD_MIN = 60;
const DNS_PAD_MAX = 180;
const STUN_SOFTWARE_MIN = 16;
const STUN_SOFTWARE_MAX = 32;

const I1_HOSTS = ['www.apple.com', 'www.google.com', 'www.microsoft.com', 'cdn.jsdelivr.net'];

export type I1Profile = 'dns' | 'sip' | 'stun';
export type I1ProfileChoice = I1Profile | 'random';

export const I1_PROFILE_CHOICES: I1ProfileChoice[] = ['random', 'dns', 'sip', 'stun'];

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

export interface I1GenResult {
  chain: string;
  label: string;
}

// Generates an I1 CPS chain for the given profile (or a uniformly random
// one among dns/sip/stun when profileChoice is 'random'), and a
// human-readable "profile(host)" label so a 'random' pick is visible to
// the admin. host defaults to a uniformly random pick from I1_HOSTS.
// Returns null only on a malformed host (see dnsName) -- not reachable
// with the built-in host pool, only relevant if a host is ever passed in.
export function genI1(profileChoice: I1ProfileChoice, host = ''): I1GenResult | null {
  const profiles: I1Profile[] = ['dns', 'sip', 'stun'];
  const profile: I1Profile = profileChoice === 'random'
    ? profiles[randRange(0, profiles.length - 1)]
    : profileChoice;
  const resolvedHost = host || I1_HOSTS[randRange(0, I1_HOSTS.length - 1)];

  let chain: string | null;
  switch (profile) {
    case 'dns':
      chain = genDnsI1(resolvedHost);
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
