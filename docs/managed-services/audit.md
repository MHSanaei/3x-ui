# Baseline and source audit

Audit date: 2026-09-28 UTC. Clean fresh clone; no user edits overwritten.
No AGENTS.md exists in the clone. Read root and frontend CLAUDE.md,
README.md, CONTRIBUTING.md, frontend/README.md, docs/architecture.md,
Makefile, Dockerfile, DockerInit.sh and the model/migration/traffic sources.

## Provenance

| Item | Evidence / fixed value |
|---|---|
| Authorized destination | `0x3f3f3f3f3f3f3f3f3f3f3/3x-ui` |
| Actual parent and source | GitHub repository API: `MHSanaei/3x-ui` |
| Latest stable upstream | `v3.8.5`, published 2026-09-16, `prerelease=false` |
| Stable tag object | `648d0fc71511a679f241179cbe06014809c5b0d6` |
| Stable peeled commit | `7ef22f94c950ff09f0870e2295fa65ad5968742c` |
| Development baseline | `17d7dd46b512d0a9c22921a6094f30c672e436c9` |
| Difference | Fork HEAD equals upstream main at audit; 64 commits after stable tag |
| Branch | `feat/unified-client-policy-backends` |
| Go | `go.mod`: 1.27.1; locally installed official linux/arm64 archive |
| Frontend | `.nvmrc`: Node 26; local Node v26.10.0 |
| Xray Go dependency | `v1.260327.1-0.20260908222543-52a412d9e2f5` |
| Packaged Xray binary | `DockerInit.sh`: v26.9.9 (binary execution not yet verified) |
| Xray release API discrepancy | `/releases/latest` returned v26.3.27; do not downgrade the existing pinned core from this response |
| Existing TUIC | `tuic-server-1.0.0` in DockerInit.sh |
| Existing MTProto | DockerInit.sh resolves mtg-multi latest dynamically; needs pinning |
| Existing AmneziaWG Go | `v3.1.20260828` in go.mod |
| New mieru candidate | Official stable v3.38.0, published 2026-09-24 |
| Snell compatibility targets | Protocol v4 / server 4.1.1; v5 / server 5.0.1; v6 / server 6.0.0rc2 (beta) |
| SSH implementation | Proposed Go x/crypto/ssh from existing v0.57.0 dependency |

Preserve the newer baseline: post-release changes include renewal races, reset
ordering, portable traffic export and WireGuard overlap checks. No rollback to
the older release; no fork-specific delta existed at audit.

Sources: [fork metadata](https://api.github.com/repos/0x3f3f3f3f3f3f3f3f3f3f3/3x-ui),
[3x-ui release](https://github.com/MHSanaei/3x-ui/releases/tag/v3.8.5),
[mieru release](https://github.com/enfein/mieru/releases/tag/v3.38.0),
[Snell release notes](https://kb.nssurge.com/surge-knowledge-base/zh/release-notes/snell),
[Snell v6 announcement](https://nssurge.com/blog/snell-v6).
Snell v5 documents v4 backward compatibility, but each target still needs its
own real Surge test. v6 may change incompatibly during beta. No permission to
redistribute proprietary binaries was established; use verified official
downloads on the destination pending license review. No compatible client is
being reverse engineered.

## Source findings

1. `model/model.go`: `ClientRecord` is the canonical client, `ClientInbound`
   attaches it to multiple inbounds. Email currently keys traffic; numeric
   row IDs and email alone are not safe deletion/recreation or distributed
   identities. Add an immutable policy incarnation; retain public credentials.
2. `database/db.go`: GORM AutoMigrate plus explicit repair/seeder steps.
   `allModels()` is also important for complete backup/migration coverage.
   Both SQLite and PostgreSQL are supported. Never implement SQLite-only SQL.
3. `xray/api.go:GetTraffic`: non-reset cumulative core counters, but an
   in-memory cursor and first-poll baseline discard. It does not provide the
   requested crash-consistent, duplicate-safe billing boundary.
4. `service/inbound_traffic.go`: raw up/down additions, raw quota predicates,
   renewal and disable mutation batches through Runtime. No client billing
   multiplier, remainder or data rate policy is present.
5. `job/xray_traffic_job.go`: live-session cutoff depends on optional
   restart-on-disable. Removing authentication alone does not stop established
   traffic; a whole-core restart also affects unrelated users.
6. `runtime/runtime.go`, `service/inbound_node.go`: all mutations must use
   local/remote Runtime. Node snapshots and global traffic mirrors already
   avoid some hierarchical double counting, but lack policy epochs and a
   globally allocated bandwidth budget.
7. `tuic/relay.go`, `tuic/manager.go`, `job/tuic_job.go`: encrypted UDP bytes
   are attributed to an inbound. The client job emits zero-byte rows only
   for online/first-use activation. This is NOT per-client metering.
8. `amneziawgnet/relay.go`: peer-specific SOCKS auth carries email into Xray.
   This is a useful identity bridge, but its UDP attribution and traffic
   boundary require real concurrent-peer verification.
9. `mtproto/manager.go`, `service/inbound_mtproto.go`: per-secret native
   limits and management reload; Xray egress exists. Verify user identity at
   that bridge before claiming per-user routing or adding another meter.
10. `service/inbound_protocol.go`: node eligibility excludes sidecar protocols;
    generic CRUD alone will not make new sidecars work on remote nodes.
11. `service/client_portable.go`, `database/backup.go`, `migrate_data.go`:
    explicit import/export and cross-dialect copy paths need new state.
12. `frontend/src/lib/xray/protocol-capabilities.ts` and protocol schemas
    control field applicability; new fields must not be copied onto invalid
    transports. Generated API files come from `tools/openapigen`.
13. `install.sh`, `update.sh`, `x-ui.sh`, release workflows point at official
    upstream releases; fork channel isolation is required before shipping.
14. Dockerfile uses Node 22 despite Node 26 frontend requirements, and Alpine
    libc differs from Snell's documented glibc requirement. Packaging needs
    correction and actual image tests.

## Decisions

- Reuse the existing client, Runtime, routing editor, subscriptions, auth,
  notifications and backup flows; no independent management website.
- Choose application payload shaping at authenticated dispatch/bridge paths
  where possible. IP policing and shared-listener shaping cannot satisfy the
  client identity requirement.
- Evaluate nftables only inside an isolated network namespace. Userspace
  forwarding is a viable first-class backend for portable routing and identity;
  kernel NAT alone is insufficient. A later kernel backend must use dedicated
  project resources and prove flow-offload/conntrack correctness.
- Ordinary design choices and feature-branch pushes are already authorized by
  the brief. Record decisions and continue without repeated approval gates.
