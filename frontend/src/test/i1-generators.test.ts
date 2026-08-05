import { describe, expect, it } from 'vitest';

import { genI1, I1_PROFILE_CHOICES } from '@/lib/xray/i1Generators';

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

describe('I1_PROFILE_CHOICES', () => {
  it('lists random plus the three implemented profiles, random first', () => {
    expect(I1_PROFILE_CHOICES).toEqual(['random', 'dns', 'sip', 'stun']);
  });
});

describe('genI1 — dns profile', () => {
  it('produces a well-formed chain: <r 2> + 3 literal blocks + a padding <r N> in range', () => {
    const result = genI1('dns', 'example.com');
    expect(result).not.toBeNull();
    expect(result!.label).toBe('dns(example.com)');
    expect(result!.chain).toMatch(/^<r 2><b 0x[0-9a-f]+><b 0x[0-9a-f]+><b 0x[0-9a-f]+><r \d+>$/);

    const padMatch = result!.chain.match(/<r (\d+)>$/);
    const padLen = Number(padMatch![1]);
    expect(padLen).toBeGreaterThanOrEqual(60);
    expect(padLen).toBeLessThanOrEqual(180);
  });

  it('encodes the fixed DNS header (flags/QD/AN/NS/ARCOUNT) as the first literal block', () => {
    const result = genI1('dns', 'example.com');
    const blocks = [...result!.chain.matchAll(/<b 0x([0-9a-f]+)>/g)].map((m) => m[1]);
    expect(blocks[0]).toBe('01000001000000000001');
  });

  it('encodes QNAME + QTYPE/QCLASS as a length matching the host labels', () => {
    // "example"(7) + "com"(3) as length-prefixed labels + terminator = 13
    // bytes, plus 4 bytes of QTYPE=A/QCLASS=IN = 17 bytes = 34 hex chars.
    const result = genI1('dns', 'example.com');
    const lengths = bBlockHexLengths(result!.chain);
    expect(lengths[1]).toBe(34);
  });

  it('encodes a fixed-size OPT pseudo-record (15 bytes) as the third literal block regardless of padLen', () => {
    const result = genI1('dns', 'example.com');
    const lengths = bBlockHexLengths(result!.chain);
    expect(lengths[2]).toBe(30); // 15 bytes = 30 hex chars
  });

  it('returns null for a host with a label over the 63-byte DNS limit', () => {
    const tooLong = `${'a'.repeat(64)}.com`;
    expect(genI1('dns', tooLong)).toBeNull();
  });

  it('picks a default host when none is given', () => {
    const result = genI1('dns');
    expect(result).not.toBeNull();
    expect(result!.label).toMatch(/^dns\(.+\..+\)$/);
  });
});

describe('genI1 — stun profile', () => {
  it('produces a well-formed chain: header block, <r 12>, attribute-header block, <rc N>', () => {
    const result = genI1('stun', 'example.com');
    expect(result).not.toBeNull();
    expect(result!.label).toBe('stun(example.com)');
    expect(result!.chain).toMatch(/^<b 0x[0-9a-f]+><r 12><b 0x[0-9a-f]+><rc \d+>$/);
  });

  it('encodes Binding Request type and the magic cookie in the header block', () => {
    const result = genI1('stun', 'example.com');
    const blocks = [...result!.chain.matchAll(/<b 0x([0-9a-f]+)>/g)].map((m) => m[1]);
    expect(blocks[0].startsWith('0001')).toBe(true); // Binding Request
    expect(blocks[0].endsWith('2112a442')).toBe(true); // magic cookie
  });

  it("SOFTWARE attribute length is a multiple of 4 in [16, 32] and matches the <rc N> tag", () => {
    const result = genI1('stun', 'example.com');
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
  it('produces a valid SIP OPTIONS request with the expected literal fragments', () => {
    const result = genI1('sip', 'example.com');
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

  it('randomizes exactly the fields a real client would vary (Via branch, From tag, Call-ID, CSeq)', () => {
    const result = genI1('sip', 'example.com');
    const tags = [...result!.chain.matchAll(/<(rc|rd) (\d+)>/g)].map((m) => `${m[1]} ${m[2]}`);
    expect(tags).toEqual(['rc 10', 'rc 8', 'rd 6', 'rc 16', 'rd 4']);
  });
});

describe('genI1 — random meta-profile', () => {
  it('always resolves to one of dns/sip/stun and never throws, across many draws', () => {
    for (let i = 0; i < 30; i++) {
      const result = genI1('random');
      expect(result).not.toBeNull();
      const profile = result!.label.split('(')[0];
      expect(['dns', 'sip', 'stun']).toContain(profile);
    }
  });
});
