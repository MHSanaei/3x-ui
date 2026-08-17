import { z } from 'zod';

const VmessSecurityEnum = z.enum([
  'aes-128-gcm',
  'chacha20-poly1305',
  'auto',
]);

// Legacy rows persisted `security: ""` (especially on VMess inbounds
// created before the enum was nailed down), and rows predating xray-core
// v26.7.11 may still hold the removed "none"/"zero" values that the core
// now treats as "auto". Preprocess maps all of them to the documented
// default so existing data parses cleanly — subsequent writes serialize
// the normalized value.
export const VmessSecuritySchema = z.preprocess(
  (val) => (val === '' || val === 'none' || val === 'zero' ? 'auto' : val),
  VmessSecurityEnum,
);
export type VmessSecurity = z.infer<typeof VmessSecurityEnum>;
