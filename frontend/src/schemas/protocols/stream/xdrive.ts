import { z } from 'zod';

// Both XDRIVE ends carry the same service, folder and secrets; an absent tuning knob takes
// the core's default (segmentBytes and concurrency are capped there at 16 MiB / 64).
export const XDriveServiceSchema = z.enum(['google-drive', 'local', 'template']);
export type XDriveService = z.infer<typeof XDriveServiceSchema>;

// xray-core 26.10.17 (#7182) renamed "Google Drive" and reads the old name only as a deprecated alias.
const canonicalXDriveService = (value: unknown) =>
  value === 'Google Drive' ? 'google-drive' : value;

const XDriveServiceInputSchema = z.preprocess(canonicalXDriveService, XDriveServiceSchema);

/** Lifts a stored stream's legacy service name before an editor reads it unparsed. */
export function liftLegacyXDriveService(stream: Record<string, unknown>): void {
  const xdrive = stream.xdriveSettings;
  if (!xdrive || typeof xdrive !== 'object' || Array.isArray(xdrive)) return;
  const settings = xdrive as Record<string, unknown>;
  const service = canonicalXDriveService(settings.service);
  if (service !== settings.service) stream.xdriveSettings = { ...settings, service };
}

const MillisSchema = z.number().int().min(0);

export const XDriveStreamSettingsSchema = z.object({
  service: XDriveServiceInputSchema.default('google-drive'),
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
  // Client XMUX (xray-core 26.10.21); left unset, the core sizes it per service, so no
  // defaults are filled in here. A JSON subscription hands an inbound's value to clients.
  xmux: z.record(z.string(), z.unknown()).optional(),
});
export type XDriveStreamSettings = z.infer<typeof XDriveStreamSettingsSchema>;
