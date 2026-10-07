import { describe, expect, it } from 'vitest';

import { buildTuicClientConfig } from '@/pages/clients/tuicConfig';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';

const client: ClientRecord = {
  id: 1,
  email: 'tuic-user@example.com',
  uuid: 'e79b9107-1607-4e6c-a496-d8f99e4f0dc5',
  password: 'testpassword123',
  inboundIds: [10],
};

const inbound: InboundOption = {
  id: 10,
  tag: 'in-8443-udp',
  remark: 'TUIC Main',
  protocol: 'tuic',
  port: 8443,
  tuicServer: {
    sni: 'vpn.example.com',
    congestion_control: 'cubic',
    alpn: ['h3', 'spdy/3.1'],
    udp_relay_mode: 'native',
    zero_rtt_handshake: true,
  },
};

describe('buildTuicClientConfig', () => {
  it('builds valid YAML proxy entry from tuicServer option', () => {
    const cfg = buildTuicClientConfig(client, inbound, 'server.example.com', '');
    expect(cfg).toContain('type: tuic');
    expect(cfg).toContain('server: server.example.com');
    expect(cfg).toContain('port: 8443');
    expect(cfg).toContain('uuid: e79b9107-1607-4e6c-a496-d8f99e4f0dc5');
    expect(cfg).toContain('password: "testpassword123"');
    expect(cfg).toContain('sni: vpn.example.com');
    expect(cfg).toContain('congestion-controller: cubic');
    expect(cfg).toContain('udp-relay-mode: native');
    expect(cfg).toContain('reduce-rtt: true');
  });

  it('falls back to endpointHost when sni is empty', () => {
    const inboundNoSni: InboundOption = {
      ...inbound,
      tuicServer: {
        ...inbound.tuicServer,
        sni: '',
      },
    };
    const cfg = buildTuicClientConfig(client, inboundNoSni, 'server.example.com', '');
    expect(cfg).toContain('sni: server.example.com');
  });

  it('exports canonical controller values for legacy settings', () => {
    const legacyInbound = {
      ...inbound,
      tuicServer: { ...inbound.tuicServer, congestion_control: ' RENO ' },
    } as InboundOption;
    const cfg = buildTuicClientConfig(client, legacyInbound, 'server.example.com', '');
    expect(cfg).toContain('congestion-controller: new_reno');
    expect(cfg).not.toContain('congestion-controller:  RENO ');
  });

  it('escapes quotes in passwords and remarks', () => {
    const dangerousClient: ClientRecord = {
      ...client,
      password: 'pass"with"quotes\nnewline',
    };
    const cfg = buildTuicClientConfig(dangerousClient, inbound, 'server.example.com', '');
    expect(cfg).toContain('password: "pass\\"with\\"quotes\\nnewline"');
  });

  it.each([
    ['bbr', 'bbr'],
    ['cubic', 'cubic'],
    [' RENO ', 'new_reno'],
    ['unknown', 'new_reno'],
  ])('keeps controller %s and relay settings when applying a Host', (controller, expected) => {
    const config = buildTuicClientConfig(
      client,
      {
        ...inbound,
        tuicServer: {
          ...inbound.tuicServer,
          congestion_control: controller,
          udp_relay_mode: 'quic',
          zero_rtt_handshake: false,
        },
      } as InboundOption,
      'panel.example.com',
      '',
      {
        dest: 'edge.example.com',
        port: 9443,
        remark: 'Edge',
        sni: 'edge.sni.example.com',
        alpn: ['h3'],
        allowInsecure: true,
      },
    );
    expect(config).toContain('server: edge.example.com');
    expect(config).toContain('port: 9443');
    expect(config).toContain('sni: edge.sni.example.com');
    expect(config).toContain('alpn:\n      - h3\n');
    expect(config).toContain(`congestion-controller: ${expected}`);
    expect(config).toContain('udp-relay-mode: quic');
    expect(config).toContain('reduce-rtt: false');
    expect(config).toContain('skip-cert-verify: true');
  });
});
