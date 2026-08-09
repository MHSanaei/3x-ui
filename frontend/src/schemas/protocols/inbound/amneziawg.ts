import { z } from 'zod';

// AntD InputNumber emits null (not undefined) when the user clears it, and
// the form store hands that null straight to safeParse on submit — a bare
// .optional() would reject it and block the save.
const optionalClearedInt = (schema: z.ZodNumber) =>
  z.preprocess((v) => (v == null ? undefined : v), schema.optional());

// An AmneziaWG client (multi-client model). Same key/address fields as
// WireguardClientSchema — the panel's generic ClientRecord already has those
// exact keys (privateKey/publicKey/preSharedKey/allowedIPs/keepAlive), so
// bulk operations, the QR modal and subscriptions all work unmodified — plus
// one AmneziaWG-only addition, forwardedPorts (its embedded listener
// supervisor can forward into a peer's tunnel address; WireGuard's stock
// Xray-native inbound has no equivalent mechanism).
// Keys are optional on the wire — the backend generates them when absent.
export const AmneziawgClientSchema = z.object({
  privateKey: z.string().optional(),
  publicKey: z.string().optional(),
  preSharedKey: z.string().optional(),
  allowedIPs: z.array(z.string()).default([]),
  keepAlive: optionalClearedInt(z.number().int().min(0)),
  forwardedPorts: z.string().default(''),
  email: z.string().min(1),
  limitIp: z.number().int().min(0).default(0),
  totalGB: z.number().int().min(0).default(0),
  expiryTime: z.number().int().default(0),
  enable: z.boolean().default(true),
  tgId: z.union([z.number(), z.string()]).transform((v) => Number(v) || 0).default(0),
  subId: z.string().default(''),
  comment: z.string().default(''),
  reset: z.number().int().min(0).default(0),
  created_at: z.number().int().optional(),
  updated_at: z.number().int().optional(),
});
export type AmneziawgClient = z.infer<typeof AmneziawgClientSchema>;

// Server-wide AmneziaWG 2.0 obfuscation parameters and tunnel identity,
// mirroring internal/amneziawg.ServerSettings on the Go side (same field
// names) — the listen port is not duplicated here, it's the inbound's own
// port like every other protocol. H1-H4 blank falls back to the classic
// 1/2/3/4 magic header on save; I1 blank omits the 2.0-only CPS signature
// packet (a 1.x-compatible config).
//
// Deliberately missing routeThroughXray: that field is vestigial on the Go
// side too, as of the hard cutover to the embedded (amneziawg-go) path --
// see ServerSettings' own doc comment. Omitting it here means z.object's
// default unknown-key stripping quietly drops it from an existing stored
// settings blob on the next save, rather than the form round-tripping a
// value nothing reads anymore.
// AWG_VERSION_2/AWG_VERSION_3 mirror internal/amneziawg.AwgVersion2/AwgVersion3
// on the Go side exactly (same string values).
export const AWG_VERSION_2 = '2';
export const AWG_VERSION_3 = '3';

// effectiveAwgVersion mirrors internal/amneziawg.EffectiveAwgVersion exactly:
// an inbound saved before the awgVersion field existed can already have one
// of the AWG3-only fields (headerProtectionKey, contentPaddingAddition, or
// any of the 5 device-timer fields) set with no awgVersion at all
// (rawInboundToFormValues casts the stored settings JSON verbatim, with no
// schema-default pass on read). Treating a blank awgVersion alongside any of
// those as already-opted-into-3 means the edit form's version Select shows
// what's actually in effect, instead of defaulting to "2" and the very next
// unrelated save getting rejected by the backend's own ValidateAwgVersion
// consistency check. Only an explicitly blank awgVersion is promoted this
// way -- an explicit, non-"3" value is returned as-is so the backend's own
// error can surface the real inconsistency. awg3Fields takes every AWG3-only
// field's current value, in any order (only "is at least one non-empty"
// matters) -- mirrors the Go side's own variadic signature.
export function effectiveAwgVersion(
  awgVersion: string | undefined,
  ...awg3Fields: (string | undefined)[]
): string {
  if (!awgVersion && awg3Fields.some((f) => !!f)) {
    return AWG_VERSION_3;
  }
  return awgVersion || AWG_VERSION_2;
}

export const AmneziawgServerSchema = z.object({
  privateKey: z.string().optional(),
  publicKey: z.string().optional(),
  subnetIp: z.string().default('10.8.1.0'),
  subnetCidr: z.number().int().min(1).max(32).default(24),
  mtu: optionalClearedInt(z.number().int().min(1)),
  primaryDns: z.string().default('8.8.8.8'),
  secondaryDns: z.string().default('8.8.4.4'),
  externalInterface: z.string().default(''),
  ipv6Enabled: z.boolean().default(false),
  ipv6Subnet: z.string().default(''),
  ipv6ExternalInterface: z.string().default(''),
  jc: z.number().int().min(0).default(5),
  jmin: z.number().int().min(0).default(10),
  jmax: z.number().int().min(0).default(50),
  s1: z.number().int().min(0).default(30),
  s2: z.number().int().min(0).default(45),
  s3: z.number().int().min(0).max(64).default(10),
  s4: z.number().int().min(0).max(32).default(5),
  h1: z.string().default(''),
  h2: z.string().default(''),
  h3: z.string().default(''),
  h4: z.string().default(''),
  // i1-i5 are the real protocol's five CPS signature-packet slots (i1
  // shipped first; i2-i5 are the same grammar, the remaining four slots).
  i1: z.string().default(''),
  i2: z.string().default(''),
  i3: z.string().default(''),
  i4: z.string().default(''),
  i5: z.string().default(''),
  // awgVersion is the admin-declared protocol-version ceiling: AWG_VERSION_2
  // (default) or AWG_VERSION_3. headerProtectionKey/contentPaddingAddition
  // below require this to be AWG_VERSION_3 -- see effectiveAwgVersion above
  // for the back-compat rule and ValidateAwgVersion on the Go side for the
  // save-time consistency check.
  awgVersion: z.string().default(AWG_VERSION_2),
  // AmneziaWG 3.0 fields, strictly opt-in -- see amneziawg.tsx's own hint
  // copy. headerProtectionKey requires s1-s4 above to all be >= 12.
  headerProtectionKey: z.string().default(''),
  contentPaddingAddition: z.string().default(''),
  // The 5 AWG3 device-timer fields: same "low-high"/bare-integer grammar as
  // h1-h4/contentPaddingAddition. Empty leaves that one field at
  // amneziawg-go's own real-protocol default (rekeyAfterTime 120s,
  // rekeyTimeout 5s, rejectAfterTime 180s, keepaliveTimeout 10s,
  // maxHandshakeAttempts 18).
  rekeyAfterTime: z.string().default(''),
  rekeyTimeout: z.string().default(''),
  rejectAfterTime: z.string().default(''),
  keepaliveTimeout: z.string().default(''),
  maxHandshakeAttempts: z.string().default(''),
});
export type AmneziawgServer = z.infer<typeof AmneziawgServerSchema>;

export const AmneziawgInboundSettingsSchema = z.object({
  server: AmneziawgServerSchema,
  clients: z.array(AmneziawgClientSchema).default([]),
});
export type AmneziawgInboundSettings = z.infer<typeof AmneziawgInboundSettingsSchema>;
