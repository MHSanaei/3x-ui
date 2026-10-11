import { z } from 'zod';

// XDRIVE tunnels the stream through files in shared cloud storage, so both ends
// carry the same service, folder and secrets. Zero/absent tuning knobs take the
// core's defaults; segmentBytes and concurrency are capped there at 16 MiB / 64.
export const XDriveServiceSchema = z.enum(['Google Drive', 'local', 'template']);
export type XDriveService = z.infer<typeof XDriveServiceSchema>;

const MillisSchema = z.number().int().min(0);

export const XDriveStreamSettingsSchema = z.object({
  service: XDriveServiceSchema.default('Google Drive'),
  remoteFolder: z.string().default(''),
  // Google Drive takes exactly three, in order: ClientID, ClientSecret, RefreshToken.
  secrets: z.array(z.string()).default([]),
  segmentBytes: z.number().int().min(0).optional(),
  flushIntervalMs: MillisSchema.optional(),
  pollIntervalMs: MillisSchema.optional(),
  maxPollIntervalMs: MillisSchema.optional(),
  sessionTtlSeconds: z.number().int().min(0).optional(),
  concurrency: z.number().int().min(0).optional(),
  eagerWindowMs: MillisSchema.optional(),
  holeTimeoutMs: MillisSchema.optional(),
  template: z.record(z.string(), z.unknown()).optional(),
});
export type XDriveStreamSettings = z.infer<typeof XDriveStreamSettingsSchema>;
