# Node traffic snapshots (#7007)

## Problem and decision

The five-second poll used `Remote.FetchTrafficSnapshot` -> `/panel/api/inbounds/list` -> `InboundService.GetInbounds`: full settings, traffic preloads, UUID/SubID enrichment and batched shared-client backfills. The master then parsed settings and ran membership reconciliation even for an unchanged node. Database-wide production statistics alone cannot attribute every matching query to this endpoint.

This change combines a versioned node endpoint with durable structural revisions. A stateless counter-only projection cannot safely drive the existing adoption/removal merge. Hashing every settings document retains the expensive read; application-only invalidation misses SQL/background writers. Per-row database tokens avoid both without a revision table, index, external cache or a writer hotspot shared by every client.

## Protocol and storage

`GET /panel/api/inbounds/trafficSnapshot?revision=<opaque revision>` uses existing API authentication, TLS/mTLS and outbound transport. Admin, node-admin and node-sync scopes are allowed; monitor and unauthenticated requests cannot read the catalog.

Every successful v1 response contains complete inbound counters, unique raw client counters/lifecycle metadata, last-online data and online/active GUID trees. Display UUID/SubID enrichment, subscription-fetch timestamps and global display overlays are excluded. The structural catalog contains inbound configuration and canonical email attachments.

- Initial/changed revision: present, complete `structure`.
- Unchanged revision: only `structure` is omitted; counters remain complete and fresh.
- Valid empty catalog: present structure with empty inbound/attachment arrays, plus empty counter arrays.
- Failed/unknown/malformed response: no merge. Missing/duplicate inbound counters, unknown omitted structure and incomplete/changed client rosters under an unchanged revision are rejected.

The Remote reconstructs a complete `TrafficSnapshot` from validated catalog history. It never passes omitted inbounds to the full deletion/adoption code. A context-aware per-Remote gate orders fetches; callers receive separate inbound objects and fresh counter slices. Errors discard history so recovery requests complete structure.

Hidden, GORM-read-only `snapshot_revision` columns on `inbounds` and `clients` are maintained by SQLite/PostgreSQL triggers, including direct SQL. Tokens change only on actual structural edits. All persisted inbound fields except up/down are covered; all canonical client fields except timestamp-only `updated_at` are covered. Link IDs, flow overrides and attachment timestamps are projected directly. Traffic identity/lifecycle metadata is hashed without usage or online timestamps. Process and database-connection epochs invalidate history after restart/reopen/migration.

A consistent node read selects versions, membership tokens and the required raw counter columns. Settings and attachment emails are loaded only on revision mismatch. PostgreSQL uses repeatable-read/read-only transactions. SQLite uses a pinned deferred WAL reader with cancellation rollback, avoiding reservation of the traffic writer.

The master validates its own inbound/client/link revisions, lifecycle metadata, filtering/alias set and node identity. Cache admission requires a successful stable merge with matching complete memberships. Dirty/just-pushed ticks, lifecycle changes, incomplete adoption, failed sync and local edits use the established full path. Counter/baseline/reset logic runs every poll. Deleted, disabled and offline nodes lose master cache entries.

## Compatibility and accounting

Successful API responses advertise `traffic-snapshot-v1` in `X-3x-Node-Caps`. Cold connections can bootstrap through the legacy list. Old nodes retain the legacy path. Only HTTP 404/405 classify the endpoint as unsupported, using complete legacy data with a five-minute retry cooldown. Capability transitions and new runtime connections re-evaluate support. HTTP 401/403, timeouts and server errors remain errors. Protocol validation failures fail closed, clear history and are not interpreted as empty authoritative data.

The common accounting loop retains per-node/email baselines, max-folded shared counters, reset clamps, owed resets, frozen lifecycle, renewals, tombstones, degraded-snapshot guards, ownership checks and GUID attribution. Structural sweeps and `SyncInbound` are skipped only after both sides validate unchanged state. Depletion maintenance, global pushes and heartbeat freshness do not depend on a catalog refresh. Cadence and concurrency limits are unchanged.

Full/slim/detail API contracts remain unchanged. The full loader's private database injection enables independently backed measurements; public origin/fallback/identity behavior is preserved. Neither slim lists nor depletion predicates consume the snapshot DTO. #7008 and #7009 remain separate changes.

## Measurement method

`TestNodeSnapshotCycleScale` uses real databases, HTTP servers, Remote decoding and `SetRemoteTraffic`. It compares the preserved legacy path against v1 on the same migrated schema. Each case resets fixtures, warms five cycles and samples twelve cycles. Cases cover 1,000/10,000/20,000 clients, one/eight inbounds and one/two peer nodes. Eight-inbound fixtures attach each client twice and include both attachments in settings. The peers read one controlled node fixture through separate Remotes; the master has its own database. This is synthetic fanout, not separate production panel processes.

Measurement handlers invoke production storage builders with explicit user/GUID, excluding auth and the GUID-setting lookup equally in both arms. Real router/auth and multi-process tests are separate. Timed cycles cover fetch plus merge, excluding scheduling, depletion/global/IP jobs and websocket broadcasts. Counter cases advance 100 clients before timing; resulting master writes are measured. Add/remove/config events each measure one immediate refresh.

SQL counts exclude transaction control. SELECT rows returned are recorded separately from driver-reported affected rows. Summed GORM trace durations include scanning, callbacks and overlapping nested preload time; they are neither server execution time nor CPU. PostgreSQL `pg_stat_statements.total_exec_time` is collected separately, including transaction execution but excluding fixture updates. The extension/reset privilege is optional benchmark infrastructure, never a runtime dependency.

Go allocations cover each cycle. GC runs before timing/allocation sampling; CPU profiles also include those forced collections. PostgreSQL server CPU is outside Go profiles. p50/p95 use nearest ranks 6/12 from twelve samples, not production latency guarantees. `TestNodeSnapshotScale` separately measures storage, JSON encoding/decoding and allocations with matching shared-client fixtures.

Measured on 2026-10-29, Linux/amd64 Docker, Go 1.27.3, `GOMAXPROCS=4`, SQLite and PostgreSQL 18. The host exposed 16 logical CPUs; this is not the production two-vCPU machine. All 12 topology/size cases passed on each database, with 48 steady-state scenario/arm rows and 72 edit-event rows per database.

### Fetch plus merge latency (eight inbounds, two peers)

| Database | Clients | Scenario | p50 legacy / v1 (ms) | p95 legacy / v1 (ms) | p50 speedup |
| -------- | ------: | -------- | -------------------: | -------------------: | ----------: |
| sqlite   |   1,000 | idle     |       201.71 / 49.80 |       210.79 / 55.12 |       4.05x |
| sqlite   |   1,000 | counters |       209.02 / 62.74 |       219.26 / 66.83 |       3.33x |
| sqlite   |  10,000 | idle     |     1841.66 / 422.04 |     2633.93 / 436.44 |       4.36x |
| sqlite   |  10,000 | counters |     2533.78 / 453.55 |     3601.30 / 553.41 |       5.59x |
| sqlite   |  20,000 | idle     |     3660.05 / 859.34 |     3887.16 / 893.04 |       4.26x |
| sqlite   |  20,000 | counters |     3642.94 / 881.21 |     3797.47 / 922.19 |       4.13x |
| postgres |   1,000 | idle     |       427.13 / 66.03 |       445.92 / 80.03 |       6.47x |
| postgres |   1,000 | counters |      476.94 / 113.21 |      483.74 / 132.35 |       4.21x |
| postgres |  10,000 | idle     |     2125.99 / 541.65 |     2210.80 / 569.30 |       3.93x |
| postgres |  10,000 | counters |     2168.30 / 585.50 |     2254.90 / 620.23 |       3.70x |
| postgres |  20,000 | idle     |    4434.61 / 1026.38 |    5106.25 / 1050.07 |       4.32x |
| postgres |  20,000 | counters |    4433.07 / 1092.56 |    4523.11 / 1172.83 |       4.06x |

### Work per cycle (20,000 clients, eight inbounds, two peers)

Values are legacy / v1. SQL and SELECT-row totals combine both roles. MiB uses 1,048,576 bytes.

| Database | Scenario |  SQL calls |           Rows read |     Wire MiB |    Allocated MiB |       Allocation count | PostgreSQL execution ms |
| -------- | -------- | ---------: | ------------------: | -----------: | ---------------: | ---------------------: | ----------------------: |
| sqlite   | idle     |   738 / 98 | 1,180,936 / 420,056 | 43.87 / 9.13 | 2775.46 / 675.77 | 24,632,741 / 6,228,450 |                     n/a |
| sqlite   | counters | 1138 / 498 | 1,180,936 / 420,056 | 43.87 / 9.14 | 2783.63 / 676.66 | 24,667,281 / 6,262,611 |                     n/a |
| postgres | idle     |   738 / 98 | 1,180,936 / 420,056 | 43.87 / 9.13 | 2955.94 / 688.62 | 25,547,955 / 6,305,494 |        1497.56 / 371.15 |
| postgres | counters | 1138 / 498 | 1,180,936 / 420,056 | 43.87 / 9.14 | 2947.64 / 692.61 | 25,587,681 / 6,344,891 |        1352.26 / 387.31 |

Every v1 steady-state case reported **zero full catalog fetches and zero structural polls** across its twelve measured cycles. Each add/remove/config event refreshed every affected peer immediately (one full fetch per peer). Those events intentionally retain full reconciliation cost.

### Isolated node JSON work (20,000 clients)

Encoding/decoding figures exclude the master merge and HTTP envelope.

| Database | Inbounds | Encode legacy / v1 (ms) | Decode legacy / v1 (ms) |
| -------- | -------: | ----------------------: | ----------------------: |
| sqlite   |        1 |           80.83 / 20.78 |           61.52 / 26.81 |
| sqlite   |        8 |          139.23 / 21.66 |          118.13 / 27.60 |
| postgres |        1 |           80.02 / 21.21 |           60.10 / 25.94 |
| postgres |        8 |          138.85 / 20.73 |          120.17 / 26.76 |

### CPU profiles (20,000 clients, eight inbounds, two peers)

Total sampled Go-process CPU for twelve cycles, including explicit GC, excludes PostgreSQL server CPU.

| Database | Scenario | Legacy CPU seconds | v1 CPU seconds |
| -------- | -------- | -----------------: | -------------: |
| sqlite   | idle     |              56.94 |          14.34 |
| sqlite   | counters |              56.38 |          14.64 |
| postgres | idle     |              55.20 |          14.49 |
| postgres | counters |              55.23 |          15.25 |

The SQLite idle profile reduced `runtime.cgocall` cumulative samples from 11.88s to 3.32s, alongside lower JSON scanning/copying and GC work. Counter scan/conversion and raw transport remain visible in the optimized profile; this is reduced work, not elimination of traffic processing.

Raw results: [SQLite cycles](cycles-sqlite.csv), [PostgreSQL cycles](cycles-postgres.csv), [SQLite events](events-sqlite.csv), [PostgreSQL events](events-postgres.csv), [SQLite JSON projection](projections-sqlite.csv), [PostgreSQL JSON projection](projections-postgres.csv), [CPU samples](cpu-profiles.csv). All one-inbound/one-peer results are included there.

## Reproduction

Use Go from `go.mod`, CGo, a C compiler and normal frontend/dist prerequisites. Fixtures contain generated test clients.

```sh
mkdir -p .cache/node-snapshots
GOMAXPROCS=4 XUI_SCALE_TEST=1 XUI_SNAPSHOT_PROFILE_DIR="$PWD/.cache/node-snapshots/sqlite" go test ./internal/web/service -run '^TestNodeSnapshotCycleScale$' -count=1 -timeout 30m -v
# Use a throwaway PostgreSQL database. Optional stats need pg_stat_statements preloaded and installed.
GOMAXPROCS=4 XUI_DB_TYPE=postgres XUI_DB_DSN="$TEST_DSN" XUI_SNAPSHOT_PG_STATS=1 XUI_SNAPSHOT_PROFILE_DIR="$PWD/.cache/node-snapshots/postgres" go test ./internal/web/service -run '^TestNodeSnapshotCycleScale$' -count=1 -timeout 40m -v
XUI_SCALE_TEST=1 go test ./internal/web/service -run '^TestNodeSnapshotScale$' -count=1 -timeout 10m -v
go tool pprof -top .cache/node-snapshots/sqlite/compact-idle-20000-8-2.cpu
```

Omit `XUI_SNAPSHOT_PG_STATS` when the optional extension is unavailable; its metric becomes `n/a`. `XUI_SCALE_SIZES=1000` narrows sizes and `XUI_SNAPSHOT_MODE=compact` selects one arm.

## Validation

| Check                                                                                        | Actual result                                                                                                                                                                                                            |
| -------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `make verify`                                                                                | PASS: generation freshness, Go/frontend lint, typecheck, complete Go tests, 2,023 frontend tests in 215 files, panel build, Storybook build and matching docs OpenAPI.                                                   |
| `go test -race -shuffle=on -count=1 -timeout 25m ./internal/web/... ./internal/database/...` | PASS; service 234.102s, database 84.520s; no race reports.                                                                                                                                                               |
| PostgreSQL regression gate                                                                   | Six top-level tests PASS, including both trigger dialects and the real PostgreSQL storage/merge/reopen/cancellation cases; no skipped PostgreSQL cell.                                                                   |
| `TestMasterNodeContract`                                                                     | PASS with real production router for admin and node-sync; negotiated v1 exercised repeatedly. Controller tests additionally cover node-admin acceptance, monitor denial, unauthenticated denial and verified mTLS scope. |
| `XUI_NODE_E2E_OLD_BINARY=... make node-e2e`                                                  | PASS, 442.272s, no skips: ordinary sync/scopes/offline/filter/reset/shared-IP cases, all four old/new pairs, new/new node and master restarts, three-panel global-heartbeat/quota chain (276.52s).                       |
| Thirty-second `FuzzParseLink`                                                                | PASS, 504,158 executions.                                                                                                                                                                                                |
| Thirty-second `FuzzDecodeCertPin`                                                            | PASS, 366,154 executions.                                                                                                                                                                                                |
| `make vulncheck`                                                                             | PASS: zero reachable/imported-package vulnerabilities; three findings only in required modules whose affected code is not called.                                                                                        |
| Docs site checks                                                                             | PASS: formatter, lint, typecheck, 112 tests and production build.                                                                                                                                                        |
| `actionlint`                                                                                 | PASS for the updated CI workflow.                                                                                                                                                                                        |
| OpenAPI compatibility comparison                                                             | No existing path/component changed; one route and five schemas added.                                                                                                                                                    |
| Both complete scale ladders and JSON projection ladders                                      | PASS; all result CSVs above are from the final minimal DTO.                                                                                                                                                              |

The old matrix binary was built from exact upstream commit `c621ba9014a6aa21fd7c9ecf45655e44e24ebf13`; the new binary includes this change. The normal harness initializes temporary databases, which also exercises old-process compatibility with the migrated schema. CI fetches/builds that pinned baseline and provides `XUI_NODE_E2E_OLD_BINARY`; no optional matrix cell is silently omitted in CI.

A rehearsal hit Go's default ten-minute benchmark timeout; the full measured sweeps above passed with explicit limits. The first observer-based E2E run exposed a test-proxy bug reading gzip as JSON. The inspector was corrected to decode for inspection while forwarding original bytes, then the entire E2E suite passed. Those failed rehearsals are not counted as successful checks.

Focused regressions were observed failing without their protections: redundant settings reads/reconciliation, SQLite writer reservation, missing legacy tokens, database-reopen history and truncated unchanged counters. Tests also cover direct/canonical/lifecycle edits, detach, no-op/timestamp writes, rollback, cancellation, empty/full/unchanged catalogs, authorization, unsupported endpoints, shared backfills and exact deltas across repeated/lower counters.

## Architecture audit

1. **Unchanged settings reads:** none in the node builder or validated master steady-state merge. Cold/change/recovery uses full structure.
2. **Identity JOINs:** UUID/SubID enrichment and email-IN backfill batches are removed from v1. Projected token/membership JOINs remain O(clients + attachments); no constant-time claim.
3. **Reconciliation:** stable validated revisions skip adoption/sweeps, settings parsing and `SyncInbound`; lifecycle/depletion maintenance continues.
4. **Partial deletion:** omission needs matching validated history and complete counter rosters. Failed data never enters the merge; valid present empty catalogs retain full semantics and existing guards.
5. **Prompt edits:** triggers, link projections and lifecycle hashes detect config/add/remove/disable changes at the next poll, including SQL/background writers.
6. **Mixed versions:** legacy API for old masters, legacy fallback for old nodes, negotiated v1 for new pairs.
7. **Restarts:** connection-local Remote history plus boot/database-generation epochs require complete acquisition; persisted accounting baselines survive.
8. **Errors:** no partial merge; clear history, retain baselines and request full structure on recovery. Auth failures are not downgrade signals.
9. **Duplicate usage:** unchanged per-node/email baselines and max-folded shared counters apply each positive delta once.
10. **Reset safety:** reset ordering, grace periods, owed resets and frozen-dip baseline protection remain in the common loop.
11. **Global accounting:** overlays never become local counters; freshness, heartbeat and quota predicates are unchanged.
12. **#7008:** depletion SQL/freshness candidate selection remains independently changeable.
13. **#7009:** list consumers can select independent projections without full settings or a dependency on the node DTO.
14. **Contracts:** existing full/slim/detail schemas are unchanged; endpoint/schemas are additive.
15. **Benefit:** matched measurements, raw CSVs and reproduction commands above.

## Costs and limits

Token/lifecycle projections still scale with client/attachment count, and complete fresh counters cross the network each tick. Catalog memory scales per active node. Structural changes intentionally pay full reconciliation cost. SQLite structural edits incur a token-trigger update; PostgreSQL stamps the row before writing. Counter-only writes do not fire structural triggers. Migration adds two columns, initializes legacy tokens and installs triggers with normal database-owner migration privileges.

The scale harness isolates fetch/merge and reaches two fanout peers. Real process/version/restart and nested-node behavior is tested separately; cadence-bound integration duration is not an isolated nested performance benchmark. Response-size limits remain unchanged.

## Review hardening (#7014)

Cached reconciliation retains the validated remote inbound ID → central inbound ID association, including deployed aliases whose tags differ. A cached poll validates the complete association and its membership coverage before restoring aliases. Missing, duplicate, foreign-node or incompatible targets force full reconciliation. Alias restoration occurs after lifecycle validation so a full tick still performs normal alias discovery and orphan protection. Counter resets, quota disablement and renewal continue through the existing traffic merge.

The compact decoder enforces these limits before allocating typed collections:

| Collection                                    |                                                 Limit |
| --------------------------------------------- | ----------------------------------------------------: |
| Structural inbounds and inbound counters      |                                   65,536 entries each |
| Canonical client counters and attachments     |                                1,048,576 entries each |
| Flat online email list                        |                                     1,048,576 entries |
| Each GUID-keyed online/active/last-online map | 1,048,576 aggregate JSON values, including containers |

The HTTP response byte limit remains in force. Embedded legacy `clientStats` lists in structural rows are bounded and discarded because the compact counter collection replaces them. Invalid collections, malformed responses and semantic protocol errors invalidate history and require a full catalog on recovery. Only HTTP 404/405 can enable legacy fallback; decode errors never downgrade to the legacy endpoint.

Revision triggers use unscoped update events with actual-change guards. Bootstrap removes previous revision triggers before AutoMigrate and handwritten migrations, then recreates them afterwards. This also handles SQLite trigger bodies that mention retired columns. PostgreSQL type changes no longer encounter column dependencies from `UPDATE OF`. Counter-only, timestamp-only and no-op writes retain tokens; direct configuration writes change them. Database/process epochs still invalidate histories across migration or reopening.

The review regressions cover cached alias traffic and resets, quota/disable/renewal state, whole-mapping fallback, unchanged-catalog reads, malicious and boundary collections, malformed-response recovery, and SQLite/PostgreSQL DDL, rollback and reopen behavior. The PostgreSQL CI step runs the new migration and alias cases with a required pass-count guard. `BenchmarkSnapshotRevisionSteadyState` compares scoped-event baseline triggers with generic-event triggers on both engines; `BenchmarkCompactSnapshotDecode` compares the original unbounded decoder with the bounded decoder on 20,000 counters.

### Local hardening measurements

The following are synthetic local measurements on the same Go 1.27.3 toolchain with `GOMAXPROCS=2`, PostgreSQL 16 and SQLite. Values are medians of three runs; trigger benchmarks used 500 ms per run and the decoder benchmark used one second. They measure the stated operation, rather than production latency.

| Operation                                                                                           |    Scoped/unbounded baseline |     Hardened implementation |
| --------------------------------------------------------------------------------------------------- | ---------------------------: | --------------------------: |
| SQLite: update eight inbound counters, 4,042,344 bytes of configuration / 20,000 configured clients |                      4.54 ms |                     4.84 ms |
| PostgreSQL: same eight-counter update                                                               |                     0.687 ms |                    1.412 ms |
| Decode 20,000 compact client counters                                                               | 21.38 ms; 13.82 MB allocated | 42.78 ms; 6.56 MB allocated |

Removing column-scoped events adds approximately 0.30 ms (SQLite) / 0.73 ms (PostgreSQL) per representative eight-inbound write. The decoder intentionally performs extra validation and counting passes before typed allocation; this halves allocated bytes in this fixture, at roughly 21 ms additional decode time per 20,000-client response. These are explicit validation costs, not claims that bounded decoding itself is faster. The optimization remains enabled and unchanged aliases are covered by assertions that reject full catalog reads.

Repeat the isolated measurements with:

```bash
go test ./internal/database -run '^$' -bench '^BenchmarkSnapshotRevision(SteadyState|CatalogCounters)$' -benchtime=500ms -count=3
go test ./internal/web/runtime -run '^$' -bench '^BenchmarkCompactSnapshotDecode$' -benchtime=1s -count=3
```

Set `XUI_TEST_PG_DSN` to an isolated PostgreSQL test database for its benchmark cases. The trigger benchmark builds a scoped-event baseline with the same change guards, isolating event-dispatch overhead; database migration and fixture construction are outside the timed region.

Matched end-to-end fetch/merge reruns on SQLite used the same 20,000-client, eight-inbound fixture, one/two peers, two attachments/client, five warmups and twelve samples per scenario, with `GOMAXPROCS=2`:

| Peers | Scenario | Legacy p50 / p95 | Compact p50 / p95 | Node/master SQL per cycle, legacy → compact | Full master structural polls, legacy → compact |
| ----: | -------- | ---------------: | ----------------: | ------------------------------------------- | ---------------------------------------------: |
|     1 | Idle     |   2538 / 2730 ms |      702 / 922 ms | 55/448 → 4/45                               |                                         12 → 0 |
|     1 | Counters |   2489 / 2584 ms |      630 / 683 ms | 55/648 → 4/245                              |                                         12 → 0 |
|     2 | Idle     |   5556 / 6621 ms |    5432 / 5506 ms | 110/896 → 8/900                             |                                        24 → 24 |
|     2 | Counters |   5538 / 8187 ms |    5488 / 5693 ms | 110/1296 → 8/1300                           |                                        24 → 24 |

The one-peer case keeps master caching active. The two-peer fixture shares every canonical client across peers; existing shared-claim safeguards intentionally require full master reconciliation, so it demonstrates reduced node query/wire work rather than a large master CPU reduction. Those safeguards are preserved. Compact idle wire traffic was 4,789,320 versus 23,001,295 bytes for one peer, and 9,578,640 versus 46,002,590 bytes for two peers. All compact unchanged samples avoided a full node catalog fetch. Add/remove/config events refreshed the affected peers in both cases.

These current runs supersede any assumption that the earlier full-ladder speedup applies unchanged to shared-claim workloads after the upstream ownership hardening. The earlier CSVs and PostgreSQL 18 ladder remain historical measurements; the current trigger microbenchmarks used PostgreSQL 16, and the current end-to-end reruns above used SQLite only.

```bash
XUI_SCALE_TEST=1 XUI_SCALE_SIZES=20000 go test -run '^TestNodeSnapshotCycleScale/20000/8/(1|2)$' -count=1 -timeout=20m -v ./internal/web/service
```
