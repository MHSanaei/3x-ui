import {
  formValuesToWirePayload,
  rawInboundToFormValues,
  type WireInboundPayload,
} from '@/lib/xray/inbound-form-adapter';
import { INBOUND_PRESETS, type InboundPreset } from '@/lib/xray/inbound-presets';
import { parseOutboundLink } from '@/lib/xray/outbound-link-parser';
import type { XraySettingsValue } from '@/schemas/xray';

// Relay entries are always cert-free (Reality) presets — a relay node users
// connect to shouldn't need a domain/cert. The landing side carries the
// protocol variety instead.
export const RELAY_ENTRY_PRESETS: readonly InboundPreset[] = INBOUND_PRESETS.filter(
  (p) => !p.needsDomain,
);

// Relay (中转) wiring helpers. A relay setup = a local inbound users connect
// to (the entry, built from a one-click preset) + an outbound that forwards to
// the landing server (出口/落地) + a routing rule that pins that inbound's
// traffic to that outbound. These pure builders own the outbound + rule + the
// immutable template splice; the wizard handles the API orchestration.

export type LandingProtocol = 'vless' | 'vmess' | 'trojan' | 'shadowsocks' | 'socks' | 'http';

// An outbound object as it lives in xraySetting.outbounds[]. Kept loose to
// match the template schema (z.object(...).loose()).
export type RelayOutbound = Record<string, unknown> & {
  tag: string;
  protocol: string;
  settings: unknown;
  streamSettings?: unknown;
};

export interface RelayRule {
  type: 'field';
  inboundTag: string[];
  outboundTag: string;
}

// Parsed result of a pasted landing endpoint string. Any field may be empty
// when the input didn't carry it. The wizard spreads these into its form.
export interface ParsedLandingEndpoint {
  address: string;
  port?: number;
  user?: string;
  pass?: string;
}

// Parse common colon-delimited landing endpoint formats users paste for a
// SOCKS/HTTP landing:
//   host:port
//   host:port:user:pass            (the residential-proxy 4-tuple)
//   user:pass@host:port            (URL-style userinfo)
// Bracketed IPv6 ([::1]:port) is handled. Returns null when it doesn't look
// like an endpoint (e.g. a bare hostname or a vless:// link — those go
// through other paths).
export function parseLandingEndpoint(raw: string): ParsedLandingEndpoint | null {
  const s = raw.trim();
  if (s === '' || s.includes('://')) return null;

  // user:pass@host:port
  if (s.includes('@')) {
    const at = s.lastIndexOf('@');
    const creds = s.slice(0, at);
    const hostPort = s.slice(at + 1);
    const hp = parseHostPort(hostPort);
    if (!hp) return null;
    const ci = creds.indexOf(':');
    if (ci >= 0) return { ...hp, user: creds.slice(0, ci), pass: creds.slice(ci + 1) };
    return { ...hp, user: creds };
  }

  // Bracketed IPv6 forms — let parseHostPort handle host[:port], then any
  // trailing :user:pass.
  if (s.startsWith('[')) {
    return parseHostPort(s);
  }

  const parts = s.split(':');
  if (parts.length === 2) {
    const port = toPort(parts[1]);
    return port ? { address: parts[0], port } : null;
  }
  if (parts.length === 4) {
    const port = toPort(parts[1]);
    return port ? { address: parts[0], port, user: parts[2], pass: parts[3] } : null;
  }
  // 3+ colons without 4 parts, or a single token → not a recognizable endpoint.
  return null;
}

function toPort(v: string): number | undefined {
  const n = Number(v.trim());
  return Number.isInteger(n) && n >= 1 && n <= 65535 ? n : undefined;
}

function parseHostPort(s: string): ParsedLandingEndpoint | null {
  const t = s.trim();
  if (t.startsWith('[')) {
    const end = t.indexOf(']');
    if (end < 0) return null;
    const host = t.slice(1, end);
    const rest = t.slice(end + 1);
    if (rest.startsWith(':')) {
      const port = toPort(rest.slice(1));
      return port ? { address: host, port } : { address: host };
    }
    return { address: host };
  }
  const ci = t.lastIndexOf(':');
  if (ci < 0) return t.length > 0 ? { address: t } : null;
  const port = toPort(t.slice(ci + 1));
  return port ? { address: t.slice(0, ci), port } : { address: t };
}

export interface LandingManualInput {
  protocol: LandingProtocol;
  address: string;
  port: number;
  // Protocol-specific credentials. Unused fields are ignored per protocol.
  id?: string; // vless / vmess UUID
  password?: string; // trojan / shadowsocks
  method?: string; // shadowsocks cipher
  flow?: string; // vless
  user?: string; // socks / http username
  pass?: string; // socks / http password
}

function socksHttpUsers(user?: string, pass?: string) {
  const u = (user ?? '').trim();
  const p = pass ?? '';
  return u ? [{ user: u, pass: p }] : [];
}

// Build the landing outbound's `settings` block from manual fields. Manual
// mode covers the plain (no transport-security) case — for a TLS/Reality
// landing (e.g. another 3x-ui node) the operator pastes its share link, which
// carries the full streamSettings. socks/http (the common residential-IP
// case) are plain by nature.
function manualLandingSettings(input: LandingManualInput): unknown {
  const { protocol, address, port } = input;
  switch (protocol) {
    case 'vless':
      return { address, port, id: (input.id ?? '').trim(), flow: input.flow ?? '', encryption: 'none' };
    case 'vmess':
      return { vnext: [{ address, port, users: [{ id: (input.id ?? '').trim(), security: 'auto' }] }] };
    case 'trojan':
      return { servers: [{ address, port, password: input.password ?? '' }] };
    case 'shadowsocks':
      return {
        servers: [{ address, port, password: input.password ?? '', method: input.method ?? '2022-blake3-aes-256-gcm' }],
      };
    case 'socks':
      return { servers: [{ address, port, users: socksHttpUsers(input.user, input.pass) }] };
    case 'http':
      return { servers: [{ address, port, users: socksHttpUsers(input.user, input.pass) }] };
    default:
      return {};
  }
}

// Suffix `base` with -2, -3, … until it's not already taken. Keeps relay
// outbound tags unique within the template so the routing rule binds to the
// right one.
export function uniqueOutboundTag(existingTags: Iterable<string>, base: string): string {
  const taken = new Set(existingTags);
  if (!taken.has(base)) return base;
  for (let i = 2; ; i += 1) {
    const candidate = `${base}-${i}`;
    if (!taken.has(candidate)) return candidate;
  }
}

// Build the landing outbound from manual fields. Caller supplies the final
// (already-unique) tag.
export function landingOutboundFromManual(input: LandingManualInput, tag: string): RelayOutbound {
  return {
    tag,
    protocol: input.protocol,
    settings: manualLandingSettings(input),
  };
}

// Build the landing outbound by parsing a share link (vless/vmess/trojan/ss/
// hysteria2). Returns null when the link can't be parsed. The parsed remark
// tag is discarded — the caller's unique relay tag is used instead so the
// routing rule can reference it deterministically.
export function landingOutboundFromLink(link: string, tag: string): RelayOutbound | null {
  const parsed = parseOutboundLink(link.trim());
  if (!parsed || typeof parsed.protocol !== 'string') return null;
  const outbound: RelayOutbound = {
    tag,
    protocol: parsed.protocol,
    settings: parsed.settings ?? {},
  };
  if (parsed.streamSettings != null) outbound.streamSettings = parsed.streamSettings;
  return outbound;
}

// Tag prefix marking an inbound as a relay entry. The backend's
// resolveInboundTag preserves a non-empty, non-colliding tag, so this prefix
// survives and lets both the inbound list and the client list flag relay
// entries without any DB schema change.
export const RELAY_ENTRY_TAG_PREFIX = 'relay-in-';

// True when an inbound tag identifies a relay entry (created by the relay
// wizard). Used to badge rows in the inbound and client lists.
export function isRelayEntryTag(tag: string | undefined | null): boolean {
  return typeof tag === 'string' && tag.startsWith(RELAY_ENTRY_TAG_PREFIX);
}

export interface RelayEntryOverrides {
  remark?: string;
  port?: number;
  // Reality key pair fetched from the panel (GET /panel/api/server/getNewX25519Cert).
  realityPrivateKey?: string;
  realityPublicKey?: string;
}

// Build the wire payload for the relay's entry inbound from a (cert-free)
// preset, applying remark/port and the fetched Reality key pair. Mirrors the
// path InboundFormModal.submit() uses: preset row → form values → wire payload.
export function buildRelayInboundPayload(
  preset: InboundPreset,
  ov: RelayEntryOverrides = {},
): WireInboundPayload {
  const values = rawInboundToFormValues(preset.build()) as unknown as Record<string, unknown>;
  if (ov.remark != null) values.remark = ov.remark;
  if (ov.port != null && ov.port > 0) values.port = ov.port;
  // Tag the entry inbound so the lists can badge it as a relay. Port keeps it
  // unique; the backend preserves this tag when it doesn't collide.
  const taggedPort = ov.port != null && ov.port > 0 ? ov.port : (values.port as number);
  values.tag = `${RELAY_ENTRY_TAG_PREFIX}${taggedPort}`;

  const stream = values.streamSettings as Record<string, unknown> | undefined;
  const reality = stream?.realitySettings as Record<string, unknown> | undefined;
  if (reality) {
    if (ov.realityPrivateKey != null) reality.privateKey = ov.realityPrivateKey;
    if (ov.realityPublicKey != null) {
      const inner = (reality.settings as Record<string, unknown> | undefined) ?? {};
      inner.publicKey = ov.realityPublicKey;
      reality.settings = inner;
    }
  }
  return formValuesToWirePayload(values as never);
}

export function buildRelayRule(relayInboundTag: string, landingOutboundTag: string): RelayRule {
  return { type: 'field', inboundTag: [relayInboundTag], outboundTag: landingOutboundTag };
}

// Immutably splice a relay outbound + its routing rule into the xray template.
// The rule is prepended so it wins over any catch-all rule already present
// (Xray evaluates routing rules top-to-bottom, first match wins).
export function applyRelayToTemplate(
  template: XraySettingsValue,
  outbound: RelayOutbound,
  rule: RelayRule,
): XraySettingsValue {
  const outbounds = [...(template.outbounds ?? []), outbound];
  const prevRouting = template.routing ?? {};
  const rules = [rule, ...(prevRouting.rules ?? [])];
  return {
    ...template,
    outbounds,
    routing: { ...prevRouting, rules },
  } as XraySettingsValue;
}

