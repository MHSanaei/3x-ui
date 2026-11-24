import { describe, expect, it } from 'vitest';

import type { InboundOption } from '@/hooks/useClients';
import { allHostsExcluded, hostLinksClosed, withHostEndpoints } from '@/lib/hosts/host-link';
import { tunnelConfigEndpoints } from '@/pages/clients/tunnelEndpoints';
import { InboundSchema } from '@/schemas/api/inbound';
import type { HostRecord } from '@/schemas/api/host';

const PANEL = 'panel.example.com';

function hostRecord(overrides: Partial<HostRecord>): HostRecord {
  return {
    groupId: 'g',
    inboundIds: [7],
    hosts: ['edge.example.com:443'],
    remark: 'H',
    ...overrides,
  };
}

const tuicInbound = { id: 7, port: 8443, protocol: 'tuic', listen: '' } as unknown as InboundOption;

// Mirrors the backend's closed case: enabled hosts exist and every one of them
// is excluded from the format, so the inbound renders nothing there.
describe('allHostsExcluded', () => {
  it('is true when every enabled host of the inbound excludes the format', () => {
    const records = [
      hostRecord({ excludeFromSubTypes: ['raw'] }),
      hostRecord({ groupId: 'g2', excludeFromSubTypes: ['raw', 'clash'] }),
    ];
    expect(allHostsExcluded(records, 7, 'raw')).toBe(true);
  });

  it('is false while one enabled host still serves the format', () => {
    const records = [
      hostRecord({ excludeFromSubTypes: ['raw'] }),
      hostRecord({ groupId: 'g2', excludeFromSubTypes: ['clash'] }),
    ];
    expect(allHostsExcluded(records, 7, 'raw')).toBe(false);
  });

  it('is false with no hosts, only disabled hosts, or hosts of other inbounds', () => {
    expect(allHostsExcluded([], 7, 'raw')).toBe(false);
    expect(
      allHostsExcluded([hostRecord({ isDisabled: true, excludeFromSubTypes: ['raw'] })], 7, 'raw'),
    ).toBe(false);
    expect(
      allHostsExcluded([hostRecord({ inboundIds: [8], excludeFromSubTypes: ['raw'] })], 7, 'raw'),
    ).toBe(false);
  });
});

describe('tunnelConfigEndpoints with excluded hosts', () => {
  it('yields no config when every host is excluded from the format', () => {
    const hosts = [hostRecord({ excludeFromSubTypes: ['raw'] })];
    expect(tunnelConfigEndpoints(tuicInbound, hosts, PANEL, '')).toEqual([]);
  });

  it('keeps the inbound own address when it has no enabled host', () => {
    expect(tunnelConfigEndpoints(tuicInbound, [], PANEL, '')).toEqual([undefined]);
    const disabled = [hostRecord({ isDisabled: true, excludeFromSubTypes: ['raw'] })];
    expect(tunnelConfigEndpoints(tuicInbound, disabled, PANEL, '')).toEqual([undefined]);
  });

  it('still lists the hosts of a format they are not excluded from', () => {
    const hosts = [hostRecord({ excludeFromSubTypes: ['raw'] })];
    const clash = tunnelConfigEndpoints(tuicInbound, hosts, PANEL, '', 'clash');
    expect(clash.map((ep) => ep?.dest)).toEqual(['edge.example.com']);
  });
});

describe('hostLinksClosed', () => {
  const wireguard = InboundSchema.parse({
    port: 51820,
    protocol: 'wireguard',
    settings: { secretKey: 'iJ2cBkrSGqRwIfYIDIxk7hr5RXfdR93MfJUL7yqkkH8=', peers: [], clients: [] },
  });

  it('closes host-reading protocols whose hosts are all excluded from raw', () => {
    const hosts = [hostRecord({ excludeFromSubTypes: ['raw'] })];
    expect(hostLinksClosed(wireguard, 7, hosts)).toBe(true);
    // the open case must keep producing the host endpoints it did before
    const open = [hostRecord({})];
    expect(hostLinksClosed(wireguard, 7, open)).toBe(false);
    expect(withHostEndpoints(wireguard, 7, open, '', PANEL).streamSettings).toMatchObject({
      externalProxy: [{ dest: 'edge.example.com', port: 443 }],
    });
  });

  it('leaves protocols that do not read hosts alone', () => {
    const vless = InboundSchema.parse({ port: 443, protocol: 'vless', settings: { clients: [] } });
    const hosts = [hostRecord({ excludeFromSubTypes: ['raw'] })];
    expect(hostLinksClosed(vless, 7, hosts)).toBe(false);
  });
});
