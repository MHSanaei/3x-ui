import { TlsStreamSettingsSchema } from '@/schemas/protocols/security/tls';

function defaultCertificate(): Record<string, unknown> {
  return {
    useFile: true,
    certificateFile: '',
    keyFile: '',
    certificate: [],
    key: [],
    ocspStapling: 3600,
    oneTimeLoading: false,
    usage: 'encipherment',
    buildChain: false,
  };
}

export function createTlsSettingsWithDefaultCert(): Record<string, unknown> {
  const tls = TlsStreamSettingsSchema.parse({}) as Record<string, unknown>;
  tls.certificates = [defaultCertificate()];
  return tls;
}

export function createHysteriaTlsSettingsWithDefaultCert(): Record<string, unknown> {
  const tls = createTlsSettingsWithDefaultCert();
  tls.alpn = ['h3'];

  const settings = tls.settings && typeof tls.settings === 'object' && !Array.isArray(tls.settings)
    ? { ...(tls.settings as Record<string, unknown>) }
    : {};
  settings.fingerprint = '';
  tls.settings = settings;

  return tls;
}
