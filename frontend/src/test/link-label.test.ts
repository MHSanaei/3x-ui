import { describe, it, expect } from 'vitest';

import { parseLinkParts, linkMetaText } from '@/lib/xray/link-label';
import { genAmneziaWGLink } from '@/lib/xray/inbound-link';
import type { AmneziawgInboundSettings } from '@/schemas/protocols/inbound/amneziawg';

// The panel shows the subscription's remark verbatim. Per-client traffic/expiry
// info is rendered only into the body a client app imports (backend, first link
// only), so the panel's display links are already clean — nothing is stripped.
describe('link-label parseLinkParts', () => {
  const linkWith = (remark: string) =>
    `vless://uid@host.example.com:443?type=tcp&security=tls#${encodeURIComponent(remark)}`;

  it('parses protocol / network / security and keeps the remark verbatim', () => {
    const parts = parseLinkParts(linkWith('Germany-john@example.com'));
    expect(parts?.protocol).toBe('Vless');
    expect(parts?.network).toBe('TCP');
    expect(parts?.security).toBe('TLS');
    expect(parts?.remark).toBe('Germany-john@example.com');
    expect(parts?.port).toBe('443');
  });

  it('linkMetaText joins the remark with the port', () => {
    const parts = parseLinkParts(linkWith('Germany-john@example.com'));
    expect(parts && linkMetaText(parts)).toBe('Germany-john@example.com:443');
  });

  it('returns null for an unparseable scheme', () => {
    expect(parseLinkParts('not-a-link')).toBeNull();
  });

  // MTProto share links are tg://proxy deep links whose port rides in a query
  // param, not the URL authority; they carry no transport and use FakeTLS.
  it('labels an mtproto tg://proxy link with its query-param port and FakeTLS', () => {
    const parts = parseLinkParts(
      'tg://proxy?server=host.example.com&port=8443&secret=ee00#mt-inbound',
    );
    expect(parts?.protocol).toBe('MTProto');
    expect(parts?.network).toBe('');
    expect(parts?.security).toBe('FAKETLS');
    expect(parts?.port).toBe('8443');
    expect(parts?.remark).toBe('mt-inbound');
    expect(parts && linkMetaText(parts)).toBe('mt-inbound:8443');
  });

  // AmneziaWG's vpn:// links are base64url of a plain .conf text, not a
  // structured URL (see inbound-link.ts's genAmneziaWGLink) -- there's no
  // query string or #hash available, so the remark/port have to be read back
  // out of the decoded .conf body instead. Regression test for a real report:
  // these links were showing a generic "Vpn" tag and falling back to "Link N"
  // instead of "AmneziaWG" + the actual remark:port, unlike every other
  // protocol's link row.
  it('labels an AmneziaWG vpn:// link with its decoded remark and endpoint port', () => {
    const settings = {
      server: {
        publicKey: 'serverPubKey==',
        jc: 5,
        jmin: 10,
        jmax: 50,
        s1: 30,
        s2: 45,
        s3: 10,
        s4: 5,
        h1: '',
        h2: '',
        h3: '',
        h4: '',
        i1: '',
      },
      clients: [{ email: 'peer-1', privateKey: 'clientPrivKey==', allowedIPs: ['10.8.1.2/32'] }],
    } as unknown as AmneziawgInboundSettings;

    // Cyrillic remark on purpose -- matches the real report, and exercises
    // the unicode round-trip through base64url (not just plain ASCII).
    const link = genAmneziaWGLink({
      settings,
      address: 'awg.example.test',
      port: 36541,
      remark: 'wg-Майфун',
      peerIndex: 0,
    });

    const parts = parseLinkParts(link);
    expect(parts?.protocol).toBe('AmneziaWG');
    expect(parts?.remark).toBe('wg-Майфун');
    expect(parts?.port).toBe('36541');
    expect(parts && linkMetaText(parts)).toBe('wg-Майфун:36541');
  });
});
