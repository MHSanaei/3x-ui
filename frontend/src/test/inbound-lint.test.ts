import { describe, expect, it } from 'vitest';

import { lintInboundConfig } from '@/lib/xray/inbound-lint';

describe('lintInboundConfig', () => {
  it('warns on fingerprintable Reality defaults', () => {
    const issues = lintInboundConfig({
      protocol: 'vless',
      settings: { clients: [{ id: 'u', email: 'e', flow: '' }] },
      streamSettings: {
        network: 'tcp',
        security: 'reality',
        realitySettings: {
          target: 'images.apple.com:443',
          shortIds: ['abcd'],
          settings: { fingerprint: 'chrome', spiderX: '/' },
        },
      },
    });

    expect(issues.map((issue) => issue.key)).toEqual(expect.arrayContaining([
      'reality-shortids',
      'reality-spiderx',
      'reality-fingerprint',
      'reality-default-target',
      'reality-vision',
    ]));
  });

  it('warns on XHTTP root path and stable interval', () => {
    const issues = lintInboundConfig({
      protocol: 'vless',
      streamSettings: {
        network: 'xhttp',
        security: 'reality',
        xhttpSettings: { path: '/', scMinPostsIntervalMs: '30' },
        realitySettings: {
          target: 'www.microsoft.com:443',
          shortIds: ['abcd', '1234'],
          settings: { fingerprint: 'firefox', spiderX: '/assets/a' },
        },
      },
    });

    expect(issues.map((issue) => issue.key)).toEqual(expect.arrayContaining([
      'xhttp-root-path',
      'xhttp-interval-default',
    ]));
  });

  it('accepts a Hysteria2 config with masquerade, obfs, and udpHop', () => {
    const issues = lintInboundConfig({
      protocol: 'hysteria',
      streamSettings: {
        network: 'hysteria',
        security: 'tls',
        hysteriaSettings: {
          masquerade: { type: 'string', statusCode: 200, content: 'ok' },
        },
        finalmask: {
          udp: [{ type: 'salamander', settings: { password: 'secret' } }],
          quicParams: { udpHop: { ports: '20000-50000', interval: '5-10' } },
        },
      },
    });

    expect(issues).toEqual([]);
  });
});
