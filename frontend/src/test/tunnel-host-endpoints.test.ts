import { describe, expect, it } from 'vitest';

import type { ClientRecord, InboundOption } from '@/hooks/useClients';
import { withHostEndpoints } from '@/lib/hosts/host-link';
import { formatTunnelConfigMeta } from '@/lib/inbounds/label';
import { genAmneziaWGPeerConfigs, genWireguardPeerConfigs } from '@/lib/xray/inbound-link';
import { buildAmneziaWGClientConfig } from '@/pages/clients/amneziawgConfig';
import { buildTuicClientConfig } from '@/pages/clients/tuicConfig';
import { tunnelConfigEndpoints } from '@/pages/clients/tunnelEndpoints';
import { buildWireguardClientConfig } from '@/pages/clients/wireguardConfig';
import { InboundSchema, type Inbound } from '@/schemas/api/inbound';
import type { HostRecord } from '@/schemas/api/host';

const PANEL = 'panel.example.com';
const HOSTS: HostRecord[] = [
  {
    groupId: 'cdn',
    inboundIds: [7],
    hosts: ['edge.example.com:443', 'edge2.example.com'],
    remark: 'CDN',
  },
];

function endpointLines(configs: string[]): string[] {
  return configs.map((cfg) => cfg.match(/^Endpoint = (.*)$/m)?.[1] ?? '');
}

// The panel's own tunnel previews must advertise the same Hosts the
// subscription does, not the panel address (#6369 did this for MTProto).
describe('inbounds page tunnel configs follow Hosts', () => {
  it('renders one WireGuard config per Host for each peer', () => {
    const inbound = InboundSchema.parse({
      port: 51820,
      protocol: 'wireguard',
      settings: {
        secretKey: 'iJ2cBkrSGqRwIfYIDIxk7hr5RXfdR93MfJUL7yqkkH8=',
        peers: [],
        clients: [
          {
            email: 'alice',
            privateKey: 'QGVlb2dXc1ZTWGw0ZXBzZndsWmtMaUM5MUlNYjBHWFdYbz0=',
            allowedIPs: ['10.0.0.2/32'],
          },
        ],
      },
    });
    const peers = genWireguardPeerConfigs({
      inbound: withHostEndpoints(inbound, 7, HOSTS, '', PANEL),
      remark: 'wg',
      fallbackHostname: PANEL,
    });
    expect(peers).toHaveLength(1);
    expect(endpointLines(peers[0])).toEqual(['edge.example.com:443', 'edge2.example.com:51820']);
  });

  it('renders one AmneziaWG config per Host for each peer', () => {
    const inbound = {
      port: 51821,
      protocol: 'amneziawg',
      settings: {
        server: { publicKey: 'serverPubKey==', jc: 4, jmin: 40, jmax: 100, s1: 30, s2: 90 },
        clients: [{ email: 'alice', privateKey: 'clientPrivKey==', allowedIPs: ['10.8.1.2/32'] }],
      },
      streamSettings: {},
    } as unknown as Inbound;
    const peers = genAmneziaWGPeerConfigs({
      inbound: withHostEndpoints(inbound, 7, HOSTS, '', PANEL),
      remark: 'awg',
      fallbackHostname: PANEL,
    });
    expect(peers).toHaveLength(1);
    expect(endpointLines(peers[0])).toEqual(['edge.example.com:443', 'edge2.example.com:51821']);
  });
});

describe('clients page tunnel configs follow Hosts', () => {
  const client = {
    email: 'alice',
    privateKey: 'clientPrivKey==',
    allowedIPs: '10.0.0.2/32',
    uuid: 'e79b9107-1607-4e6c-a496-d8f99e4f0dc5',
    password: 'secret',
  } as unknown as ClientRecord;

  it('advertises each Host in the WireGuard config', () => {
    const inbound = { id: 7, remark: 'wg', protocol: 'wireguard', port: 51820 } as InboundOption;
    const configs = tunnelConfigEndpoints(inbound, HOSTS, PANEL, '').map((ep) =>
      buildWireguardClientConfig(client, inbound, PANEL, '', '', ep),
    );
    expect(endpointLines(configs)).toEqual(['edge.example.com:443', 'edge2.example.com:51820']);
  });

  it('advertises each Host in the AmneziaWG config', () => {
    const inbound = {
      id: 7,
      remark: 'awg',
      protocol: 'amneziawg',
      port: 51821,
      awgServer: { publicKey: 'serverPubKey==', jc: 4, jmin: 40, jmax: 100, s1: 30, s2: 90 },
    } as unknown as InboundOption;
    const configs = tunnelConfigEndpoints(inbound, HOSTS, PANEL, '').map((ep) =>
      buildAmneziaWGClientConfig(client, inbound, PANEL, '', '', ep),
    );
    expect(endpointLines(configs)).toEqual(['edge.example.com:443', 'edge2.example.com:51821']);
  });

  it('keeps the inbound address when no Host applies to it', () => {
    const inbound = { id: 8, remark: 'wg', protocol: 'wireguard', port: 51820 } as InboundOption;
    const configs = tunnelConfigEndpoints(inbound, HOSTS, PANEL, '').map((ep) =>
      buildWireguardClientConfig(client, inbound, PANEL, '', '', ep),
    );
    expect(endpointLines(configs)).toEqual([`${PANEL}:51820`]);
  });

  it('applies a Host SNI, ALPN and insecure flag to the TUIC config', () => {
    const inbound = {
      id: 7,
      remark: 'tuic',
      protocol: 'tuic',
      port: 8443,
      tuicServer: { sni: 'inbound.sni', alpn: ['h3'] },
    } as unknown as InboundOption;
    const hosts: HostRecord[] = [
      {
        groupId: 'tuic',
        inboundIds: [7],
        hosts: ['tuic.example.com:9443'],
        sni: 'host.sni',
        alpn: ['h3', 'h2'],
        allowInsecure: true,
      },
    ];
    const [ep] = tunnelConfigEndpoints(inbound, hosts, PANEL, '');
    const cfg = buildTuicClientConfig(client, inbound, PANEL, '', ep);
    expect(cfg).toContain('server: tuic.example.com');
    expect(cfg).toContain('port: 9443');
    expect(cfg).toContain('sni: host.sni');
    expect(cfg).toContain('alpn:\n      - h3\n      - h2\n');
    expect(cfg).toContain('skip-cert-verify: true');
  });

  it('skips a Host excluded from Clash in the TUIC Clash config', () => {
    const inbound = { id: 7, remark: 'tuic', protocol: 'tuic', port: 8443 } as InboundOption;
    const hosts: HostRecord[] = [
      {
        groupId: 'raw-only',
        inboundIds: [7],
        hosts: ['raw.example.com:443'],
        excludeFromSubTypes: ['clash'],
      },
    ];
    const configs = tunnelConfigEndpoints(inbound, hosts, PANEL, '', 'clash').map((ep) =>
      buildTuicClientConfig(client, inbound, PANEL, '', ep),
    );
    expect(configs).toHaveLength(1);
    expect(configs[0]).toContain(`server: ${PANEL}`);
  });

  it('names two Hosts of one inbound apart', () => {
    const inbound = { id: 7, remark: 'wg' };
    const a = formatTunnelConfigMeta(inbound, 'alice', 2, 'edge.example.com:443');
    const b = formatTunnelConfigMeta(inbound, 'alice', 2, 'edge2.example.com:51820');
    expect([a.fileName, b.fileName]).toEqual([
      'alice-wg-edge.example.com_443.conf',
      'alice-wg-edge2.example.com_51820.conf',
    ]);
    expect([a.label, b.label]).toEqual([
      'wg - edge.example.com:443',
      'wg - edge2.example.com:51820',
    ]);
  });
});
