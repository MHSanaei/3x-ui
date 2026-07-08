import { describe, expect, it } from 'vitest';

import { buildWireguardClientConfig } from '@/pages/clients/wireguardConfig';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';

const client: ClientRecord = {
  email: 'alice',
  privateKey: 'QGVlb2dXc1ZTWGw0ZXBzZndsWmtMaUM5MUlNYjBHWFdYbz0=',
  allowedIPs: '10.0.0.2/32',
  preSharedKey: 'cHNrLXZhbHVlLWZvci13aXJlZ3VhcmQtdGVzdC1jYXNlIQ==',
  keepAlive: 25,
  inboundIds: [90],
};

const inbound: InboundOption = {
  id: 90,
  tag: 'in-51820-udp',
  remark: 'wg-mc',
  protocol: 'wireguard',
  port: 51820,
  wgPublicKey: 'DGSYIcEKAUkA7HhzGSjxLZuV67BR3LeyU0BMLJzNVHQ=',
  wgMtu: 1420,
};

describe('buildWireguardClientConfig', () => {
  it('emits the canonical PresharedKey key, not PreSharedKey', () => {
    const cfg = buildWireguardClientConfig(client, inbound, 'example.com', '');
    expect(cfg).toContain(`PresharedKey = ${client.preSharedKey}`);
    expect(cfg).not.toContain('PreSharedKey =');
  });

  it('defaults DNS to 1.1.1.1, 1.0.0.1 when the inbound sets none', () => {
    const cfg = buildWireguardClientConfig(client, inbound, 'example.com', '');
    expect(cfg).toContain('DNS = 1.1.1.1, 1.0.0.1');
  });

  it('uses the inbound DNS override when present', () => {
    const cfg = buildWireguardClientConfig(client, { ...inbound, wgDns: '9.9.9.9' }, 'example.com', '');
    expect(cfg).toContain('DNS = 9.9.9.9');
    expect(cfg).not.toContain('DNS = 1.1.1.1, 1.0.0.1');
  });

  it('builds the endpoint from host, port, MTU and server public key', () => {
    const cfg = buildWireguardClientConfig(client, inbound, 'example.com', '');
    expect(cfg).toContain('Endpoint = example.com:51820');
    expect(cfg).toContain('MTU = 1420');
    expect(cfg).toContain(`PublicKey = ${inbound.wgPublicKey}`);
    expect(cfg).toContain('PersistentKeepalive = 25');
  });

  it('omits the PresharedKey line when the client has no preshared key', () => {
    const cfg = buildWireguardClientConfig({ ...client, preSharedKey: undefined }, inbound, 'example.com', '');
    expect(cfg).not.toContain('PresharedKey');
  });

  it('uses the hosting node address as the endpoint host for node-managed inbounds', () => {
    const cfg = buildWireguardClientConfig(client, { ...inbound, nodeAddress: 'node.example.net' }, 'master.example.com', '');
    expect(cfg).toContain('Endpoint = node.example.net:51820');
    expect(cfg).not.toContain('master.example.com');
  });

  it('falls back to the panel host when the node address is blank', () => {
    const cfg = buildWireguardClientConfig(client, { ...inbound, nodeAddress: '   ' }, 'master.example.com', '');
    expect(cfg).toContain('Endpoint = master.example.com:51820');
  });

  it('honors the custom share-address strategy over the node address', () => {
    const cfg = buildWireguardClientConfig(
      client,
      { ...inbound, nodeAddress: 'node.example.net', shareAddrStrategy: 'custom', shareAddr: 'vpn.example.com' },
      'master.example.com',
      '',
    );
    expect(cfg).toContain('Endpoint = vpn.example.com:51820');
  });

  it('honors the listen share-address strategy over the node address', () => {
    const cfg = buildWireguardClientConfig(
      client,
      { ...inbound, nodeAddress: 'node.example.net', shareAddrStrategy: 'listen', listen: '198.51.100.7' },
      'master.example.com',
      '',
    );
    expect(cfg).toContain('Endpoint = 198.51.100.7:51820');
  });

  it('keeps a panel hostname that fails share-host normalization instead of emitting an empty endpoint', () => {
    const cfg = buildWireguardClientConfig(client, { ...inbound, listen: '0.0.0.0' }, 'wg_gw.corp.lan', '');
    expect(cfg).toContain('Endpoint = wg_gw.corp.lan:51820');
    expect(cfg).not.toContain('Endpoint = :51820');
  });
});
