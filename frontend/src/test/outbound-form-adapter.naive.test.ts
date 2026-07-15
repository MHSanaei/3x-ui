import { describe, expect, it } from 'vitest';

import {
  formValuesToWirePayload,
  rawOutboundToFormValues,
} from '@/lib/xray/outbound-form-adapter';

describe('outbound-form-adapter: naive', () => {
  it('maps naive wire payload into split form fields', () => {
    const form = rawOutboundToFormValues({
      protocol: 'naive',
      tag: 'naive-a',
      settings: {
        proxy: 'https://user:pass@example.com:8443',
        tunnelTimeout: 1800,
        idleTimeout: 600,
      },
    });
    expect(form.protocol).toBe('naive');
    if (form.protocol === 'naive') {
      expect(form.settings.scheme).toBe('https');
      expect(form.settings.user).toBe('user');
      expect(form.settings.pass).toBe('pass');
      expect(form.settings.host).toBe('example.com');
      expect(form.settings.port).toBe(8443);
      expect(form.settings.tunnelTimeout).toBe(1800);
    }
  });

  it('serializes split form fields back into the wire proxy URL', () => {
    const wire = formValuesToWirePayload({
      protocol: 'naive',
      tag: 'naive-b',
      sendThrough: '',
      targetStrategy: '',
      settings: {
        scheme: 'quic',
        user: 'user',
        pass: 'pass',
        host: 'server.example',
        port: 443,
        insecureConcurrency: 2,
        tunnelTimeout: 900,
      },
      mux: { enabled: false, concurrency: 8, xudpConcurrency: 16, xudpProxyUDP443: 'reject' },
    });
    expect(wire).toMatchObject({
      protocol: 'naive',
      tag: 'naive-b',
      settings: {
        proxy: 'quic://user:pass@server.example:443',
        insecureConcurrency: 2,
        tunnelTimeout: 900,
      },
    });
    expect(wire).not.toHaveProperty('streamSettings');
    expect(wire).not.toHaveProperty('mux');
  });

  it('encodes special characters in credentials without double-encoding', () => {
    const wire = formValuesToWirePayload({
      protocol: 'naive',
      tag: 'naive-special',
      sendThrough: '',
      targetStrategy: '',
      settings: {
        scheme: 'https',
        user: 'user@domain',
        pass: 'p/a%ss',
        host: 'example.com',
        port: 443,
      },
      mux: { enabled: false, concurrency: 8, xudpConcurrency: 16, xudpProxyUDP443: 'reject' },
    });
    const naiveWire = wire as { settings: { proxy: string } };
    expect(naiveWire.settings.proxy).toBe('https://user%40domain:p%2Fa%25ss@example.com:443');
  });

  it('preserves literal percent in credentials', () => {
    const wire = formValuesToWirePayload({
      protocol: 'naive',
      tag: 'naive-percent',
      sendThrough: '',
      targetStrategy: '',
      settings: {
        scheme: 'https',
        user: 'admin',
        pass: '100%safe',
        host: 'example.com',
        port: 443,
      },
      mux: { enabled: false, concurrency: 8, xudpConcurrency: 16, xudpProxyUDP443: 'reject' },
    });
    const naiveWire = wire as { settings: { proxy: string } };
    expect(naiveWire.settings.proxy).toBe('https://admin:100%25safe@example.com:443');
  });
});