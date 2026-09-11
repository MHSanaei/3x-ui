import { describe, expect, it } from 'vitest';

import { InboundFormSchema, InboundStreamFormSchema } from '@/schemas/forms/inbound-form';
import { TlsCertSchema, TlsStreamSettingsSchema } from '@/schemas/protocols/security';
import { createTlsSettingsWithDefaultCert } from '@/lib/xray/inbound-tls-defaults';
import { formValuesToWirePayload } from '@/lib/xray/inbound-form-adapter';
import { inboundFromDb } from '@/lib/xray/inbound-from-db';

const fileCert = { certificateFile: '/cert/server.pem', keyFile: '/cert/server.key' };
const inlineCert = { certificate: ['certificate content'], key: ['private key content'] };

function parseCertificates(certificates?: unknown[]) {
  return InboundFormSchema.safeParse({
    port: 443,
    protocol: 'vless',
    settings: { clients: [] },
    streamSettings: {
      network: 'tcp',
      tcpSettings: {},
      security: 'tls',
      tlsSettings: { certificates },
    },
  });
}

describe('inbound TLS certificate validation', () => {
  it('rejects the empty certificate seeded by the TLS editor with a useful field error', () => {
    const result = parseCertificates(createTlsSettingsWithDefaultCert().certificates as unknown[]);
    expect(result.success).toBe(false);
    if (result.success) return;
    expect(result.error.issues[0]).toMatchObject({
      path: ['streamSettings', 'tlsSettings', 'certificates', 0, 'certificateFile'],
      message: 'pages.inbounds.form.tlsCertificateRequired',
    });
  });

  it.each([
    ['missing certificates', undefined],
    ['empty certificates', []],
    ['empty row', [{}]],
    ['blank paths', [{ certificateFile: '  ', keyFile: '\t' }]],
    ['certificate path only', [{ certificateFile: fileCert.certificateFile }]],
    ['private key path only', [{ keyFile: fileCert.keyFile }]],
    ['empty content', [{ useFile: false, certificate: [], key: [] }]],
    ['blank content', [{ useFile: false, certificate: [' ', '\n'], key: ['\t'] }]],
    ['certificate content only', [{ useFile: false, certificate: inlineCert.certificate }]],
    ['private key content only', [{ useFile: false, key: inlineCert.key }]],
    ['empty file mode with stale inline content', [{ useFile: true, ...inlineCert }]],
    ['empty content mode with stale file paths', [{ useFile: false, ...fileCert }]],
    ['valid certificate followed by an empty row', [fileCert, {}]],
    ['verify certificate only', [{ certificateFile: '/ca.pem', usage: 'verify' }]],
    ['issue certificate without its key', [{ certificate: ['CA'], usage: 'issue' }]],
    ['empty verify certificate alongside server certificate', [fileCert, { usage: 'verify' }]],
  ])('rejects %s', (_name, certificates) => {
    expect(parseCertificates(certificates as unknown[] | undefined).success).toBe(false);
  });

  it.each([
    ['file certificate', [fileCert]],
    ['inline certificate', [inlineCert]],
    ['explicit file mode', [{ useFile: true, ...fileCert }]],
    ['explicit inline mode', [{ useFile: false, ...inlineCert }]],
    ['multiple certificates', [fileCert, inlineCert]],
    ['issuing CA with its key', [{ ...inlineCert, usage: 'issue' }]],
    [
      'file verification CA without a key',
      [fileCert, { certificateFile: '/ca.pem', usage: 'verify' }],
    ],
    [
      'inline verification CA without a key',
      [inlineCert, { certificate: ['CA'], usage: 'verify' }],
    ],
  ])('accepts %s', (_name, certificates) => {
    expect(parseCertificates(certificates).success).toBe(true);
  });

  it.each([true, false])('serializes only the selected mode (useFile=%s)', (useFile) => {
    const result = parseCertificates([{ useFile, ...fileCert, ...inlineCert }]);
    expect(result.success).toBe(true);
    if (!result.success) return;
    const stream = JSON.parse(formValuesToWirePayload(result.data).streamSettings);
    const cert = stream.tlsSettings.certificates[0];
    expect(cert).toMatchObject(useFile ? fileCert : inlineCert);
    expect(cert).not.toHaveProperty('useFile');
    expect(cert).not.toHaveProperty(useFile ? 'certificate' : 'certificateFile');
    expect(cert).not.toHaveProperty(useFile ? 'key' : 'keyFile');
  });

  it.each([
    ['file', { certificateFile: '/cert/ca.pem', usage: 'verify' }],
    ['inline', { certificate: ['CA certificate'], usage: 'verify' }],
  ])('preserves TLS settings when reading back a %s verification CA without a key', (_mode, ca) => {
    const tlsSettings = {
      serverName: 'tls.example.test',
      alpn: ['h3'],
      certificates: [fileCert, ca],
      settings: { fingerprint: 'firefox', pinnedPeerCertSha256: ['test-pin'] },
    };
    const values = InboundFormSchema.parse({
      port: 443,
      protocol: 'vless',
      settings: { clients: [] },
      streamSettings: { network: 'tcp', tcpSettings: {}, security: 'tls', tlsSettings },
    });

    const restored = inboundFromDb(formValuesToWirePayload(values));

    expect(restored.streamSettings).toMatchObject({ security: 'tls', tlsSettings });
  });

  it.each([undefined, 'encipherment', 'issue'])(
    'keeps wire private keys required for usage=%s',
    (usage) => {
      expect(TlsCertSchema.safeParse({ certificateFile: '/cert.pem', usage }).success).toBe(false);
      expect(
        TlsCertSchema.safeParse({ certificateFile: '/cert.pem', keyFile: '', usage }).success,
      ).toBe(false);
      expect(TlsCertSchema.safeParse({ certificate: ['certificate'], usage }).success).toBe(false);
    },
  );

  it('applies the same certificate requirement to Hysteria TLS', () => {
    const stream = {
      network: 'hysteria',
      hysteriaSettings: {},
      security: 'tls',
      tlsSettings: createTlsSettingsWithDefaultCert(),
    };
    expect(InboundStreamFormSchema.safeParse(stream).success).toBe(false);
    expect(
      InboundStreamFormSchema.safeParse({ ...stream, tlsSettings: { certificates: [fileCert] } })
        .success,
    ).toBe(true);
  });

  it('keeps Reality, unsecured, transportless and outbound TLS certificate-free', () => {
    for (const security of [{ security: 'reality', realitySettings: {} }, { security: 'none' }]) {
      expect(
        InboundStreamFormSchema.safeParse({ network: 'tcp', tcpSettings: {}, ...security }).success,
      ).toBe(true);
    }
    expect(InboundStreamFormSchema.safeParse({}).success).toBe(true);
    expect(TlsStreamSettingsSchema.safeParse({}).success).toBe(true);
  });
});
