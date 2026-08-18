import { describe, expect, it } from 'vitest';

import {
  formValuesToWirePayload,
  rawOutboundToFormValues,
} from '@/lib/xray/outbound-form-adapter';

function naiveProxy(outbound: Record<string, unknown>): string {
  return (outbound.settings as { proxy: string }).proxy;
}

describe('outbound-form-adapter: naive', () => {
  it('maps a wire payload into form fields', () => {
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
    if (form.protocol !== 'naive') return;
    expect(form.settings).toMatchObject({
      scheme: 'https',
      user: 'user',
      pass: 'pass',
      host: 'example.com',
      port: 8443,
      tunnelTimeout: 1800,
      idleTimeout: 600,
    });
  });

  it('serializes form fields into the wire proxy URL', () => {
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

  it('round-trips encoded credentials without double encoding', () => {
    const source = {
      protocol: 'naive',
      tag: 'naive-special',
      settings: {
        proxy: 'https://user%40domain:p%2Fa%25ss@example.com:443',
      },
    };
    const form = rawOutboundToFormValues(source);

    expect(form.protocol).toBe('naive');
    if (form.protocol !== 'naive') return;
    expect(form.settings.user).toBe('user@domain');
    expect(form.settings.pass).toBe('p/a%ss');
    expect(naiveProxy(formValuesToWirePayload(form))).toBe(source.settings.proxy);
  });

  it('preserves percent-encoded-looking text typed literally', () => {
    const wire = formValuesToWirePayload({
      protocol: 'naive',
      tag: 'naive-percent',
      sendThrough: '',
      targetStrategy: '',
      settings: {
        scheme: 'https',
        user: 'admin',
        pass: 'literal%40value',
        host: 'example.com',
        port: 443,
      },
      mux: { enabled: false, concurrency: 8, xudpConcurrency: 16, xudpProxyUDP443: 'reject' },
    });

    expect(naiveProxy(wire)).toBe('https://admin:literal%2540value@example.com:443');
    const form = rawOutboundToFormValues(wire);
    expect(form.protocol).toBe('naive');
    if (form.protocol !== 'naive') return;
    expect(form.settings.pass).toBe('literal%40value');
  });

  it('preserves explicit zero timeout values', () => {
    const form = rawOutboundToFormValues({
      protocol: 'naive',
      settings: {
        proxy: 'https://user:pass@example.com:443',
        tunnelTimeout: 0,
        idleTimeout: 0,
      },
    });

    expect(form.protocol).toBe('naive');
    if (form.protocol !== 'naive') return;
    expect(form.settings.tunnelTimeout).toBe(0);
    expect(form.settings.idleTimeout).toBe(0);
    expect(formValuesToWirePayload(form).settings).toMatchObject({
      tunnelTimeout: 0,
      idleTimeout: 0,
    });
  });

  it('falls back to HTTPS for an unsupported proxy scheme', () => {
    const form = rawOutboundToFormValues({
      protocol: 'naive',
      settings: { proxy: 'ftp://user:pass@example.com' },
    });

    expect(form.protocol).toBe('naive');
    if (form.protocol !== 'naive') return;
    expect(form.settings.scheme).toBe('https');
  });

  it('drops stale stream settings from a Naive payload', () => {
    const form = rawOutboundToFormValues({
      protocol: 'naive',
      tag: 'naive-clean',
      sendThrough: '127.0.0.1',
      targetStrategy: 'UseIPv4',
      settings: { proxy: 'https://u:p@example.com:443' },
      streamSettings: { sockopt: { dialerProxy: 'old-outbound' } },
      mux: { enabled: true, concurrency: 8 },
    });
    const wire = formValuesToWirePayload(form);

    expect(wire).not.toHaveProperty('sendThrough');
    expect(wire).not.toHaveProperty('targetStrategy');
    expect(wire).not.toHaveProperty('streamSettings');
    expect(wire).not.toHaveProperty('mux');
  });

  it('uses port 80 for an HTTP proxy without an explicit port', () => {
    const form = rawOutboundToFormValues({
      protocol: 'naive',
      settings: { proxy: 'http://user:pass@example.com' },
    });

    expect(form.protocol).toBe('naive');
    if (form.protocol !== 'naive') return;
    expect(form.settings.port).toBe(80);
  });

  it('serializes bracketed IPv6 hosts', () => {
    const wire = formValuesToWirePayload({
      protocol: 'naive',
      tag: 'naive-v6',
      sendThrough: '',
      targetStrategy: '',
      settings: {
        scheme: 'https',
        user: 'u',
        pass: 'p',
        host: '2001:db8::1',
        port: 443,
      },
      mux: { enabled: false, concurrency: 8, xudpConcurrency: 16, xudpProxyUDP443: 'reject' },
    });

    expect(naiveProxy(wire)).toBe('https://u:p@[2001:db8::1]:443');
  });

  it('round-trips encoded-looking credential text without changing its meaning', () => {
    const form = rawOutboundToFormValues({
      protocol: 'naive',
      tag: 'naive-roundtrip',
      settings: {
        proxy: 'https://literal%2540user:p%252Fword%2525@example.com:443',
      },
    });
    expect(form.protocol).toBe('naive');
    if (form.protocol !== 'naive') return;
    expect(form.settings.user).toBe('literal%40user');
    expect(form.settings.pass).toBe('p%2Fword%25');

    const wire = formValuesToWirePayload({
      ...form,
      sendThrough: '',
      targetStrategy: '',
      mux: { enabled: false, concurrency: 8, xudpConcurrency: 16, xudpProxyUDP443: 'reject' },
    });
    const naiveWire = wire as { settings: { proxy: string } };
    expect(naiveWire.settings.proxy).toBe(
      'https://literal%2540user:p%252Fword%2525@example.com:443',
    );
  });

});
