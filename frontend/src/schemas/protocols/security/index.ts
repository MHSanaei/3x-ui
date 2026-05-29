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
export const SecuritySettingsSchema = z.discriminatedUnion('security', [
  z.object({ security: z.literal('none') }),
  z.object({ security: z.literal('tls'),     tlsSettings:     TlsStreamSettingsSchema }),
  z.object({ security: z.literal('reality'), realitySettings: RealityStreamSettingsSchema }),
]);
export type SecuritySettings = z.infer<typeof SecuritySettingsSchema>;
