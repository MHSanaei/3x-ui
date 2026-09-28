# Architecture and actual/proposed paths

Status: design selected for implementation, not yet an implemented system.
The binding scope is [the complete request](requirements.zh-CN.md).

## Choices evaluated

1. Put arbitrary protocol names into Xray config: rejected; the core does not
   implement those protocols and a generated JSON object is no backend.
2. Shape each listener/IP externally: useful only for exclusive ownership;
   cannot classify authenticated clients sharing a listener/public address.
3. Reuse panel management with authenticated backend adapters and one policy
   authority per immutable client: selected. Use core extension points for
   Xray, embedded handlers where feasible, isolated processes otherwise.

## Identity and control plane

Retain `ClientRecord`, `ClientInbound`, `InboundService`, `ClientService` and
`runtime.Runtime`. Introduce immutable client policy UUIDs (new UUID after
deletion/recreation), policy revision, independent restriction reasons, rate
units and fixed-point multiplier. Inbound/credential bindings map to that
UUID. Email remains a user-facing label, not a durable accounting identity.

The DB transaction owns cumulative raw totals, billed bytes plus fractional
remainder, per-source cursor/epoch and policy revision. A durable report is
idempotent by owner/client/source/incarnation/sequence; no in-memory-only
cursor is sufficient. Out-of-order, stale and conflicting reports fail safely.
Rate changes and multiplier changes serialize with the old revision's final
counter snapshot; no retroactive reprice. Readers expose raw and billed totals.

Policy application validates backend capabilities, stages configuration,
applies it, confirms observed revision and then reports success. Failed apply
retains the last safe config or blocks the affected service; never direct
fallback. Reconcile desired/observed revisions after crashes and reboot.
Subprocesses use private config files, bounded logs, exit supervision and
owned-resource cleanup. All node mutations still dispatch through Runtime.

## Data paths (targets to prove)

| Service | Identity, meter, shaper and route location |
|---|---|
| Xray protocols | client → native auth → client-aware dispatcher wrapper → shared policy budget/meter → existing Xray router → selected outbound → target |
| SSH -L/-D | SSH public-key auth → direct-tcpip channel mapped to client UUID → shared policy stream → authenticated internal routing bridge → Xray outbound → target |
| SSH -R | authorized bounded listener → forwarded-tcpip channel → same client policy; listener and remote target allowlists; disabled by default |
| SSH outbound | Xray-selected loopback bridge → pinned upstream SSH host key → direct-tcpip channel → target; no host management sshd changes |
| mieru | native authenticated user → policy-aware server dispatch adapter → same routing bridge → selected outbound; TCP and UDP separately tested |
| Snell v4/v5/v6 | unique client PSK + exclusive managed instance/listener → per-client isolated egress namespace → transparent TCP/UDP routing bridge → policy/routing → outbound |
| Port forwarding | exclusive node/transport/address/port owner → TCP stream or UDP association → client policy → original configured target + unified routing → outbound |
| AmneziaWG/WireGuard | authenticated peer → stable client mapping → userspace flow bridge/core dispatcher → shared policy → router/outbound |
| TUIC | authenticated UUID at backend dispatch (existing encrypted aggregate relay insufficient) → client policy → routing bridge |
| MTProto | per-secret authenticated identity → backend policy/identity-preserving egress → routing; no second charge at bridge |

An internal bridge authenticates the client with a private per-binding secret
and carries original destination, network and source inbound. Domain rules use
only domains supplied by the protocol or actually observed; an IP-only Snell
egress cannot invent a domain. Namespace DNS interception and observable
mapping can preserve available DNS information, but must not promise domains
for externally resolved requests. Sniffing follows existing explicit policy.
Route precedence, block, balancer and egress choice need real requests.

## Rate and quota enforcement

Each authenticated data path acquires raw-byte budget from the same client
policy object, shared across directions' independent buckets, connections,
channels and listeners. Bounded queues/backpressure implement shaping; a
datagram larger than the available quota is rejected whole. A policy update
wakes waiters; disable/quota/expiry cancels only that client's flows.
The accounting boundary is before forwarding application payload; backend
network counters remain diagnostic, never a second billing source.

Quota uses durable reservations so simultaneous flows cannot independently
spend the same remainder. The reservation/event journal has bounded units,
acknowledgment and recovery rules; do not claim zero overuse until independently
observed at the documented boundary. Expiry and manual disable are distinct
from depletion; reset clears usage/depletion only.

For multiple nodes, an owner allocates bounded byte credits and rate shares;
the sum of shares cannot exceed the configured global client rate. A node with
an expired policy lease stops protected traffic. Do not expose node-local caps
as global. Node snapshots carry already billed usage and source identity,
without applying the multiplier again. Mixed-version nodes reject policies
they cannot enforce rather than silently running unrestricted.

## Backend-specific constraints

- Snell: separate v4, v5 and beta v6 binaries until each compatibility pairing
  is verified. One client per process/listener; describe memory/process/port
  costs. Test ordinary UDP and v5/v6 QUIC mode separately. Official binary
  provenance/hash/architecture checks, license review and Linux capability
  gating are required. Real Surge clients are currently unavailable here.
- mieru: prefer its native user model. Its rolling-window quota and panel
  lifetime/reset quota must not run as independent authorities. A policy-aware
  adapter must preserve username before outgoing SOCKS dispatch and UDP.
- SSH: embedded dedicated server; public keys, no session/shell/exec/PTY/SFTP/
  agent forwarding. Standard -L/-D are TCP; -R requires admin opt-in and
  controlled addresses/ports. No UDP claim for standard SSH channels.
- Port forwarding: userspace implementation gives explicit ownership and a
  uniform routing hook. nftables evaluation remains documented; if introduced,
  only dedicated rules/chains/marks, atomic validation/application, restored
  conntrack marks, bidirectional counting and established-flow revocation.

## Integration and acceptance

Every vertical task includes models/migrations, service, API schemas and
docs, React forms/validation, en-US/zh-CN plus existing locale fallback rules,
real backend lifecycle, raw/billed statistics, export, recovery and packaging.
Protocol-only fields appear only where meaningful. Diagnostics expose observed
capabilities and revision without secrets. Missing binaries or permissions
produce actionable errors.

The matrix tracks all existing features, including LDAP/groups/HWID/host
overrides/fallbacks/notifications, and must be revisited per vertical slice.
The final gate includes make verify/race, PostgreSQL, real clients, sustained
throughput at two caps and unlimited baseline, concurrent same-IP clients,
quota cutoff/restart, route egress/priority/block and failure recovery.
