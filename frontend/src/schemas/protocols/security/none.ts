import { z } from 'zod';

// `security: 'none'` carries no payload — the streamSettings root just omits
// both `tlsSettings` and `realitySettings`. This empty leaf is kept for
// symmetry so the discriminated union has a branch for every security value.
export const NoneSecuritySettingsSchema = z.object({});
export type NoneSecuritySettings = z.infer<typeof NoneSecuritySettingsSchema>;
