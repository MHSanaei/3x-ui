import { describe, expect, it } from 'vitest';

import { PRESETS } from '@/pages/xray/dns/DnsPresetsModal';
import { DnsObjectSchema, DnsServerObjectSchema } from '@/schemas/dns';

describe('DNS presets', () => {
  it('include encrypted presets accepted by xray DNS config', () => {
    const servers = PRESETS.flatMap((preset) => preset.data);

    expect(servers).toContain('https://freedns.controld.com/p2');
    expect(servers).toContain('quic+local://p2.freedns.controld.com');
    expect(servers).toContain('https://dns.google/dns-query');
    expect(servers.every((server) => !server.startsWith('tls://'))).toBe(true);
    expect(DnsObjectSchema.parse({ servers }).servers).toEqual(servers);
    for (const server of servers) {
      expect(DnsServerObjectSchema.parse({ address: server }).address).toBe(server);
    }
  });
});
