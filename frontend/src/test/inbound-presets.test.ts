/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import { rawInboundToFormValues } from '@/lib/xray/inbound-form-adapter';
import { genVlessLink } from '@/lib/xray/inbound-link';
import { INBOUND_PRESETS, applyPresetSecrets, getPreset } from '@/lib/xray/inbound-presets';
import { InboundFormSchema } from '@/schemas/forms/inbound-form';
import type { Inbound } from '@/schemas/api/inbound';

// Every preset must produce a row that, once mapped to InboundFormValues,
// passes InboundFormSchema — the exact gate the modal's submit() runs before
// POSTing. If a preset ever drifts out of schema shape this fails loudly
// instead of silently rejecting the operator's one-click create.

describe('inbound presets', () => {
  for (const preset of INBOUND_PRESETS) {
    it(`${preset.id} builds a schema-valid inbound`, () => {
      const domain = preset.needsDomain ? 'example.com' : undefined;
      const values = rawInboundToFormValues(preset.build(domain));
      const parsed = InboundFormSchema.safeParse(values);
      if (!parsed.success) {
        throw new Error(`${preset.id} failed: ${JSON.stringify(parsed.error.issues, null, 2)}`);
      }
      expect(parsed.success).toBe(true);
    });

    it(`${preset.id} seeds exactly one client`, () => {
      const row = preset.build('example.com');
      const settings = row.settings as { clients?: unknown[] };
      expect(Array.isArray(settings.clients)).toBe(true);
      expect(settings.clients).toHaveLength(1);
    });
  }

  it('reality presets carry a target and shortIds but leave keys empty', () => {
    const preset = getPreset('vless-reality-vision')!;
    const stream = preset.build().streamSettings as {
      realitySettings: { target: string; shortIds: string[]; privateKey: string };
    };
    expect(stream.realitySettings.target).not.toBe('');
    expect(stream.realitySettings.shortIds.length).toBeGreaterThan(0);
    // Keys are fetched from the panel after apply, not baked into the preset.
    expect(stream.realitySettings.privateKey).toBe('');
  });

  it('TLS presets thread the domain into serverName', () => {
    const preset = getPreset('trojan-tls')!;
    const stream = preset.build('my.host.example').streamSettings as {
      tlsSettings: { serverName: string };
    };
    expect(stream.tlsSettings.serverName).toBe('my.host.example');
  });

  it('vision preset uses xtls-rprx-vision flow, grpc preset does not', () => {
    const vision = getPreset('vless-reality-vision')!.build().settings as {
      clients: { flow: string }[];
    };
    const grpc = getPreset('vless-reality-grpc')!.build().settings as {
      clients: { flow: string }[];
    };
    expect(vision.clients[0].flow).toBe('xtls-rprx-vision');
    expect(grpc.clients[0].flow).toBe('');
  });

  it('applyPresetSecrets injects reality keys for a reality preset', () => {
    const row = getPreset('vless-reality-vision')!.build();
    applyPresetSecrets(row, { realityPrivateKey: 'PRIV', realityPublicKey: 'PUB' });
    const stream = row.streamSettings as {
      realitySettings: { privateKey: string; settings: { publicKey: string } };
    };
    expect(stream.realitySettings.privateKey).toBe('PRIV');
    expect(stream.realitySettings.settings.publicKey).toBe('PUB');
  });

  it('applyPresetSecrets stamps a shared subId on every client', () => {
    const row = getPreset('vless-reality-vision')!.build();
    applyPresetSecrets(row, { subId: 'SHARED123' });
    const clients = (row.settings as { clients: { subId: string }[] }).clients;
    expect(clients.every((c) => c.subId === 'SHARED123')).toBe(true);
  });

  it('applyPresetSecrets injects panel cert + domain for a TLS preset', () => {
    const row = getPreset('trojan-tls')!.build();
    applyPresetSecrets(row, { certFile: '/c/fullchain.pem', keyFile: '/c/privkey.pem', domain: 'my.host' });
    const tls = (row.streamSettings as { tlsSettings: {
      serverName: string; certificates: { certificateFile: string; keyFile: string }[];
    } }).tlsSettings;
    expect(tls.serverName).toBe('my.host');
    expect(tls.certificates[0].certificateFile).toBe('/c/fullchain.pem');
    expect(tls.certificates[0].keyFile).toBe('/c/privkey.pem');
    // The row still maps to a schema-valid form value.
    expect(InboundFormSchema.safeParse(rawInboundToFormValues(row)).success).toBe(true);
  });

  // Regression: the Reality share link MUST carry sni — without it the
  // client handshakes with an empty SNI and the server rejects it (the
  // "imported into v2rayN, shows -1" bug). The sni must match the preset's
  // serverName and must not leak a :port.
  for (const id of ['vless-reality-vision', 'vless-reality-grpc'] as const) {
    it(`${id} share link carries sni matching serverName`, () => {
      const preset = getPreset(id)!;
      const row = preset.build();
      const values = rawInboundToFormValues(row) as unknown as Inbound;
      const stream = row.streamSettings as {
        realitySettings: { serverNames: string[]; settings: { publicKey: string } };
      };
      // Simulate the panel injecting the fetched public key post-apply.
      stream.realitySettings.settings.publicKey = 'TESTPUBKEY';
      const client = (row.settings as { clients: { id: string; flow: string }[] }).clients[0];

      const link = genVlessLink({
        inbound: values,
        address: 'vps.example.com',
        clientId: client.id,
        flow: client.flow as never,
        remark: 'preset',
      });

      const sni = new URL(link).searchParams.get('sni');
      const expected = stream.realitySettings.serverNames[0];
      expect(sni).toBe(expected);
      expect(sni).not.toContain(':');
      expect(new URL(link).searchParams.get('pbk')).toBe('TESTPUBKEY');
    });
  }
});

