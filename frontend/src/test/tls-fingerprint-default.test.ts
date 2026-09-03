/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import { TlsStreamSettingsSchema } from '@/schemas/protocols/security/tls';
import {
  createTlsSettingsWithDefaultCert,
  createHysteriaTlsSettingsWithDefaultCert,
} from '@/lib/xray/inbound-tls-defaults';
import { genHysteriaLink } from '@/lib/xray/inbound-link';
import type { Inbound } from '@/schemas/api/inbound';

// uTLS None ('') must survive a schema parse; the old default flipped it to
// chrome on every save.
describe('TlsClientSettingsSchema fingerprint default', () => {
  it('parses an omitted fingerprint as None, not chrome', () => {
    const parsed = TlsStreamSettingsSchema.parse({});
    expect(parsed.settings.fingerprint).toBe('');
  });

  it('keeps an explicit empty-string fingerprint through parse', () => {
    const parsed = TlsStreamSettingsSchema.parse({
      settings: {
        fingerprint: '',
        echConfigList: '',
        pinnedPeerCertSha256: [],
        verifyPeerCertByName: '',
      },
    });
    expect(parsed.settings.fingerprint).toBe('');
  });

  it('initializes generic TLS inbounds with chrome fingerprint default', () => {
    const tls = createTlsSettingsWithDefaultCert();
    expect((tls.settings as Record<string, unknown>)?.fingerprint).toBe('chrome');
  });

  it('initializes hysteria TLS inbounds with empty fingerprint default', () => {
    const tls = createHysteriaTlsSettingsWithDefaultCert();
    expect((tls.settings as Record<string, unknown>)?.fingerprint).toBe('');
  });

  it('does not inject fp into the hysteria share link when fingerprint is None', () => {
    const raw = {
      id: 1,
      port: 443,
      protocol: 'hysteria',
      settings: { version: 2, clients: [{ auth: 'secret' }] },
      streamSettings: {
        security: 'tls',
        tlsSettings: {
          serverName: 'hy.test',
          alpn: ['h3'],
          settings: {
            fingerprint: '',
            echConfigList: '',
            pinnedPeerCertSha256: [],
            verifyPeerCertByName: '',
          },
        },
        finalmask: {
          udp: [{ type: 'salamander', settings: { password: 'pw', packetSize: '512-1200' } }],
        },
      },
    };
    const link = genHysteriaLink({
      inbound: raw as unknown as Inbound,
      address: 'example.test',
      remark: 'gecko',
      clientAuth: 'secret',
    });
    expect(link).toContain('obfs=gecko');
    expect(link).toContain('minPacketSize=512');
    expect(link).toContain('maxPacketSize=1200');
    expect(link).not.toContain('fp=');
    expect(link).not.toContain('fm=');
  });
});
