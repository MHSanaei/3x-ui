import { describe, it, expect } from 'vitest';

import { parseLinkParts, linkMetaText } from '@/lib/xray/link-label';

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
});
