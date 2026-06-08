/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import { formValuesToWirePayload } from '@/lib/xray/inbound-form-adapter';
import { formValuesToWirePayload as outboundToWire } from '@/lib/xray/outbound-form-adapter';
import {
  normalizeSockoptForWire,
  normalizeStreamSettingsForWire,
  normalizeXhttpForWire,
  validateRealityTarget,
} from '@/lib/xray/stream-wire-normalize';
import { InboundFormSchema } from '@/schemas/forms/inbound-form';
import type { InboundFormValues } from '@/schemas/forms/inbound-form';

describe('validateRealityTarget', () => {
  it('accepts host:port and bare port', () => {
    expect(validateRealityTarget('play.google.com:443')).toBeUndefined();
    expect(validateRealityTarget('443')).toBeUndefined();
  });

  it('rejects host without port', () => {
    expect(validateRealityTarget('play.google.com')).toBe('pages.inbounds.form.realityTargetNeedsPort');
    expect(validateRealityTarget('')).toBe('pages.inbounds.form.realityTargetRequired');
  });
});

describe('normalizeXhttpForWire stream-one', () => {
  it('drops packet-up and stream-up-only fields on inbound', () => {
    const out = normalizeXhttpForWire({
      path: '/app',
      host: 'play.google.com',
      mode: 'stream-one',
      xPaddingBytes: '100-1000',
      scMaxEachPostBytes: '1000000',
      scMinPostsIntervalMs: '30',
      scMaxBufferedPosts: 30,
      scStreamUpServerSecs: '20-80',
      enableXmux: false,
      headers: {},
    }, 'inbound');

    expect(out).toMatchObject({
      path: '/app',
      host: 'play.google.com',
      mode: 'stream-one',
      xPaddingBytes: '100-1000',
    });
    expect(out).not.toHaveProperty('scMaxEachPostBytes');
    expect(out).not.toHaveProperty('scMinPostsIntervalMs');
    expect(out).not.toHaveProperty('scMaxBufferedPosts');
    expect(out).not.toHaveProperty('scStreamUpServerSecs');
    expect(out).not.toHaveProperty('enableXmux');
    expect(out).not.toHaveProperty('headers');
  });

  it('keeps xmux on outbound stream-one', () => {
    const out = normalizeXhttpForWire({
      path: '/app',
      mode: 'stream-one',
      xPaddingBytes: '100-1000',
      xmux: { maxConcurrency: '16-32' },
      scMaxEachPostBytes: '1000000',
    }, 'outbound');

    expect(out.xmux).toEqual({ maxConcurrency: '16-32' });
    expect(out).not.toHaveProperty('scMaxEachPostBytes');
  });
});

describe('normalizeSockoptForWire', () => {
  it('omits doc-example defaults that throttle throughput', () => {
    const out = normalizeSockoptForWire({
      tcpWindowClamp: 0,
      tcpMaxSeg: 0,
      tcpUserTimeout: 0,
      tcpFastOpen: true,
      tcpcongestion: 'bbr',
      domainStrategy: 'AsIs',
      tproxy: 'off',
      mark: 0,
    });

    expect(out).toEqual({
      tcpFastOpen: true,
      tcpcongestion: 'bbr',
    });
  });

  it('preserves happyEyeballs on freedom-style outbound', () => {
    const out = normalizeSockoptForWire({
      domainStrategy: 'UseIP',
      happyEyeballs: {
        tryDelayMs: 150,
        prioritizeIPv6: true,
        interleave: 1,
        maxConcurrentTry: 4,
      },
    });

    expect(out?.happyEyeballs).toMatchObject({
      tryDelayMs: 150,
      prioritizeIPv6: true,
    });
    expect(out?.domainStrategy).toBe('UseIP');
  });
});

describe('normalizeStreamSettingsForWire reality', () => {
  it('preserves the nested client settings on inbound (share links read publicKey from there)', () => {
    const out = normalizeStreamSettingsForWire({
      network: 'xhttp',
      security: 'reality',
      realitySettings: {
        target: 'play.google.com:443',
        privateKey: 'priv',
        serverNames: ['play.google.com'],
        shortIds: ['abcd'],
        settings: {
          publicKey: 'pub',
          fingerprint: 'chrome',
          spiderX: '/',
        },
      },
    }, { side: 'inbound' });

    const reality = out.realitySettings as Record<string, unknown>;
    expect(reality.target).toBe('play.google.com:443');
    expect(reality.privateKey).toBe('priv');
    const settings = reality.settings as Record<string, unknown>;
    expect(settings.publicKey).toBe('pub');
    expect(settings.spiderX).toBe('/');
  });

  it('passes client realitySettings through unchanged on outbound', () => {
    const out = normalizeStreamSettingsForWire({
      network: 'xhttp',
      security: 'reality',
      realitySettings: {
        publicKey: 'pub',
        fingerprint: 'chrome',
        serverName: 'play.google.com',
        shortId: 'abcd',
        spiderX: '/x',
      },
    }, { side: 'outbound' });

    const reality = out.realitySettings as Record<string, unknown>;
    expect(reality.publicKey).toBe('pub');
    expect(reality.serverName).toBe('play.google.com');
    expect(reality.spiderX).toBe('/x');
  });
});

describe('normalizeStreamSettingsForWire tls', () => {
  it('drops empty uTLS fingerprints from inbound and outbound TLS shapes', () => {
    const out = normalizeStreamSettingsForWire({
      network: 'hysteria',
      security: 'tls',
      tlsSettings: {
        fingerprint: '',
        settings: {
          fingerprint: '',
          echConfigList: '',
        },
      },
    }, { side: 'inbound' });

    const tls = out.tlsSettings as Record<string, unknown>;
    const settings = tls.settings as Record<string, unknown>;
    expect(tls).not.toHaveProperty('fingerprint');
    expect(settings).not.toHaveProperty('fingerprint');
    expect(settings.echConfigList).toBe('');
  });
});

describe('inbound formValuesToWirePayload integration', () => {
  it('emits lean stream-one xhttp + sockopt on save', () => {
    const values = {
      remark: 't',
      enable: true,
      port: 443,
      listen: '0.0.0.0',
      tag: 'in-443',
      expiryTime: 0,
      sniffing: { enabled: false },
      up: 0,
      down: 0,
      total: 0,
      trafficReset: 'never',
      lastTrafficResetTime: 0,
      nodeId: null,
      protocol: 'vless',
      settings: { clients: [{ id: '7eeb09ed-ae97-400d-a1ce-2485fb904407', email: 'n' }], decryption: 'none' },
      streamSettings: {
        network: 'xhttp',
        security: 'reality',
        realitySettings: {
          target: 'play.google.com:443',
          privateKey: 'priv',
          serverNames: ['play.google.com'],
          shortIds: ['44003d86dc1e'],
          settings: { publicKey: 'pub', fingerprint: 'chrome', spiderX: '/' },
        },
        xhttpSettings: {
          path: '/app',
          host: 'play.google.com',
          mode: 'stream-one',
          xPaddingBytes: '100-1000',
          scMaxEachPostBytes: '1000000',
          scMinPostsIntervalMs: '30',
          enableXmux: false,
        },
        sockopt: {
          tcpWindowClamp: 0,
          tcpMaxSeg: 0,
          tcpUserTimeout: 0,
          tcpFastOpen: true,
          tcpcongestion: 'bbr',
        },
      },
    } as InboundFormValues;

    const payload = formValuesToWirePayload(values);
    const stream = JSON.parse(payload.streamSettings) as Record<string, unknown>;
    const xhttp = stream.xhttpSettings as Record<string, unknown>;
    const sockopt = stream.sockopt as Record<string, unknown>;
    const reality = stream.realitySettings as Record<string, unknown>;

    expect(xhttp).not.toHaveProperty('scMaxEachPostBytes');
    expect(sockopt).not.toHaveProperty('tcpWindowClamp');
    expect(sockopt.tcpFastOpen).toBe(true);
    const realitySettings = reality.settings as Record<string, unknown>;
    expect(realitySettings.publicKey).toBe('pub');
  });

  it('accepts Hysteria TLS with uTLS None and omits fingerprint on save', () => {
    const values = {
      remark: 'hy2',
      enable: true,
      port: 443,
      listen: '',
      tag: 'hy2-443',
      expiryTime: 0,
      sniffing: { enabled: false },
      up: 0,
      down: 0,
      total: 0,
      trafficReset: 'never',
      lastTrafficResetTime: 0,
      nodeId: null,
      protocol: 'hysteria',
      settings: { version: 2, clients: [] },
      streamSettings: {
        network: 'hysteria',
        security: 'tls',
        hysteriaSettings: {
          version: 2,
          auth: 'auth',
          udpIdleTimeout: 60,
        },
        tlsSettings: {
          alpn: ['h3'],
          settings: {
            fingerprint: '',
          },
        },
      },
    };

    const parsed = InboundFormSchema.safeParse(values);
    expect(parsed.success).toBe(true);
    if (!parsed.success) throw parsed.error;

    const payload = formValuesToWirePayload(parsed.data);
    const stream = JSON.parse(payload.streamSettings) as Record<string, unknown>;
    const tls = stream.tlsSettings as Record<string, unknown>;
    const settings = tls.settings as Record<string, unknown>;
    expect(settings).not.toHaveProperty('fingerprint');
  });
});

describe('freedom outbound sockopt wire payload', () => {
  it('preserves happyEyeballs on direct freedom outbound', () => {
    const wire = outboundToWire({
      protocol: 'freedom',
      tag: 'direct',
      settings: { domainStrategy: 'UseIP' },
      streamSettings: {
        sockopt: {
          domainStrategy: 'UseIP',
          happyEyeballs: {
            tryDelayMs: 150,
            prioritizeIPv6: true,
            interleave: 1,
            maxConcurrentTry: 4,
          },
        },
      },
    } as Parameters<typeof outboundToWire>[0]);

    expect(wire.streamSettings).toMatchObject({
      sockopt: {
        domainStrategy: 'UseIP',
        happyEyeballs: {
          tryDelayMs: 150,
          prioritizeIPv6: true,
        },
      },
    });
  });
});
