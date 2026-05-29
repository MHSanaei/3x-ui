import { z } from 'zod';

import { PortSchema } from '@/schemas/primitives';
import { VmessSecuritySchema } from '@/schemas/protocols/inbound/vmess';
import { SSMethodSchema } from '@/schemas/protocols/inbound/shadowsocks';
import { SecuritySettingsSchema } from '@/schemas/protocols/security';
import { NetworkSettingsSchema, StreamExtrasSchema } from '@/schemas/protocols/stream';
import {
  BlackholeResponseTypeSchema,
  DNSRuleActionSchema,
  FreedomFinalRuleActionSchema,
  FreedomFragmentSchema,
  FreedomNoiseSchema,
  OutboundDomainStrategySchema,
  WireguardDomainStrategySchema,
} from '@/schemas/protocols/outbound';

// OutboundFormValues = the shape Form.useForm<T>() carries inside
// OutboundFormModal. Differences from schemas/api wire schemas:
//
//   - vmess vnext / trojan-ss-socks-http servers are FLATTENED into
//     {address, port, ...auth} at settings root. The adapter handles
//     nesting on submit.
//   - wireguard `address` (string[] wire) and `reserved` (number[] wire)
//     are comma-joined STRINGS in the form. The adapter splits + coerces.
//   - wireguard `pubKey` is a UI-only field derived from `secretKey`. Not
//     emitted on the wire — the adapter strips it.
//   - VLESS `reverseTag` and `reverseSniffing` are flat at settings root;
//     the adapter wraps them as { reverse: { tag, sniffing } } on the wire.
//   - blackhole `type` ('' | 'none' | 'http') is flat; the adapter wraps it
//     as { response: { type } } on the wire (omitted when empty).
//   - DNS rules carry `qtype` and `domain` as comma-joined strings (matches
//     the legacy DNSRule UI). The adapter normalizes them on submit.
//
// All flat-form settings types are documented inline so the adapter has a
// single source of truth for the shape it converts between.

// VMess outbound: connect target (address+port) + first user (id+security).
// Wire: { vnext: [{ address, port, users: [{ id, security }] }] }.
export const VmessOutboundFormSettingsSchema = z.object({
  address: z.string().default(''),
  port: PortSchema.default(443),
  id: z.string().default(''),
  security: VmessSecuritySchema.default('auto'),
});
export type VmessOutboundFormSettings = z.infer<typeof VmessOutboundFormSettingsSchema>;

// Reverse-sniffing is only emitted when reverseTag is non-empty. Defaults
// match legacy ReverseSniffing constructor.
export const ReverseSniffingFormSchema = z.object({
  enabled: z.boolean().default(false),
  destOverride: z.array(z.string()).default(['http', 'tls', 'quic', 'fakedns']),
  metadataOnly: z.boolean().default(false),
  routeOnly: z.boolean().default(false),
  ipsExcluded: z.array(z.string()).default([]),
  domainsExcluded: z.array(z.string()).default([]),
});
export type ReverseSniffingForm = z.infer<typeof ReverseSniffingFormSchema>;

// VLESS outbound: flat connect target + auth + Vision-specific knobs +
// reverse-sniffing slice. testpre/testseed live behind canEnableVisionSeed.
export const VlessOutboundFormSettingsSchema = z.object({
  address: z.string().default(''),
  port: PortSchema.default(443),
  id: z.string().default(''),
  flow: z.string().default(''),
  encryption: z.string().min(1).default('none'),
  reverseTag: z.string().default(''),
  reverseSniffing: ReverseSniffingFormSchema.default({
    enabled: false,
    destOverride: ['http', 'tls', 'quic', 'fakedns'],
    metadataOnly: false,
    routeOnly: false,
    ipsExcluded: [],
    domainsExcluded: [],
  }),
  testpre: z.number().int().min(0).default(0),
  testseed: z.array(z.number().int().positive()).default([]),
});
export type VlessOutboundFormSettings = z.infer<typeof VlessOutboundFormSettingsSchema>;

export const TrojanOutboundFormSettingsSchema = z.object({
  address: z.string().default(''),
  port: PortSchema.default(443),
  password: z.string().default(''),
});
export type TrojanOutboundFormSettings = z.infer<typeof TrojanOutboundFormSettingsSchema>;

export const ShadowsocksOutboundFormSettingsSchema = z.object({
  address: z.string().default(''),
  port: PortSchema.default(443),
  password: z.string().default(''),
  method: SSMethodSchema.default('2022-blake3-aes-128-gcm'),
  uot: z.boolean().default(false),
  UoTVersion: z.number().int().min(1).max(2).default(1),
});
export type ShadowsocksOutboundFormSettings = z.infer<typeof ShadowsocksOutboundFormSettingsSchema>;

// SOCKS / HTTP: panel only supports a single server, with optionally one
// user (the adapter emits users: [] when user is empty).
export const SocksOutboundFormSettingsSchema = z.object({
  address: z.string().default(''),
  port: PortSchema.default(1080),
  user: z.string().default(''),
  pass: z.string().default(''),
});
export type SocksOutboundFormSettings = z.infer<typeof SocksOutboundFormSettingsSchema>;

export const HttpOutboundFormSettingsSchema = z.object({
  address: z.string().default(''),
  port: PortSchema.default(8080),
  user: z.string().default(''),
  pass: z.string().default(''),
});
export type HttpOutboundFormSettings = z.infer<typeof HttpOutboundFormSettingsSchema>;

// Wireguard peer mirrors the legacy Outbound.WireguardSettings.Peer class.
// `psk` (form) <-> `preSharedKey` (wire) — adapter renames.
export const WireguardOutboundFormPeerSchema = z.object({
  publicKey: z.string().default(''),
  psk: z.string().default(''),
  allowedIPs: z.array(z.string()).default(['0.0.0.0/0', '::/0']),
  endpoint: z.string().default(''),
  keepAlive: z.number().int().min(0).default(0),
});
export type WireguardOutboundFormPeer = z.infer<typeof WireguardOutboundFormPeerSchema>;

// Wireguard: `address` and `reserved` are comma-joined strings in the form
// (the legacy UI binds them to a single Input). pubKey is UI-only — the
// modal derives it from secretKey via Wireguard.generateKeypair() and
// displays it disabled; the adapter strips it.
export const WireguardOutboundFormSettingsSchema = z.object({
  mtu: z.number().int().min(0).default(1420),
  secretKey: z.string().default(''),
  pubKey: z.string().default(''),
  address: z.string().default(''),
  workers: z.number().int().min(0).default(2),
  domainStrategy: z.union([WireguardDomainStrategySchema, z.literal('')]).default(''),
  reserved: z.string().default(''),
  peers: z.array(WireguardOutboundFormPeerSchema).default([]),
  noKernelTun: z.boolean().default(false),
});
export type WireguardOutboundFormSettings = z.infer<typeof WireguardOutboundFormSettingsSchema>;

// Hysteria outbound carries the connect target only; transport-layer knobs
// (auth, congestion, up/down, hop port, timeouts) ride on stream.hysteria.
export const HysteriaOutboundFormSettingsSchema = z.object({
  address: z.string().default(''),
  port: PortSchema.default(443),
  version: z.literal(2).default(2),
});
export type HysteriaOutboundFormSettings = z.infer<typeof HysteriaOutboundFormSettingsSchema>;

// FinalRule (freedom): network/port are strings; ip is string[]; blockDelay
// is only meaningful when action === 'block'. The adapter omits empty
// fields from the wire payload.
export const FreedomFinalRuleFormSchema = z.object({
  action: FreedomFinalRuleActionSchema.default('block'),
  network: z.string().default(''),
  port: z.string().default(''),
  ip: z.array(z.string()).default([]),
  blockDelay: z.string().default(''),
});
export type FreedomFinalRuleForm = z.infer<typeof FreedomFinalRuleFormSchema>;

export const FreedomOutboundFormSettingsSchema = z.object({
  domainStrategy: z.union([OutboundDomainStrategySchema, z.literal('')]).default(''),
  redirect: z.string().default(''),
  proxyProtocol: z.number().int().min(0).max(2).default(0),
  fragment: FreedomFragmentSchema.default({
    packets: '1-3',
    length: '',
    interval: '',
    maxSplit: '',
  }),
  noises: z.array(FreedomNoiseSchema).default([]),
  finalRules: z.array(FreedomFinalRuleFormSchema).default([]),
});
export type FreedomOutboundFormSettings = z.infer<typeof FreedomOutboundFormSettingsSchema>;

// Blackhole: legacy form keeps `type` as a flat string ('' | 'none' | 'http');
// adapter wraps as { response: { type } } on the wire and omits when empty.
export const BlackholeOutboundFormSettingsSchema = z.object({
  type: z.union([BlackholeResponseTypeSchema, z.literal('')]).default(''),
});
export type BlackholeOutboundFormSettings = z.infer<typeof BlackholeOutboundFormSettingsSchema>;

// DNS rules: form holds qtype + domain as joined strings (the legacy UI
// binds to <Input>). Adapter parses them on submit per the DNSRule class.
export const DnsRuleFormSchema = z.object({
  action: DNSRuleActionSchema.default('direct'),
  qtype: z.string().default(''),
  domain: z.string().default(''),
});
export type DnsRuleForm = z.infer<typeof DnsRuleFormSchema>;

export const DnsOutboundFormSettingsSchema = z.object({
  rewriteNetwork: z.union([z.enum(['udp', 'tcp']), z.literal('')]).default(''),
  rewriteAddress: z.string().default(''),
  rewritePort: z.number().int().min(0).max(65535).default(53),
  userLevel: z.number().int().min(0).default(0),
  rules: z.array(DnsRuleFormSchema).default([]),
});
export type DnsOutboundFormSettings = z.infer<typeof DnsOutboundFormSettingsSchema>;

export const LoopbackOutboundFormSettingsSchema = z.object({
  inboundTag: z.string().default(''),
});
export type LoopbackOutboundFormSettings = z.infer<typeof LoopbackOutboundFormSettingsSchema>;

// Discriminated union on `protocol`. Same tagged-wrapper pattern as the
// inbound side: each branch is { protocol: literal, settings: <flat> }.
export const OutboundFormSettingsSchema = z.discriminatedUnion('protocol', [
  z.object({ protocol: z.literal('vmess'),       settings: VmessOutboundFormSettingsSchema }),
  z.object({ protocol: z.literal('vless'),       settings: VlessOutboundFormSettingsSchema }),
  z.object({ protocol: z.literal('trojan'),      settings: TrojanOutboundFormSettingsSchema }),
  z.object({ protocol: z.literal('shadowsocks'), settings: ShadowsocksOutboundFormSettingsSchema }),
  z.object({ protocol: z.literal('socks'),       settings: SocksOutboundFormSettingsSchema }),
  z.object({ protocol: z.literal('http'),        settings: HttpOutboundFormSettingsSchema }),
  z.object({ protocol: z.literal('wireguard'),   settings: WireguardOutboundFormSettingsSchema }),
  z.object({ protocol: z.literal('hysteria'),    settings: HysteriaOutboundFormSettingsSchema }),
  z.object({ protocol: z.literal('freedom'),     settings: FreedomOutboundFormSettingsSchema }),
  z.object({ protocol: z.literal('blackhole'),   settings: BlackholeOutboundFormSettingsSchema }),
  z.object({ protocol: z.literal('dns'),         settings: DnsOutboundFormSettingsSchema }),
  z.object({ protocol: z.literal('loopback'),    settings: LoopbackOutboundFormSettingsSchema }),
]);
export type OutboundFormSettings = z.infer<typeof OutboundFormSettingsSchema>;

// Mux ride: only emitted when enabled. The adapter respects canEnableMux
// (gated by protocol + flow + network).
export const MuxFormSchema = z.object({
  enabled: z.boolean().default(false),
  concurrency: z.number().int().default(8),
  xudpConcurrency: z.number().int().default(16),
  xudpProxyUDP443: z.enum(['reject', 'allow', 'skip']).default('reject'),
});
export type MuxForm = z.infer<typeof MuxFormSchema>;

// Stream form mirrors the inbound side: NetworkSettings DU + SecuritySettings
// DU + extras (sockopt). Hysteria gets a side-channel branch in the modal
// (legacy ob.stream.hysteria) — keeping the DU strict for now and routing
// hysteria transport knobs through the Advanced JSON tab if needed.
export const OutboundStreamFormSchema = NetworkSettingsSchema
  .and(SecuritySettingsSchema)
  .and(StreamExtrasSchema);
export type OutboundStreamFormValues = z.infer<typeof OutboundStreamFormSchema>;

// Top-level form base: identity (tag, sendThrough), then the per-protocol
// settings DU, then the stream sub-form, then mux.
export const OutboundFormBaseSchema = z.object({
  tag: z.string().default(''),
  sendThrough: z.string().default(''),
  streamSettings: OutboundStreamFormSchema.optional(),
  mux: MuxFormSchema.default({
    enabled: false,
    concurrency: 8,
    xudpConcurrency: 16,
    xudpProxyUDP443: 'reject',
  }),
});
export type OutboundFormBase = z.infer<typeof OutboundFormBaseSchema>;

// Full form values = base + protocol-discriminated settings. Consumers
// narrow on `.protocol` to access the matching settings branch.
export const OutboundFormSchema = OutboundFormBaseSchema.and(OutboundFormSettingsSchema);
export type OutboundFormValues = z.infer<typeof OutboundFormSchema>;
