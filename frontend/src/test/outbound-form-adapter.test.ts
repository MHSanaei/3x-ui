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

  it('vless preserves a non-none encryption value (post-quantum)', () => {
    const enc = 'mlkem768x25519plus.native.0rtt.G3cdPSd1-NnlpTbWNSM5vHsT5VNzWfFzYSKwbUMnV1Y';
    const wire = {
      protocol: 'vless',
      settings: {
        address: 'srv',
        port: 443,
        id: '11111111-2222-4333-8444-555555555555',
        flow: '',
        encryption: enc,
      },
    };
    const form = rawOutboundToFormValues(wire);
    if (form.protocol === 'vless') {
      expect(form.settings.encryption).toBe(enc);
    }
    expect((formValuesToWirePayload(form).settings as Record<string, unknown>).encryption).toBe(enc);
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

  it('dns rules normalize qType numeric strings, split domains, carry rCode', () => {
    const wire = {
      protocol: 'dns',
      settings: {
        rewriteNetwork: 'udp',
        rewriteAddress: '1.1.1.1',
        rewritePort: 53,
        rules: [
          { action: 'direct', qType: 'A,AAAA', domain: ['example.com', 'ext.org'] },
          { action: 'return', qType: 28, domain: 'blocked.com', rCode: 3 },
        ],
      },
    };
    const back = formValuesToWirePayload(rawOutboundToFormValues(wire));
    const settings = back.settings as Record<string, unknown>;
    const rules = settings.rules as Array<Record<string, unknown>>;
    expect(rules[0]).toEqual({ action: 'direct', qType: 'A,AAAA', domain: ['example.com', 'ext.org'] });
    expect(rules[1]).toEqual({ action: 'return', qType: 28, domain: ['blocked.com'], rCode: 3 });
  });

  it('dns rules read the legacy qtype wire key for back-compat', () => {
    const wire = {
      protocol: 'dns',
      settings: { rules: [{ action: 'direct', qtype: 'TXT' }] },
    };
    const back = formValuesToWirePayload(rawOutboundToFormValues(wire));
    const rules = (back.settings as Record<string, unknown>).rules as Array<Record<string, unknown>>;
    expect(rules[0]).toEqual({ action: 'direct', qType: 'TXT' });
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
        userLevel: 3,
        proxyProtocol: 2,
        fragment: { packets: 'tlshello', length: '100-200' },
        noises: [{ type: 'rand', packet: '10-20', delay: '10-16', applyTo: 'ipv4' }],
      },
    }));
    expect(filled.settings).toMatchObject({
      domainStrategy: 'UseIPv4',
      redirect: '1.1.1.1',
      userLevel: 3,
      proxyProtocol: 2,
      fragment: { packets: 'tlshello', length: '100-200' },
      noises: [{ type: 'rand', packet: '10-20', delay: '10-16', applyTo: 'ipv4' }],
    });
  });

  it('freedom tolerates settings without a fragment object (issue #4686)', () => {
    const values = {
      protocol: 'freedom',
      tag: 'direct',
      settings: {
        domainStrategy: '',
        redirect: '',
        proxyProtocol: 0,
        noises: [],
        finalRules: [
          { action: 'block', network: '', port: '', ip: ['geoip:private'], blockDelay: '' },
        ],
      },
    } as unknown as Parameters<typeof formValuesToWirePayload>[0];

    expect(() => formValuesToWirePayload(values)).not.toThrow();
    const back = formValuesToWirePayload(values);
    expect((back.settings as { fragment?: unknown }).fragment).toBeUndefined();
    expect((back.settings as { finalRules?: unknown[] }).finalRules).toHaveLength(1);
  });

  it('freedom omits proxyProtocol when disabled (0)', () => {
    const round = formValuesToWirePayload(rawOutboundToFormValues({
      protocol: 'freedom',
      settings: { proxyProtocol: 0 },
    }));
    expect((round.settings as { proxyProtocol?: number }).proxyProtocol).toBeUndefined();
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

describe('outbound-form-adapter: xhttp xmux toggle', () => {
  const xmuxWire = {
    protocol: 'vless',
    tag: 'out-xhttp',
    settings: {
      address: 's', port: 443, id: '11111111-2222-4333-8444-555555555555',
      flow: '', encryption: 'none',
    },
    streamSettings: {
      network: 'xhttp',
      security: 'none',
      xhttpSettings: {
        path: '/', host: '', mode: '',
        xPaddingBytes: '100-1000', scMaxEachPostBytes: '1000000',
        xmux: { maxConcurrency: '11', maxConnections: '1', hMaxRequestTimes: '1', hMaxReusableSecs: '1' },
      },
    },
  };

  it('derives enableXmux from a saved xmux object and backfills missing knobs', () => {
    const form = rawOutboundToFormValues(xmuxWire);
    const stream = form.streamSettings as Record<string, unknown>;
    const xhttp = stream.xhttpSettings as Record<string, unknown>;
    expect(xhttp.enableXmux).toBe(true);
    expect(xhttp.xmux).toMatchObject({
      maxConcurrency: '11',
      maxConnections: '1',
      hMaxRequestTimes: '1',
      hMaxReusableSecs: '1',
      cMaxReuseTimes: 0,
      hKeepAlivePeriod: 0,
    });
  });

  it('round-trips xmux on save, strips enableXmux, and enforces xmux exclusivity', () => {
    const back = formValuesToWirePayload(rawOutboundToFormValues(xmuxWire));
    const xhttp = (back.streamSettings as Record<string, unknown>).xhttpSettings as Record<string, unknown>;
    expect(xhttp).not.toHaveProperty('enableXmux');
    const xmux = xhttp.xmux as Record<string, unknown>;
    // xray-core rejects maxConnections + maxConcurrency together; the
    // explicit maxConnections wins and maxConcurrency is dropped.
    expect(xmux).not.toHaveProperty('maxConcurrency');
    expect(xmux).toMatchObject({ maxConnections: '1', hMaxRequestTimes: '1', hMaxReusableSecs: '1' });
  });

  it('drops xmux on save when the toggle is off', () => {
    const form = rawOutboundToFormValues(xmuxWire);
    const xhttp = (form.streamSettings as Record<string, unknown>).xhttpSettings as Record<string, unknown>;
    xhttp.enableXmux = false;
    const back = formValuesToWirePayload(form);
    const wireXhttp = (back.streamSettings as Record<string, unknown>).xhttpSettings as Record<string, unknown>;
    expect(wireXhttp).not.toHaveProperty('xmux');
  });
});

describe('outbound-form-adapter: full optional-block round-trip', () => {
  const wire = {
    protocol: 'vless',
    settings: {
      address: '1', port: 443, id: '1', flow: '', encryption: 'none',
      reverse: {
        tag: '1',
        sniffing: {
          enabled: true,
          destOverride: ['http', 'tls', 'quic', 'fakedns'],
          metadataOnly: true,
          routeOnly: true,
          ipsExcluded: ['1'],
          domainsExcluded: ['1'],
        },
      },
    },
    tag: '1',
    streamSettings: {
      network: 'tcp',
      tcpSettings: { header: { type: 'http', request: { version: '1.1', method: 'GET', path: ['/'], headers: { '1': ['1'] } }, response: { version: '1.1', status: '200', reason: 'OK', headers: { '1': ['1'] } } } },
      security: 'none',
      sockopt: { tcpFastOpen: true, customSockopt: [{ type: 'int', level: '6', opt: '1', value: '1' }] },
      finalmask: { tcp: [{ type: 'fragment', settings: { packets: '1-3', length: '1', delay: '1', maxSplit: '1' } }] },
    },
    sendThrough: '1',
    mux: { enabled: true, concurrency: 8, xudpConcurrency: 16, xudpProxyUDP443: 'reject' },
  };

  it('preserves sockopt, finalmask, mux, and reverse excludes', () => {
    const back = formValuesToWirePayload(rawOutboundToFormValues(wire));
    const settings = back.settings as Record<string, unknown>;
    const sniffing = (settings.reverse as Record<string, unknown>).sniffing as Record<string, unknown>;
    expect(sniffing.ipsExcluded).toEqual(['1']);
    expect(sniffing.domainsExcluded).toEqual(['1']);

    const stream = back.streamSettings as Record<string, unknown>;
    expect(stream.sockopt).toMatchObject({ tcpFastOpen: true });
    expect((stream.sockopt as Record<string, unknown>).customSockopt).toHaveLength(1);
    expect(stream.finalmask).toMatchObject({ tcp: [{ type: 'fragment' }] });

    expect(back.mux).toMatchObject({ enabled: true });
  });
});
