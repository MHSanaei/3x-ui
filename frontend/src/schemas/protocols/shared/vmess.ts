import { z } from 'zod';

const VmessSecurityEnum = z.enum([
  'aes-128-gcm',
  'chacha20-poly1305',
  'auto',
  'none',
  'zero',
]);

// Legacy rows persisted `security: ""` (especially on VMess inbounds
// created before the enum was nailed down). Preprocess maps the empty
// string back to the documented default so existing data parses cleanly
// — subsequent writes serialize the normalized value.
export const VmessSecuritySchema = z.preprocess(
  (val) => (val === '' ? 'auto' : val),
  VmessSecurityEnum,
);
export type VmessSecurity = z.infer<typeof VmessSecurityEnum>;
