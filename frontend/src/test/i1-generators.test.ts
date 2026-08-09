import { afterEach, describe, expect, it, vi } from 'vitest';

import { genI1, I1_PROFILE_CHOICES, isQuicI1Supported } from '@/lib/xray/i1Generators';

function hexToBytes(hex: string): Uint8Array {
  const bytes = new Uint8Array(hex.length / 2);
  for (let i = 0; i < bytes.length; i++) {
    bytes[i] = parseInt(hex.slice(i * 2, i * 2 + 2), 16);
  }
  return bytes;
}

// Decodes every <b 0xHEX> block back to UTF-8 text (fine for the SIP
// profile, whose literal bytes are all ASCII) and leaves <tag N> markers
// as-is, so the reconstructed string can be asserted against directly.
function decodeChainAsText(chain: string): string {
  return chain.replace(/<b 0x([0-9a-f]+)>/g, (_m, hex: string) => new TextDecoder().decode(hexToBytes(hex)));
}

function bBlockHexLengths(chain: string): number[] {
  return [...chain.matchAll(/<b 0x([0-9a-f]+)>/g)].map((m) => m[1].length);
}

type ChainToken = { kind: 'bytes'; hex: string } | { kind: 'tag'; name: string; n: number };

function tokenize(chain: string): ChainToken[] {
  const tokens: ChainToken[] = [];
  const re = /<b 0x([0-9a-f]+)>|<(\w+) (\d+)>/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(chain)) !== null) {
    if (m[1] !== undefined) tokens.push({ kind: 'bytes', hex: m[1] });
    else tokens.push({ kind: 'tag', name: m[2], n: Number(m[3]) });
  }
  return tokens;
}

function tokenByteLen(t: ChainToken): number {
  return t.kind === 'bytes' ? t.hex.length / 2 : t.n;
}

function sumByteLen(tokens: ChainToken[]): number {
  return tokens.reduce((s, t) => s + tokenByteLen(t), 0);
}

function isGreaseHex(hex4: string): boolean {
  return /^([0-9a-f]a)\1$/.test(hex4);
}

// Shared by the 3 browser-fingerprint profiles below (they all go through
// the same buildTlsClientHello wrapper): asserts the record/handshake/
// extensions length *fields* actually match what follows them byte-for-byte
// -- a real self-consistency check, not just "looks plausible" -- using
// each <tag N> token's own declared size for the bytes hidden behind it.
// Returns the negotiated cipher count/first-cipher hex and the raw
// extension tokens so each profile's own test can do its distinguishing
// spot-checks (GREASE presence, extension order, exact cipher count).
function expectValidClientHello(chain: string): { cipherCount: number; firstCipherHex: string; extTokens: ChainToken[] } {
  const tokens = tokenize(chain);

  const record = tokens[0];
  if (record.kind !== 'bytes') throw new Error('expected record header');
  expect(record.hex.slice(0, 6)).toBe('160301'); // handshake(22), legacy record version 3.1
  expect(parseInt(record.hex.slice(6, 10), 16)).toBe(sumByteLen(tokens.slice(1)));

  const handshake = tokens[1];
  if (handshake.kind !== 'bytes') throw new Error('expected handshake header');
  expect(handshake.hex.slice(0, 2)).toBe('01'); // ClientHello
  expect(parseInt(handshake.hex.slice(2, 8), 16)).toBe(sumByteLen(tokens.slice(2)));

  expect(tokens[2]).toEqual({ kind: 'bytes', hex: '0303' }); // legacy_version: TLS 1.2 on the wire
  expect(tokens[3]).toEqual({ kind: 'tag', name: 'r', n: 32 }); // Random
  expect(tokens[4]).toEqual({ kind: 'bytes', hex: '20' }); // session_id length = 32
  expect(tokens[5]).toEqual({ kind: 'tag', name: 'r', n: 32 }); // session_id

  const cipherBlock = tokens[6];
  if (cipherBlock.kind !== 'bytes') throw new Error('expected cipher_suites block');
  const cipherDeclaredLen = parseInt(cipherBlock.hex.slice(0, 4), 16);
  expect(cipherBlock.hex.length / 2 - 2).toBe(cipherDeclaredLen);
  expect(cipherDeclaredLen % 2).toBe(0);

  expect(tokens[7]).toEqual({ kind: 'bytes', hex: '0100' }); // compression_methods: len=1, null method

  const extLenPrefix = tokens[8];
  if (extLenPrefix.kind !== 'bytes') throw new Error('expected extensions length prefix');
  const extTokens = tokens.slice(9);
  expect(parseInt(extLenPrefix.hex, 16)).toBe(sumByteLen(extTokens));

  return { cipherCount: cipherDeclaredLen / 2, firstCipherHex: cipherBlock.hex.slice(4, 8), extTokens };
}

describe('I1_PROFILE_CHOICES', () => {
  it('lists random plus the seven implemented profiles, random first', () => {
    expect(I1_PROFILE_CHOICES).toEqual(['random', 'dns', 'quic', 'sip', 'stun', 'chrome', 'firefox', 'safari']);
  });
});

describe('genI1 — dns profile', () => {
  it('produces a well-formed chain: <r 2> + 3 literal blocks + a padding <r N> in range', async () => {
    const result = await genI1('dns', 'example.com');
    expect(result).not.toBeNull();
    expect(result!.label).toBe('dns(example.com)');
    expect(result!.chain).toMatch(/^<r 2><b 0x[0-9a-f]+><b 0x[0-9a-f]+><b 0x[0-9a-f]+><r \d+>$/);

    const padMatch = result!.chain.match(/<r (\d+)>$/);
    const padLen = Number(padMatch![1]);
    expect(padLen).toBeGreaterThanOrEqual(60);
    expect(padLen).toBeLessThanOrEqual(180);
  });

  it('encodes the fixed DNS header (flags/QD/AN/NS/ARCOUNT) as the first literal block', async () => {
    const result = await genI1('dns', 'example.com');
    const blocks = [...result!.chain.matchAll(/<b 0x([0-9a-f]+)>/g)].map((m) => m[1]);
    expect(blocks[0]).toBe('01000001000000000001');
  });

  it('encodes QNAME + QTYPE/QCLASS as a length matching the host labels', async () => {
    // "example"(7) + "com"(3) as length-prefixed labels + terminator = 13
    // bytes, plus 4 bytes of QTYPE=A/QCLASS=IN = 17 bytes = 34 hex chars.
    const result = await genI1('dns', 'example.com');
    const lengths = bBlockHexLengths(result!.chain);
    expect(lengths[1]).toBe(34);
  });

  it('encodes a fixed-size OPT pseudo-record (15 bytes) as the third literal block regardless of padLen', async () => {
    const result = await genI1('dns', 'example.com');
    const lengths = bBlockHexLengths(result!.chain);
    expect(lengths[2]).toBe(30); // 15 bytes = 30 hex chars
  });

  it('returns null for a host with a label over the 63-byte DNS limit', async () => {
    const tooLong = `${'a'.repeat(64)}.com`;
    expect(await genI1('dns', tooLong)).toBeNull();
  });

  it('picks a default host when none is given', async () => {
    const result = await genI1('dns');
    expect(result).not.toBeNull();
    expect(result!.label).toMatch(/^dns\(.+\..+\)$/);
  });
});

describe('genI1 — stun profile', () => {
  it('produces a well-formed chain: header block, <r 12>, attribute-header block, <rc N>', async () => {
    const result = await genI1('stun', 'example.com');
    expect(result).not.toBeNull();
    expect(result!.label).toBe('stun(example.com)');
    expect(result!.chain).toMatch(/^<b 0x[0-9a-f]+><r 12><b 0x[0-9a-f]+><rc \d+>$/);
  });

  it('encodes Binding Request type and the magic cookie in the header block', async () => {
    const result = await genI1('stun', 'example.com');
    const blocks = [...result!.chain.matchAll(/<b 0x([0-9a-f]+)>/g)].map((m) => m[1]);
    expect(blocks[0].startsWith('0001')).toBe(true); // Binding Request
    expect(blocks[0].endsWith('2112a442')).toBe(true); // magic cookie
  });

  it("SOFTWARE attribute length is a multiple of 4 in [16, 32] and matches the <rc N> tag", async () => {
    const result = await genI1('stun', 'example.com');
    const rcMatch = result!.chain.match(/<rc (\d+)>$/);
    const softwareLen = Number(rcMatch![1]);
    expect(softwareLen).toBeGreaterThanOrEqual(16);
    expect(softwareLen).toBeLessThanOrEqual(32);
    expect(softwareLen % 4).toBe(0);

    const blocks = [...result!.chain.matchAll(/<b 0x([0-9a-f]+)>/g)].map((m) => m[1]);
    expect(blocks[1]).toBe(`8022${softwareLen.toString(16).padStart(4, '0')}`);
  });
});

describe('genI1 — sip profile', () => {
  it('produces a valid SIP OPTIONS request with the expected literal fragments', async () => {
    const result = await genI1('sip', 'example.com');
    expect(result).not.toBeNull();
    expect(result!.label).toBe('sip(example.com)');

    const text = decodeChainAsText(result!.chain);
    expect(text).toContain('OPTIONS sip:example.com SIP/2.0\r\n');
    expect(text).toContain('Via: SIP/2.0/UDP 192.168.');
    expect(text).toContain(':5060;branch=z9hG4bK');
    expect(text).toContain('\r\nFrom: <sip:');
    expect(text).toContain('@example.com>;tag=');
    expect(text).toContain('\r\nTo: <sip:example.com>\r\nCall-ID: ');
    expect(text).toContain('@example.com\r\nCSeq: ');
    expect(text).toContain(' OPTIONS\r\nMax-Forwards: 70\r\nUser-Agent: PJSIP/2.13\r\nContent-Length: 0\r\n\r\n');
  });

  it('randomizes exactly the fields a real client would vary (Via branch, From tag, Call-ID, CSeq)', async () => {
    const result = await genI1('sip', 'example.com');
    const tags = [...result!.chain.matchAll(/<(rc|rd) (\d+)>/g)].map((m) => `${m[1]} ${m[2]}`);
    expect(tags).toEqual(['rc 10', 'rc 8', 'rd 6', 'rc 16', 'rd 4']);
  });
});

// This profile's crypto pipeline (HKDF-derive, AES-GCM seal, AES-ECB
// header-protection mask) was cross-checked byte-for-byte against a
// fixed-test-vector port of the real upstream Go source before shipping;
// these tests assert the structural invariants a regression would break,
// not re-derive correctness from scratch.
describe('genI1 — quic profile', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('produces a well-formed QUIC Initial packet chain: header block, <r N>, rest-of-hello block, <r 16> tag', async () => {
    const result = await genI1('quic', 'example.com');
    expect(result).not.toBeNull();
    expect(result!.label).toBe('quic(example.com)');
    expect(result!.chain).toMatch(/^<b 0x[0-9a-f]+><r \d+><b 0x[0-9a-f]+><r 16>$/);
  });

  it('keeps the long-header type bits and QUIC v1 version literal regardless of the random DCID', async () => {
    const result = await genI1('quic', 'example.com');
    const blocks = [...result!.chain.matchAll(/<b 0x([0-9a-f]+)>/g)].map((m) => m[1]);
    // Header protection only XORs the low nibble of the first byte (RFC
    // 9001 S5.4.1), so the long-header/fixed-bit/Initial-type top nibble
    // (0xc) survives masking unconditionally.
    expect(blocks[0][0]).toBe('c');
    expect(blocks[0].slice(2, 10)).toBe('00000001'); // QUIC version 1
    expect(blocks[0].slice(10, 12)).toBe('01'); // DCID length = 1
    expect(blocks[0].slice(14, 18)).toBe('0000'); // SCID length=0, Token length varint=0
  });

  it('produces a different chain on every call (fresh DCID + TLS Random each time)', async () => {
    const a = await genI1('quic', 'example.com');
    const b = await genI1('quic', 'example.com');
    expect(a!.chain).not.toBe(b!.chain);
  });

  it('isQuicI1Supported reflects a real crypto.subtle-capable environment', () => {
    expect(isQuicI1Supported()).toBe(true);
  });

  it('returns null for an explicit quic choice when crypto.subtle is unavailable', async () => {
    vi.stubGlobal('crypto', {});
    expect(isQuicI1Supported()).toBe(false);
    expect(await genI1('quic', 'example.com')).toBeNull();
  });
});

// The byte layouts below (extension order/ids, cipher lists, GREASE
// placement) were cross-checked against refraction-networking/utls's own
// real captured-ClientHello test fixtures (ClientHello-JSON-{Chrome102,
// Firefox105,iOS14}.json), not the code itself. These tests assert the
// wire format is self-consistent (via expectValidClientHello) plus the
// handful of details that actually distinguish one browser from another,
// not a full byte-for-byte re-derivation.
describe('genI1 — chrome profile', () => {
  it('produces a self-consistent ClientHello: 16 cipher suites, first one GREASE', async () => {
    const result = await genI1('chrome', 'chrome-test.example');
    expect(result).not.toBeNull();
    expect(result!.label).toBe('chrome(chrome-test.example)');
    const { cipherCount, firstCipherHex } = expectValidClientHello(result!.chain);
    expect(cipherCount).toBe(16);
    expect(isGreaseHex(firstCipherHex)).toBe(true);
  });

  it('opens on a GREASE extension marker and closes on padding, with 3 32-byte random tags', async () => {
    const result = await genI1('chrome', 'chrome-test.example');
    const { extTokens } = expectValidClientHello(result!.chain);
    const first = extTokens[0];
    if (first.kind !== 'bytes') throw new Error('expected bytes');
    expect(first.hex.length).toBe(8); // empty-body GREASE extension: type(2) + length=0(2)
    expect(isGreaseHex(first.hex.slice(0, 4))).toBe(true);
    expect(result!.chain.endsWith('<b 0x00150000>')).toBe(true); // padding: type=21, length=0
    expect(result!.chain.match(/<r 32>/g)?.length).toBe(3); // Random, session_id, key_share x25519
  });

  it('embeds the real hostname in the server_name extension', async () => {
    const result = await genI1('chrome', 'chrome-test.example');
    expect(decodeChainAsText(result!.chain)).toContain('chrome-test.example');
  });
});

describe('genI1 — firefox profile', () => {
  it('produces a self-consistent ClientHello: 17 cipher suites, none GREASE (Firefox sends none)', async () => {
    const result = await genI1('firefox', 'firefox-test.example');
    expect(result).not.toBeNull();
    expect(result!.label).toBe('firefox(firefox-test.example)');
    const { cipherCount, firstCipherHex } = expectValidClientHello(result!.chain);
    expect(cipherCount).toBe(17);
    expect(firstCipherHex).toBe('1301'); // TLS_AES_128_GCM_SHA256, no GREASE prepended
  });

  it('opens on server_name (no GREASE) and closes on padding, with 4 32-byte random tags', async () => {
    const result = await genI1('firefox', 'firefox-test.example');
    const { extTokens } = expectValidClientHello(result!.chain);
    const first = extTokens[0];
    if (first.kind !== 'bytes') throw new Error('expected bytes');
    expect(first.hex.startsWith('0000')).toBe(true); // server_name = extension type 0
    expect(result!.chain.endsWith('<b 0x00150000>')).toBe(true); // padding: type=21, length=0
    expect(result!.chain.match(/<r 32>/g)?.length).toBe(4); // Random, session_id, key_share x25519 + secp256r1
  });

  it('embeds the real hostname in the server_name extension', async () => {
    const result = await genI1('firefox', 'firefox-test.example');
    expect(decodeChainAsText(result!.chain)).toContain('firefox-test.example');
  });
});

describe('genI1 — safari profile', () => {
  it('produces a self-consistent ClientHello: 27 cipher suites, first one GREASE', async () => {
    const result = await genI1('safari', 'safari-test.example');
    expect(result).not.toBeNull();
    expect(result!.label).toBe('safari(safari-test.example)');
    const { cipherCount, firstCipherHex } = expectValidClientHello(result!.chain);
    expect(cipherCount).toBe(27);
    expect(isGreaseHex(firstCipherHex)).toBe(true);
  });

  it('opens on a GREASE extension marker and closes on padding, with 3 32-byte random tags', async () => {
    const result = await genI1('safari', 'safari-test.example');
    const { extTokens } = expectValidClientHello(result!.chain);
    const first = extTokens[0];
    if (first.kind !== 'bytes') throw new Error('expected bytes');
    expect(isGreaseHex(first.hex.slice(0, 4))).toBe(true);
    expect(result!.chain.endsWith('<b 0x00150000>')).toBe(true); // padding: type=21, length=0
    expect(result!.chain.match(/<r 32>/g)?.length).toBe(3); // Random, session_id, key_share x25519
  });

  it('has no session_ticket extension (unlike Chrome/Firefox) and duplicates rsa_pss_rsae_sha384, matching the real iOS14 capture', async () => {
    const result = await genI1('safari', 'safari-test.example');
    const { extTokens } = expectValidClientHello(result!.chain);
    // session_ticket = extension type 35 (0x0023); none of Safari's blocks should start with it.
    for (const t of extTokens) {
      if (t.kind === 'bytes') expect(t.hex.startsWith('0023')).toBe(false);
    }
    // signature_algorithms (type 13) carries rsa_pss_rsae_sha384 (0x0805) twice in a row.
    const sigAlgBlock = extTokens.find((t) => t.kind === 'bytes' && t.hex.startsWith('000d')) as { hex: string } | undefined;
    expect(sigAlgBlock).toBeDefined();
    expect(sigAlgBlock!.hex).toContain('08050805'); // rsa_pss_rsae_sha384 (0x0805) back-to-back
  });

  it('embeds the real hostname in the server_name extension', async () => {
    const result = await genI1('safari', 'safari-test.example');
    expect(decodeChainAsText(result!.chain)).toContain('safari-test.example');
  });
});

describe('genI1 — random meta-profile', () => {
  it('always resolves to one of the 7 implemented profiles and never throws, across many draws', async () => {
    for (let i = 0; i < 30; i++) {
      const result = await genI1('random');
      expect(result).not.toBeNull();
      const profile = result!.label.split('(')[0];
      expect(['dns', 'quic', 'sip', 'stun', 'chrome', 'firefox', 'safari']).toContain(profile);
    }
  });

  it('excludes quic from its own pool when crypto.subtle is unavailable, and still always succeeds', async () => {
    vi.stubGlobal('crypto', {});
    try {
      for (let i = 0; i < 15; i++) {
        const result = await genI1('random');
        expect(result).not.toBeNull();
        const profile = result!.label.split('(')[0];
        expect(['dns', 'sip', 'stun', 'chrome', 'firefox', 'safari']).toContain(profile);
      }
    } finally {
      vi.unstubAllGlobals();
    }
  });
});
