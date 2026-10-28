import { describe, expect, it } from 'vitest';

import { rawInboundToFormValues, formValuesToWirePayload } from '@/lib/xray/inbound-form-adapter';
import { genTuicLink } from '@/lib/xray/inbound-link';
import { InboundFormSchema } from '@/schemas/forms/inbound-form';
import { InboundSchema } from '@/schemas/api/inbound';
import { buildTuicClientConfig } from '@/pages/clients/tuicConfig';

const uuid = '11111111-1111-1111-1111-111111111111';
const client = { id: 1, email: 'user@test', uuid, password: 'secret', inboundIds: [1] };

describe('TUIC client TLS options', () => {
  it.each([
    { name: 'defaults', settings: {}, zeroRTT: '1', disableSni: false },
    {
      name: 'nested',
      settings: { server: { disable_sni: true, zero_rtt_handshake: false } },
      zeroRTT: '0',
      disableSni: true,
    },
    {
      name: 'legacy',
      settings: { disable_sni: true, zero_rtt_handshake: false },
      zeroRTT: '0',
      disableSni: true,
    },
    {
      name: 'nested false overrides legacy',
      settings: {
        disable_sni: true,
        zero_rtt_handshake: false,
        server: { disable_sni: false, zero_rtt_handshake: true },
      },
      zeroRTT: '1',
      disableSni: false,
    },
  ])('preserves $name through form save, link and YAML export', (tc) => {
    const raw = { id: 1, protocol: 'tuic', port: 8443, settings: tc.settings };
    const form = InboundFormSchema.parse(rawInboundToFormValues(raw));
    const saved = formValuesToWirePayload(form);
    const inbound = InboundSchema.parse({
      protocol: 'tuic',
      port: 8443,
      settings: JSON.parse(saved.settings),
    });
    const link = new URL(
      genTuicLink({
        inbound,
        address: 'proxy.example',
        clientUuid: uuid,
        clientPassword: client.password,
        externalProxy: {
          dest: 'edge.example',
          port: 9443,
          sni: 'verify.example',
          remark: 'Edge',
          forceTls: 'same',
        },
      }),
    );
    expect(link.searchParams.get('zero_rtt_handshake')).toBe(tc.zeroRTT);
    expect(link.searchParams.get('disable_sni')).toBe(tc.disableSni ? '1' : null);
    expect(link.searchParams.get('sni')).toBe('verify.example');
    expect(link.searchParams.get('allow_insecure')).toBe('0');

    const settings = JSON.parse(saved.settings);
    const yaml = buildTuicClientConfig(
      client,
      { id: 1, protocol: 'tuic', port: 8443, tuicServer: settings.server },
      'proxy.example',
      '',
      { dest: 'edge.example', port: 9443, sni: 'verify.example', remark: 'Edge' },
    );
    expect(yaml).toContain(`reduce-rtt: ${tc.zeroRTT === '1'}`);
    expect(yaml.includes('disable-sni: true')).toBe(tc.disableSni);
    expect(yaml).toContain('sni: verify.example');
    expect(yaml).not.toContain('skip-cert-verify: true');
    if (tc.disableSni) expect(yaml).toContain('name-cert-verify: verify.example');
  });
});
