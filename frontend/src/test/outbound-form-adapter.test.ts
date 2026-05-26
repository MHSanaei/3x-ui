import { describe, expect, it } from 'vitest';

import {
  formValuesToWirePayload,
  rawOutboundToFormValues,
} from '@/lib/xray/outbound-form-adapter';

// Round-trip parity: wire → form → wire should preserve the legacy
// Outbound.fromJson(...).toJson() output shape for each protocol's quirks.
// Spot-checking the cases the modal exercised in v0.x — vmess vnext flatten,
// vless reverse-wrap, wireguard address csv ↔ array, freedom finalRules
// emission, blackhole type wrap, dns rule normalization, mux gating.

describe('outbound-form-adapter: round-trip', () => {
  it('vmess flattens vnext to address/port/id/security and re-nests', () => {
    const wire = {
      protocol: 'vmess',
      tag: 'outbound-vmess',
      settings: {
        vnext: [{
          address: '1.2.3.4',
          port: 443,
          users: [{ id: '11111111-2222-4333-8444-555555555555', security: 'auto' }],
        }],
      },
    };
    const form = rawOutboundToFormValues(wire);
    expect(form.protocol).toBe('vmess');
    if (form.protocol === 'vmess') {
      expect(form.settings.address).toBe('1.2.3.4');
      expect(form.settings.port).toBe(443);
      expect(form.settings.id).toBe('11111111-2222-4333-8444-555555555555');
      expect(form.settings.security).toBe('auto');
    }
    const back = formValuesToWirePayload(form);
    expect(back).toMatchObject({
      protocol: 'vmess',
      tag: 'outbound-vmess',
      settings: {
        vnext: [{
          address: '1.2.3.4',
          port: 443,
          users: [{ id: '11111111-2222-4333-8444-555555555555', security: 'auto' }],
        }],
      },
    });
  });

  it('vless preserves flat shape and emits reverse only when reverseTag is set', () => {
    const wire = {
      protocol: 'vless',
      tag: 'out-vless',
      settings: {
        address: 'srv.example',
        port: 8443,
        id: '11111111-2222-4333-8444-555555555555',
        flow: 'xtls-rprx-vision',
        encryption: 'none',
      },
    };
    const form = rawOutboundToFormValues(wire);
    expect(form.protocol).toBe('vless');
    if (form.protocol === 'vless') {
      expect(form.settings.reverseTag).toBe('');
    }
    const back = formValuesToWirePayload(form);
    expect(back.settings).not.toHaveProperty('reverse');
    expect(back.settings).toMatchObject({
      address: 'srv.example',
      port: 8443,
      id: '11111111-2222-4333-8444-555555555555',
      flow: 'xtls-rprx-vision',
      encryption: 'none',
    });
  });

  it('vless emits reverse + sniffing when reverseTag is set', () => {
    const wire = {
      protocol: 'vless',
      settings: {
        address: 'srv',
        port: 8443,
        id: '11111111-2222-4333-8444-555555555555',
        flow: '',
        encryption: 'none',
        reverse: { tag: 'rev-1', sniffing: { enabled: true, destOverride: ['tls'] } },
      },
    };
    const form = rawOutboundToFormValues(wire);
    if (form.protocol === 'vless') {
      expect(form.settings.reverseTag).toBe('rev-1');
      expect(form.settings.reverseSniffing.enabled).toBe(true);
      expect(form.settings.reverseSniffing.destOverride).toEqual(['tls']);
    }
    const back = formValuesToWirePayload(form);
    const settings = back.settings as Record<string, unknown>;
    expect(settings.reverse).toMatchObject({ tag: 'rev-1' });
  });

  it('vless does not emit testpre/testseed unless flow is vision', () => {
    const wire = {
      protocol: 'vless',
      settings: {
        address: 'srv', port: 443, id: '11111111-2222-4333-8444-555555555555',
        flow: '', encryption: 'none', testpre: 5, testseed: [1, 2, 3, 4],
      },
    };
    const back = formValuesToWirePayload(rawOutboundToFormValues(wire));
    expect(back.settings).not.toHaveProperty('testpre');
    expect(back.settings).not.toHaveProperty('testseed');
  });

  it('trojan flattens servers[0] and re-nests', () => {
    const wire = {
      protocol: 'trojan',
      settings: { servers: [{ address: 's', port: 443, password: 'pw' }] },
    };
    const form = rawOutboundToFormValues(wire);
    if (form.protocol === 'trojan') {
      expect(form.settings).toEqual({ address: 's', port: 443, password: 'pw' });
    }
    expect(formValuesToWirePayload(form).settings).toEqual({
      servers: [{ address: 's', port: 443, password: 'pw' }],
    });
  });

  it('shadowsocks preserves uot + UoTVersion', () => {
    const wire = {
      protocol: 'shadowsocks',
      settings: {
        servers: [{
          address: 's', port: 443, password: 'pw',
          method: '2022-blake3-aes-128-gcm', uot: true, UoTVersion: 2,
        }],
      },
    };
    const back = formValuesToWirePayload(rawOutboundToFormValues(wire));
    expect(back.settings).toMatchObject({
      servers: [{ uot: true, UoTVersion: 2 }],
    });
  });

  it('socks emits users:[] when user is empty, users:[{...}] when set', () => {
    const noUser = formValuesToWirePayload(rawOutboundToFormValues({
      protocol: 'socks',
      settings: { servers: [{ address: 's', port: 1080 }] },
    }));
    expect(noUser.settings).toMatchObject({ servers: [{ users: [] }] });

    const withUser = formValuesToWirePayload(rawOutboundToFormValues({
      protocol: 'socks',
      settings: { servers: [{ address: 's', port: 1080, users: [{ user: 'u', pass: 'p' }] }] },
    }));
    expect(withUser.settings).toMatchObject({
      servers: [{ users: [{ user: 'u', pass: 'p' }] }],
    });
  });

  it('wireguard csv-joins address and reserved on read, splits on write', () => {
    const wire = {
      protocol: 'wireguard',
      settings: {
        mtu: 1420,
        secretKey: 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=',
        address: ['10.0.0.1', 'fd00::1'],
        workers: 2,
        peers: [{ publicKey: 'pk', allowedIPs: ['0.0.0.0/0'], endpoint: 'e:51820', preSharedKey: 'psk' }],
        reserved: [1, 2, 3],
        noKernelTun: false,
      },
    };
    const form = rawOutboundToFormValues(wire);
    if (form.protocol === 'wireguard') {
      expect(form.settings.address).toBe('10.0.0.1,fd00::1');
      expect(form.settings.reserved).toBe('1,2,3');
      expect(form.settings.peers[0].psk).toBe('psk');
    }
    const back = formValuesToWirePayload(form);
    expect(back.settings).toMatchObject({
      address: ['10.0.0.1', 'fd00::1'],
      reserved: [1, 2, 3],
      peers: [{ preSharedKey: 'psk' }],
    });
  });

  it('blackhole wraps type into {response:{type}} and omits when empty', () => {
    const empty = formValuesToWirePayload(rawOutboundToFormValues({
      protocol: 'blackhole',
      settings: {},
    }));
    expect(empty.settings).toEqual({ response: undefined });

    const withType = formValuesToWirePayload(rawOutboundToFormValues({
      protocol: 'blackhole',
      settings: { response: { type: 'http' } },
    }));
    expect(withType.settings).toEqual({ response: { type: 'http' } });
  });

  it('dns rules normalize qtype numeric strings and split domains', () => {
    const wire = {
      protocol: 'dns',
      settings: {
        rewriteNetwork: 'udp',
        rewriteAddress: '1.1.1.1',
        rewritePort: 53,
        rules: [
          { action: 'direct', qtype: 'A,AAAA', domain: ['example.com', 'ext.org'] },
          { action: 'reject', qtype: 28, domain: 'blocked.com' },
        ],
      },
    };
    const back = formValuesToWirePayload(rawOutboundToFormValues(wire));
    const settings = back.settings as Record<string, unknown>;
    const rules = settings.rules as Array<Record<string, unknown>>;
    expect(rules[0]).toEqual({ action: 'direct', qtype: 'A,AAAA', domain: ['example.com', 'ext.org'] });
    expect(rules[1]).toEqual({ action: 'reject', qtype: 28, domain: ['blocked.com'] });
  });

  it('freedom emits domainStrategy/redirect/fragment conditionally', () => {
    const empty = formValuesToWirePayload(rawOutboundToFormValues({
      protocol: 'freedom',
      settings: {},
    }));
    expect(empty.settings).toEqual({
      domainStrategy: undefined,
      redirect: undefined,
      fragment: undefined,
      noises: undefined,
      finalRules: undefined,
    });

    const filled = formValuesToWirePayload(rawOutboundToFormValues({
      protocol: 'freedom',
      settings: {
        domainStrategy: 'UseIPv4',
        redirect: '1.1.1.1',
        fragment: { packets: 'tlshello', length: '100-200' },
      },
    }));
    expect(filled.settings).toMatchObject({
      domainStrategy: 'UseIPv4',
      redirect: '1.1.1.1',
      fragment: { packets: 'tlshello', length: '100-200' },
    });
  });

  it('mux is only emitted when enabled AND protocol/network/flow allow it', () => {
    // Disabled mux: omitted
    const disabled = formValuesToWirePayload(rawOutboundToFormValues({
      protocol: 'vless',
      settings: { address: 's', port: 443, id: '11111111-2222-4333-8444-555555555555', flow: '', encryption: 'none' },
      mux: { enabled: false },
    }));
    expect(disabled).not.toHaveProperty('mux');

    // Enabled mux on vless without flow: emitted
    const enabled = formValuesToWirePayload(rawOutboundToFormValues({
      protocol: 'vless',
      settings: { address: 's', port: 443, id: '11111111-2222-4333-8444-555555555555', flow: '', encryption: 'none' },
      mux: { enabled: true, concurrency: 8, xudpConcurrency: 16, xudpProxyUDP443: 'reject' },
    }));
    expect(enabled.mux).toMatchObject({ enabled: true });

    // Enabled mux on vless with vision flow: gated out
    const withFlow = formValuesToWirePayload(rawOutboundToFormValues({
      protocol: 'vless',
      settings: { address: 's', port: 443, id: '11111111-2222-4333-8444-555555555555', flow: 'xtls-rprx-vision', encryption: 'none' },
      mux: { enabled: true },
    }));
    expect(withFlow).not.toHaveProperty('mux');

    // Freedom (non-mux protocol): gated out even if enabled
    const freedom = formValuesToWirePayload(rawOutboundToFormValues({
      protocol: 'freedom',
      settings: {},
      mux: { enabled: true },
    }));
    expect(freedom).not.toHaveProperty('mux');
  });

  it('hysteria preserves address/port/version literal 2', () => {
    const back = formValuesToWirePayload(rawOutboundToFormValues({
      protocol: 'hysteria',
      settings: { address: 'h.example', port: 8443, version: 2 },
    }));
    expect(back.settings).toEqual({ address: 'h.example', port: 8443, version: 2 });
  });

  it('loopback inboundTag round-trips', () => {
    const back = formValuesToWirePayload(rawOutboundToFormValues({
      protocol: 'loopback',
      settings: { inboundTag: 'tagged-inbound' },
    }));
    expect(back.settings).toEqual({ inboundTag: 'tagged-inbound' });
  });

  it('unknown protocol falls back to vless without throwing', () => {
    const form = rawOutboundToFormValues({ protocol: 'mysterious', settings: {} });
    expect(form.protocol).toBe('vless');
  });
});
