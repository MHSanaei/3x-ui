# Protocol × feature coverage (source audit)

Legend: **V** implemented and verified for the stated scope; **U** implemented
but not yet verified; **N** not implemented to the requested semantics (partial
existing code does not count); **—** evidence-backed not applicable. No V is
awarded by source inspection alone. Snapshot: original development baseline;
new implementation evidence must explicitly update the relevant cell.

Protocols: X = VMess, VLESS, Trojan, multi-user Shadowsocks, Hysteria2;
H = HTTP/Mixed SOCKS; W = WireGuard; A = AmneziaWG; T = TUIC v5;
M = MTProto; F = existing tunnel/dokodemo/TUN. The X members share the
management/Runtime path; this grouping does NOT claim equal wire semantics.

| Feature | X | H | W | A | T | M | F | Source evidence / qualification |
|---|---|---|---|---|---|---|---|---|
| Create/edit/delete | U | U | U | U | U | U | U | service/inbound.go; sidecar-specific services |
| Enable/disable listener | U | U | U | U | U | U | U | Runtime and backend reconcile managers |
| Client CRUD/attachment | U | N | U | U | U | U | N | model.ClientRecord/ClientInbound; H accounts need canonical integration |
| Bulk operations | U | N | U | U | U | U | N | service/client_bulk.go; lacks new policy state |
| Independent credentials | U | U | U | U | U | U | N | model.Client; HTTP/SOCKS accounts; F lacks exclusive owner model |
| Quota (new + established flow cutoff) | N | N | N | N | N | U | N | xray job optional restart; MTProto secret-limits need real test |
| Expiry | U | N | U | U | U | U | N | inbound_traffic.go, backend reconcile |
| Renewal / reset | U | N | U | U | U | U | N | client_crud.go, periodic_traffic_reset_job.go |
| Online status | U | N | U | U | U | U | N | GetOnlineUsers, per-peer stats, backend activity |
| IP limit | U | N | N | N | N | N | N | fail2ban job relies on usable authentication logs; not bandwidth shaping |
| HWID/device limit | U | N | U | U | U | U | N | sub/hwid_controller.go: subscription fetch gate, not native wire auth |
| Per-client raw statistics | U | N | U | U | N | U | N | TUIC job deliberately emits zero-byte client rows |
| Inbound statistics | U | U | U | U | U | U | U | Xray stats and backend jobs; units differ |
| Client up/down rate shaping | N | N | N | N | N | N | N | no shared authenticated client shaper |
| Billing multiplier / remainder | N | N | N | N | N | N | N | no durable billed total or revision |
| Crash/replay-safe billing cursor | N | N | N | N | N | N | N | Xray cursor memory-only; sidecar delta semantics |
| Logs | U | U | U | U | U | U | U | process logs and server controllers |
| Unified target/inbound routing | U | U | U | U | N | U | U | service/xray.go; A SOCKS bridge; M Xray egress |
| Authenticated per-client routing | U | N | U | U | N | N | N | session identity needs per-protocol E2E; M bridge identity unproved |
| Outbound selection / balancing | U | U | U | U | N | U | U | Xray router and outbound template |
| DNS settings | U | U | U | U | N | U | U | xray/dnsconf; A tunnel DNS and bridge |
| Subscription / format export | U | N | U | U | U | U | N | sub/raw, JSON, Clash; client-specific formats |
| Share links / QR | U | N | U | U | U | U | N | service/client_link.go, sub/links.go, frontend/lib/xray |
| API | U | U | U | U | U | U | U | controller/inbound.go, client.go; generated OpenAPI |
| Notifications | U | N | U | U | U | U | N | stats jobs, tgbot/discord/email; derives traffic/expiry |
| Backup / restore | U | U | U | U | U | U | U | database backup and settings upload paths |
| Portable client import/export | U | N | U | U | U | U | N | service/client_portable.go; selected fields explicit |
| Installer / updater | U | U | U | U | U | U | U | install.sh, update.sh; fork preservation missing |
| Docker | N | N | N | N | N | N | N | Node 22 build stage conflicts with required Node 26 |
| Multi-node mutation/sync | U | N | U | N | N | N | N | nodeEligibleProtocols in inbound_protocol.go |
| Global multi-node policy cap | N | N | N | N | N | N | N | snapshots provide totals, not shared bandwidth credit |
| Groups / LDAP integration | U | N | U | U | U | U | N | client group model; LDAP sync uses client bulk service |
| Subscription host overrides | U | N | U | U | U | U | N | model.Host, sub/host_sub.go |
| Inbound fallbacks | U | N | N | N | N | N | N | VLESS/Trojan TCP TLS eligibility only within X |
| First-use delayed expiry | U | N | U | U | U | U | N | adjustTraffics; TUIC online signal zero-byte rows |
| Scheduled client reset / weekly renew | U | N | U | U | U | U | N | ClientRecord.ResetWeekday/TrafficReset and jobs |
| API token permissions / CSRF | U | U | U | U | U | U | U | middleware, panel/api_token.go, session |

All source paths in this table are under `internal/` unless prefixed frontend.
N includes incomplete applicable capability; it never means excluded scope.
Single-user Shadowsocks modes must use explicit exclusive ownership instead of
pretending a shared server credential identifies several independent users.
Unauthenticated HTTP/SOCKS/TUN/forwarding requires owned resource bindings.

## New first-class services

| Required capability | Snell v4 | Snell v5 | Snell v6 beta | SSH | mieru TCP | mieru UDP | TCP/UDP forwarding |
|---|---|---|---|---|---|---|---|
| CRUD / start-stop / client management | N | N | N | N | N | N | N |
| Credential / resource ownership / revocation | N | N | N | N | N | N | N |
| Bulk / API / validation / permissions | N | N | N | N | N | N | N |
| Raw counters / billed bytes / multiplier | N | N | N | N | N | N | N |
| Aggregate live up/down shaping | N | N | N | N | N | N | N |
| Quota / live cutoff / restart persistence | N | N | N | N | N | N | N |
| Expiry / renewal / reset / reason isolation | N | N | N | N | N | N | N |
| Online status / IP-device limits | N | N | N | N | N | N | N |
| Logs / diagnostics / notifications | N | N | N | N | N | N | N |
| Routing / egress / block / balancer | N | N | N | N | N | N | N |
| DNS / available domain identity | N | N | N | N | N | N | N |
| Subscription / share / valid config export | N | N | N | N | N | N | N |
| Backup / restore / import / export | N | N | N | N | N | N | N |
| Install / upgrade / rollback / uninstall | N | N | N | N | N | N | N |
| Docker / platform capability gating | N | N | N | N | N | N | N |
| Multi-node / source deduplication | N | N | N | N | N | N | N |
| Real-client TCP / UDP interoperability | N | N | N | N | N | N | N |

## Protocol-specific applicability

Standard SSH direct-tcpip/forwarded-tcpip channels carry TCP only: native UDP
and a fabricated SSH UDP subscription node are not applicable to those modes
([RFC 4254 §7](https://www.rfc-editor.org/rfc/rfc4254#section-7)). An extra UDP
encapsulation scheme would be a separate capability, not claimed here.
Xray TLS/REALITY/XTLS fields are not Snell, SSH or mieru server options; their
own authentication and transport fields must be used instead. This excludes
invalid fields, never common management features.

No common management feature is marked not applicable because of backend
difficulty or missing runtime support. Snell IP-only egress domain visibility,
MTProto egress identity and TUIC attribution remain unresolved requirements.
