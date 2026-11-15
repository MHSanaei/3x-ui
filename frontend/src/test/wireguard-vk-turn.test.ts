import { describe, expect, it } from 'vitest';

import { buildVkTurnConnectionLink } from '@/lib/xray/vk-turn-link';
import { parseLinkParts } from '@/lib/xray/link-label';
import { genWireguardLinks } from '@/lib/xray/inbound-link';
import { InboundSchema } from '@/schemas/api/inbound';
import { WireguardInboundSettingsSchema } from '@/schemas/protocols/inbound/wireguard';
import { Base64 } from '@/utils';

const secretKey = 'iJ2cBkrSGqRwIfYIDIxk7hr5RXfdR93MfJUL7yqkkH8=';
const privateKey = 'QGVlb2dXc1ZTWGw0ZXBzZndsWmtMaUM5MUlNYjBHWFdYbz0=';
const callLink = 'https://vk.ru/call/join/test-call-token';

function inbound(enabled = true) {
  return InboundSchema.parse({
    protocol: 'wireguard',
    port: 51820,
    settings: {
      secretKey,
      vkTurn: { enabled, callLink, port: 56000 },
      clients: [
        { email: 'alice', privateKey, preSharedKey: 'shared-key', allowedIPs: ['10.0.0.2/32'] },
      ],
    },
  });
}

describe('WireGuard VK TURN export', () => {
  it('exports a usable iOS SRTP profile alongside the ordinary WireGuard link', () => {
    const links = genWireguardLinks({
      inbound: inbound(),
      fallbackHostname: 'wg.example.test',
    }).split('\r\n');
    expect(links).toHaveLength(2);
    expect(links[0]).toContain('wireguard://');
    const encoded = new URL(links[1]).searchParams.get('data') ?? '';
    expect(JSON.parse(Base64.decode(encoded))).toEqual({
      version: 1,
      type: 'connection',
      settings: {
        privateKey,
        peerPublicKey: 'Piehk2n8UewhMHMyJiBS+Sxn/OK0FalyFW1GAGzokHM=',
        presharedKey: 'shared-key',
        tunnelAddress: '10.0.0.2/32',
        peerAddress: 'wg.example.test:56000',
        vkLink: callLink,
        dnsServers: '1.1.1.1, 1.0.0.1',
        numConnections: 30,
        useDTLS: true,
        useSrtp: true,
        useUDP: false,
        useWrap: false,
        useWrapA: false,
        useWrapS: false,
        useCsqtt: false,
        wrapKeyHex: '',
      },
    });
  });

  it('labels the imported profile as WireGuard with VK and SRTP transport', () => {
    const links = genWireguardLinks({
      inbound: inbound(),
      fallbackHostname: 'wg.example.test',
    }).split('\r\n');
    expect(parseLinkParts(links[1])).toEqual({
      protocol: 'WireGuard',
      network: 'VK',
      security: 'SRTP',
      remark: '',
      port: '56000',
    });
  });

  it('keeps base64 plus characters intact through URL query parsing', () => {
    const link = buildVkTurnConnectionLink({
      vkTurn: { enabled: true, callLink, port: 56000 },
      privateKey,
      peerPublicKey: 'server-key',
      tunnelAddress: '10.0.0.2/32',
      address: 'wg.example.test',
      dnsServers: '😀😀😀',
    });
    const encoded = new URL(link).searchParams.get('data') ?? '';
    expect(encoded).toContain('+');
    expect(JSON.parse(Base64.decode(encoded)).settings.dnsServers).toBe('😀😀😀');
  });

  it('preserves ordinary exports when VK TURN is disabled', () => {
    const links = genWireguardLinks({
      inbound: inbound(false),
      fallbackHostname: 'wg.example.test',
    }).split('\r\n');
    expect(links).toHaveLength(1);
    expect(links[0]).toContain('wireguard://');
  });

  it('rejects invalid call URLs only when VK TURN is enabled', () => {
    const raw = {
      secretKey,
      vkTurn: { enabled: true, callLink: 'https://evil.test/call/join/token', port: 56000 },
    };
    const parsed = WireguardInboundSettingsSchema.safeParse(raw);
    expect(parsed.success).toBe(false);
    if (!parsed.success) expect(parsed.error.issues[0].path).toEqual(['vkTurn', 'callLink']);
    expect(
      WireguardInboundSettingsSchema.safeParse({
        ...raw,
        vkTurn: { ...raw.vkTurn, enabled: false },
      }).success,
    ).toBe(true);
  });
});
