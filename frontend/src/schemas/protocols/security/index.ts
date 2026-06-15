import { z } from 'zod';

import { RealityStreamSettingsSchema } from './reality';
import { TlsStreamSettingsSchema } from './tls';

export * from './none';
export * from './reality';
export * from './tls';

export const SecuritySchema = z.enum(['none', 'tls', 'reality']);
export type Security = z.infer<typeof SecuritySchema>;

// Tagged-wrapper DU on `security`. Wire shape: when security==='tls' only
// `tlsSettings` is present, when 'reality' only `realitySettings`, when
// 'none' neither key appears. The Xray panel's StreamSettings class emits
// `undefined` for the inactive branch which strips the key during JSON
// serialization, so this DU faithfully describes what's on disk.
//
// Tunnel (dokodemo-door / TProxy) is transportless and may carry only
// `sockopt` — its streamSettings has no `security` key at all. The
// transportless branch accepts that shape, mirroring NetworkSettingsSchema's
// `network: never().optional()` handling. A present-but-invalid security
// still fails both branches so a typo can't slip through.
export const SecuritySettingsSchema = z.union([
  z.discriminatedUnion('security', [
    z.object({ security: z.literal('none') }),
    z.object({ security: z.literal('tls'),     tlsSettings:     TlsStreamSettingsSchema }),
    z.object({ security: z.literal('reality'), realitySettings: RealityStreamSettingsSchema }),
  ]),
  z.object({ security: z.never().optional() }),
]);
export type SecuritySettings = z.infer<typeof SecuritySettingsSchema>;
