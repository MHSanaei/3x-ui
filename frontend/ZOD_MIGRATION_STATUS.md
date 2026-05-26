# 3x-ui Frontend Zod Migration — Status

Branch: `feat/frontend-zod-validation` · 83 commits ahead of `main`

Last updated: 2026-05-26

## What this is

The work tracked here is the migration described in
`C:\Users\Hossein Sanaei\.claude\plans\zod-soft-feather.md` — replacing the
class-based xray models (`models/inbound.ts`, `models/outbound.ts`) with Zod
schemas as the single source of truth, standardizing every form on AntD
`Form.useForm` + `antdRule(schema.shape.X)`, and tightening
`@typescript-eslint/no-explicit-any` to `error`.

Verify state: `npm run typecheck` clean, `npm run lint` clean,
`npm run test` 302/302, snapshot baselines 172/172.

---

## Done

### Foundations

- API-boundary Zod validation in TanStack Query hooks (`parseMsg` helper)
- Backend request-body validation via `go-playground/validator`
- Go-first codegen tool (`tools/openapigen`) emitting `zod.ts` + `types.ts`
- `antdRule(schema)` helper bridging Zod issues to AntD form rules
- Five secondary modals migrated to Pattern A (Login, 2FA, Geo, Balancer, Rule)
- Pre-save schema guard on Inbound/Outbound form submits

### Schemas — `frontend/src/schemas/`

- `primitives/` — port, protocol, sniffing, atomic dictionaries
- `protocols/inbound/*` — 10 protocols as leaf schemas
- `protocols/outbound/*` — 11 protocols as leaf schemas
- `protocols/stream/*` — 7 networks (tcp/kcp/ws/grpc/httpupgrade/xhttp/hysteria)
- `protocols/security/*` — 3 securities (none/tls/reality)
- `forms/inbound-form.ts` — `InboundFormValues` discriminated union
- `forms/outbound-form.ts` — `OutboundFormValues` discriminated union
- Stream + security families wired as `z.discriminatedUnion` with intersection

### Pure-function ports — `frontend/src/lib/xray/`

- `headers.ts` — `toHeaders`, `toV2Headers`, `getHeaderValue`
- `inbound-link.ts` — `genVmessLink`, `genVlessLink`, `genTrojanLink`,
  `genShadowsocksLink`, `genHysteriaLink`, Wireguard link/config
- `outbound-link-parser.ts` — vmess/vless/trojan/shadowsocks/hysteria2
- `inbound-defaults.ts` — `createDefault{Vmess,Vless,...}{Client,InboundSettings}`
- `outbound-defaults.ts` — settings factories + dispatcher
- `outbound-form-adapter.ts` — raw ↔ `OutboundFormValues` round-trip
- `protocol-capabilities.ts` — capability predicates as pure functions

### Form modals on Pattern A

- `InboundFormModal.tsx` — full rewrite, atomic-swapped from `.new.tsx`
  - Tabs: Basic, Sniffing, Protocol, Stream, Security, Advanced JSON,
    Fallbacks
  - All 10 protocols (VLESS, VMess, Trojan, Shadowsocks, HTTP, Mixed,
    Tunnel, TUN, Wireguard, Hysteria)
  - Full Stream tab (TCP, KCP, WS, gRPC, HTTPUpgrade, XHTTP, Hysteria)
  - Full Security tab (TLS list, Reality, ECH, mldsa65)
  - 18-field sockopt section, full TLS cert list, external-proxy section
- `OutboundFormModal.tsx` — full rewrite, atomic-swapped from `.new.tsx`
  - All 12 protocols (vmess/vless/trojan/shadowsocks/socks/http/hysteria/
    freedom/blackhole/dns/loopback/wireguard)
  - Full Stream tab with XHTTP advanced fields + xmux sub-form
  - Full Security tab (TLS + Reality + Vision flow)
  - Sockopt section (17 knobs)
  - Mux section
  - JSON tab for advanced fields
  - Link import (vmess/vless/trojan/ss/hysteria2) with full XHTTP
    round-trip (padding obfs + session/seq/uplink keys + all post-size
    knobs)
- `FinalMaskForm` rewritten to Pattern A (Form.List-driven) and wired
  into both stream tabs (Inbound + Outbound). Covers TCP/UDP mask
  arrays, all 13 UDP mask types, header-custom nested groups, noise
  items, and the QUIC params sub-form.

### Tests

- Golden-file fixture suite (`test/golden/fixtures/`)
- Snapshot-baseline regression tests for inbound-full / outbound / stream /
  security DUs
- Shadow-parse harness asserting legacy class and Zod converge
- Link-parser tests (15 round-trip cases including XHTTP padding-obfs)
- Outbound form-adapter tests (15 round-trip cases)
- 302 tests across 12 files, 172 snapshots

### Build infrastructure

- `@typescript-eslint/no-explicit-any: 'error'` enforced
- `.github/workflows/ci.yml` runs `typecheck` + `test` before `build`
- Vite pinned to 8.0.13 (dev-mode dep-optimizer regression in 8.0.14)

---

## Remaining

### Out of migration scope (per plan)

- `DBInbound`, `Status`, `AllSetting` legacy classes — flagged as out of
  scope in `zod-soft-feather.md`. The mainline migration of
  `models/inbound.ts` / `models/outbound.ts` cannot delete them entirely
  while `DBInbound.toInbound()` still imports.
- The plan accepts this and treats parity via snapshot baselines instead.

### Nice-to-haves — would not block ship

- Reality `sid=` multi-value parsing in share-link import
  (outbound reality only carries a single shortId — this is server-side
  state)
- `fm=` (FinalMask) param in share-link import
- VMess link `xmux` nested JSON parsing (currently round-trips at the
  XHTTP top level; nested xmux object is left empty)
- Tighter `.loose()` removal in `schemas/api/inbound.ts`,
  `schemas/api/client.ts`, `schemas/xray.ts` — gated on Step 6 of the plan
  (currently held because the codegen tool still emits one or two loose
  fields the panel writes back)

---

## How to pick up where this left off

1. `git checkout feat/frontend-zod-validation`
2. `cd frontend && npm install && npm run typecheck && npm run test`
3. Open `C:\Users\Hossein Sanaei\.claude\plans\zod-soft-feather.md` —
   Steps 1–5 are done. Step 6 (tighten `.loose()`) and Step 7 (lint/CI
   tightening) are partially done.
4. Nothing in this list blocks ship. The mainline migration goal
   (replace class-based models with Zod schemas + Pattern A forms) is
   done; remaining work is incremental polish.
