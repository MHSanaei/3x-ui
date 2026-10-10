import { TlsStreamSettingsSchema } from '@/schemas/protocols/security/tls';

function defaultCertificate(): Record<string, unknown> {
  return {
    useFile: true,
    certificateFile: '',
    keyFile: '',
    certificate: [],
    key: [],
    ocspStapling: 0,
    oneTimeLoading: false,
    usage: 'encipherment',
    buildChain: false,
  };
}

export function createTlsSettingsWithDefaultCert(network?: string): Record<string, unknown> {
  const tls = TlsStreamSettingsSchema.parse({}) as Record<string, unknown>;
  tls.certificates = [defaultCertificate()];
  const settings =
    tls.settings && typeof tls.settings === 'object' && !Array.isArray(tls.settings)
      ? { ...(tls.settings as Record<string, unknown>) }
      : {};
  settings.fingerprint = 'chrome';
  tls.settings = settings;
  /* WebSocket bootstraps over HTTP/1.1: the schema default ALPN
   * ['h2','http/1.1'] makes the server negotiate h2 (see issue #6782). */
  if (network === 'ws') {
    tls.alpn = ['http/1.1'];
  }
  return tls;
}

export function createHysteriaTlsSettingsWithDefaultCert(): Record<string, unknown> {
  const tls = createTlsSettingsWithDefaultCert();
  tls.alpn = ['h3'];

  const settings =
    tls.settings && typeof tls.settings === 'object' && !Array.isArray(tls.settings)
      ? { ...(tls.settings as Record<string, unknown>) }
      : {};
  settings.fingerprint = '';
  tls.settings = settings;

  return tls;
}
