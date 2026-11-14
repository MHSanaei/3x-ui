# Global-aware depletion query measurements (#7008)

The recurring depletion scan now builds global quota candidate IDs with an
uncorrelated `IN (SELECT ... JOIN ...)` instead of doing a correlated global lookup
for each enabled, limited client. A shared SQL `EXISTS` probe replaces the
freshness `COUNT(*) ... LIMIT 1` in all three read-side callers. There is no schema,
index, ingestion, polling, transaction-boundary, or dependency change.

## Callers and preserved behavior

- `XrayTrafficJob.Run` calls `InboundService.AddTraffic` every five seconds.
  `addTrafficLocked` commits usage first, then runs automatic renewal, client
  disable and inbound disable in the existing separate lifecycle transaction.
- `disableInvalidClients` uses the new set-based scan. The existing collected-ID
  updates, settings edits, canonical record updates, subscription invalidation,
  local Xray removals, remote plans and dirty-node tracking are unchanged.
- `ClientService.BulkAdjust` also calls `depletedCond` to identify previously
  disabled, depleted clients. It retains the original correlated predicate for
  its email-scoped queries; a whole-table candidate join is unnecessary there.
- The identical freshness count also appeared in `overlayGlobalTraffic` (UI and
  forwarded node totals) and `newClientQuery` (paged clients). Both now use the
  same short-circuit probe. Consequently the issue's 7,242 count executions
  cannot all be attributed to the depletion job.

Freshness is still `updated_at >= now - 24h`, using one `NowFunc` value per
depletion evaluation. Empty/all-stale global tables still select the local-only
predicate from #5392. Each invocation queries the database; there is no cache
that can hide a newly refreshed master, lower total or zero reset. The existing
probe-error fallback is preserved.

For global depletion, **one fresh master's `up + down` must reach that client's
quota**. Values are neither summed across masters nor combined direction by
direction. `IN` gives set membership even when several masters qualify. Local
quota and expiry remain independent OR branches. Unlimited/negative quotas,
enabled filtering, delayed-start expiry, and bulk renewal behavior are unchanged.
The inner and outer client reads share the statement's database snapshot.
Global traffic ingestion/upserts from #6931, depletion metadata from #6883, and
the independent #7007/#7009 work are outside this change.

## Environment and method

Base: upstream `c621ba9014a6aa21fd7c9ecf45655e44e24ebf13`.
Linux/amd64 Docker, Go 1.27.3 (`GOMAXPROCS=4`), PostgreSQL 18.6; the Docker host
exposes 16 logical CPUs. These are disposable local fixtures, **not the issue's
production two-vCPU machine**. No production database was queried or changed.

The opt-in harness uses the normal migrated schema and PostgreSQL isolated-schema
test helper. It creates 1k/10k/20k local traffic rows: 80% enabled, 90% with a
positive quota, spread over eight inbound IDs. Global rows cover 70% of emails.
The multi-master fixture has three masters (two fresh, one stale). The mixed
fixture has 0.5% local-quota candidates, 0.5% expiry candidates and 0.5% additional
global-quota candidates, all enabled. Ordinary active-global fixtures return
zero depleted rows. Quotas, counters, expiry and emails are deterministic.

Each shape gets five warmups and 30 measured read-only cycles. Client timings
include the freshness probe, GORM and the complete traffic SELECT/result decode;
the `local-only` control omits the probe. Fixtures are identical across arms.
Candidate IDs must exactly match the original SQL in every arm. PostgreSQL runs
`VACUUM ANALYZE`, SQLite runs `ANALYZE`; no planner switches are forced. Only
fixture setup writes rows. The cycles are accelerated repeated polls, not an
elapsed production observation window; application scheduling stays at five
seconds.

`server_ms`, root buffer counts, estimates, actual rows and subplan loops come
from `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)` for the depletion SELECT alone.
Separately, `pg_stat_statements` before/after deltas cover matched 30-cycle windows
(probe plus depletion SELECT), excluding warmups, correctness comparisons and
EXPLAIN. Statistics are never reset. Only this benchmark accesses that disposable
PostgreSQL database while collecting those deltas. Blank SQLite statistics mean
unavailable, not zero work. Buffer visits and SQL execution time are not OS CPU
measurements. All measured PostgreSQL plans had zero shared disk reads; these
are warm-cache results.

## Confirmed plans and choice

The existing indexes were inspected: traffic primary key, unique email,
`inbound_id`, `(expiry_time, reset)`; global primary key, `email`, and unique
`(master_guid, email)`. The original correlated branch already uses
`idx_client_global_email`; its problem is repeated lookups, not a missing email
index. No speculative index was added, so there is no additional write/WAL/index
maintenance cost or migration for administrators.

At 20k rows with one fresh master:

```text
Before: Seq Scan client_traffics (estimated 9,840 rows, actual 0)
          SubPlan: Index Scan idx_client_global_email, loops=14,000
        execution=40.451ms, root shared hits=38,409

After:  Seq Scan client_traffics (estimated 10,400 rows, actual 0)
          hashed SubPlan: Hash Join, loops=1
            Seq Scan client_traffics c (positive quota)
            Hash -> Seq Scan client_global_traffics g (fresh)
        execution=6.912ms, root shared hits=991
```

The estimates remain conservative because quota expressions are not independent
columns. Actual candidate identities, not estimates, determine correctness. The
old large estimated cost also enables PostgreSQL JIT on these large fixtures;
the new plan avoids that overhead under the unchanged default settings.

The `UNION` alternative also removes correlated lookups, but adds another local
candidate scan/deduplication without a consistent advantage. With the same old
count probe, the 20k fresh fixture's set/union client medians were 6.9620/6.4116ms;
the mixed fixture was 6.8771/7.9803ms. The selected generic SQL works on SQLite as
well, preserves the local shortcut and avoids separate engine-specific paths.
The `current` benchmark arm additionally includes the new `EXISTS` probe.

The old freshness plan is `Limit -> Aggregate -> Seq Scan`: LIMIT bounds one
aggregate output row, but the scan counts all 14k fresh input rows. `EXISTS`
reads one qualifying input row (two buffer hits in this fixture). All-stale
input still requires a complete scan, which is necessary to establish absence.

## Results

PostgreSQL, 20k local rows (client median includes probe):

| Scenario           | Matches | Before median ms | After median ms | SELECT hits before/after | SELECT subplan loops before/after |
| ------------------ | ------: | ---------------: | --------------: | -----------------------: | --------------------------------: |
| Local-only control |       0 |           1.1645 |          1.1205 |                409 / 409 |                             0 / 0 |
| Empty globals      |       0 |           1.2861 |          1.2242 |                409 / 409 |                             0 / 0 |
| All globals stale  |       0 |           1.7091 |          1.7323 |                409 / 409 |                             0 / 0 |
| One fresh master   |       0 |          39.5666 |          5.6470 |             38,409 / 991 |                        14,000 / 1 |
| Multiple masters   |       0 |          42.9749 |          8.2399 |           38,409 / 1,337 |                        14,000 / 1 |
| Mixed depleted     |     300 |          40.4412 |          6.1788 |             37,809 / 991 |                        13,800 / 1 |

Matched `pg_stat_statements` windows at 20k rows, **30 complete read cycles**:

| Scenario         | Calls before/after | Cumulative SQL ms before/after | Shared hits before/after |
| ---------------- | -----------------: | -----------------------------: | -----------------------: |
| One fresh master |            60 / 60 |          1,166.9174 / 150.7275 |       1,157,460 / 29,790 |
| Multiple masters |            60 / 60 |          1,260.5666 / 223.4002 |       1,167,840 / 40,170 |
| Mixed depleted   |            60 / 60 |          1,193.3537 / 168.8257 |       1,139,460 / 29,790 |

For the fresh-master case this is about 87% less cumulative SQL time and 97%
fewer buffer visits. **Probe calls are not reduced:** there is still one probe
per evaluation. Their work is reduced: over 30 fresh-table probes alone, COUNT
took 16.4751ms and 5,190 hits, EXISTS took 0.2211ms and 60 hits. For three masters,
44.3922ms/15,570 hits became 0.0756ms/60 hits. Empty/stale behavior is unchanged
apart from normal timing variation.

Fresh-global client medians across sizes:

| Local rows | PostgreSQL before/after ms | SQLite before/after ms |
| ---------- | -------------------------: | ---------------------: |
| 1,000      |            1.7085 / 0.4899 |        0.2828 / 0.2424 |
| 10,000     |           17.7913 / 3.1576 |        2.7428 / 2.1356 |
| 20,000     |           39.5666 / 5.6470 |        5.9925 / 4.4604 |

SQLite's 20k multi-master median was 8.3381 -> 7.0925ms. There are no wall-clock
threshold assertions in correctness tests.

The evidence addresses the recurring global-aware SELECT responsible for the
reported 55.27ms production mean, and separately the count-probe work. It does
not claim to reproduce that mean or eliminate all traffic-job costs. The outer
traffic scan, positive-quota join/hash and full absence checks on stale global
tables remain O(table size). Bulk adjustments retain their original scoped
predicate. High write churn, cold cache, far more masters and different planner
statistics need deployment-specific measurement. No query-level improvement is
claimed for the all-stale case, nor for global ingestion or UI aggregate joins.

## Reproduce and evidence files

Use only a throwaway database for the PostgreSQL environment variables below.
The normal test helper creates and removes an isolated schema. Extension setup
is optional and must be done on that disposable database beforehand if
`XUI_DEPLETION_PG_STATS=1` is set.

```sh
mkdir -p internal/web/dist && touch internal/web/dist/.gitkeep

# SQLite
XUI_DEPLETION_MEASURE=1 XUI_DEPLETION_MEASURE_DIR="$PWD/.cache/depletion-sqlite" \
  go test ./internal/web/service -run '^TestDepletion(Query|Probe)Measurements$' -count=1 -v -timeout 20m

# PostgreSQL with pg_stat_statements enabled on the disposable test server
XUI_DB_TYPE=postgres XUI_DB_DSN="$TEST_PG_DSN" \
  XUI_DEPLETION_MEASURE=1 XUI_DEPLETION_PG_STATS=1 \
  XUI_DEPLETION_MEASURE_DIR="$PWD/.cache/depletion-postgres" \
  go test ./internal/web/service -run '^TestDepletion(Query|Probe)Measurements$' -count=1 -v -timeout 20m
```

- [PostgreSQL query matrix](global-depletion-7008-postgres.csv): 72 measurements
  (three sizes, six distributions, four SQL variants), p50/p95, plans and windows.
- [PostgreSQL probes](global-depletion-7008-postgres-probe.csv): 24 measurements.
- [SQLite query matrix](global-depletion-7008-sqlite.csv) and
  [SQLite probes](global-depletion-7008-sqlite-probe.csv).
- [Matched plans](global-depletion-7008-plans.txt): PostgreSQL node trees with
  costs, estimates, actual rows/loops, filters, buffers and execution times for
  every size/distribution before and after; count/EXISTS plans; SQLite query plans.
  The harness retains the complete source JSON plans and inspected index DDL.

## Validation

- SQLite and PostgreSQL depletion semantics, exact expiry/freshness boundaries,
  multiple masters without summing or combining directions, duplicate candidates,
  unlimited/delayed clients, increases/decreases/zero resets, idle heartbeat,
  already-disabled clients and repeated empty-result polls passed.
- A commit-barrier test on both engines observes the old committed global usage
  during an uncommitted concurrent write and the new high/zero value on the next
  evaluation after commit.
- Automatic renewal and capped catch-up preserve their different global-row
  cleanup behavior. Manual quota extension and manually disabled clients retain
  their existing enable behavior. These existing regression fixtures now run
  together on both engines, including in the PostgreSQL CI gate.
- PostgreSQL heartbeat regressions (including idle recovery, forwarding, reset,
  deletion and concurrent pushes) passed. The ingestion implementation is unchanged.
- `go test -race -shuffle=on -count=1 -timeout 25m ./internal/web/service ./internal/web/job`
  passed (service 149.254s, job 30.604s); `go build ./...`, golangci-lint (0 issues)
  and actionlint syntax validation (`-shellcheck=`) passed. The full actionlint
  invocation reports two existing SC2046 warnings in unchanged Go-package list
  expansion steps; the added PostgreSQL step has no warning.
- The complete `go test -shuffle=1793268532962520015 -count=1 ./...` rerun
  passed. An earlier full run with that seed failed the existing
  `TestUpdateInboundMtprotoUnchangedDoesNotRestart/remarkOnlyEditKeepsProcess`
  spawn-count assertion once. Five focused repetitions and the complete rerun
  passed; no unrelated MTProto code or test was changed. The first failure is
  not counted as a pass.
- `make node-e2e` passed without skips (312.765s): real master/node sync with
  admin and node-sync tokens, remote disable/offline/recovery, resets and shared
  clients, plus the three-panel global-heartbeat/quota chain (189.02s).
- The PostgreSQL measurement sweep passed (20.715s), SQLite sweep passed
  (8.265s), and query/probe correctness comparisons matched in every measured
  arm. PostgreSQL focused depletion and heartbeat regressions passed (9.866s).
- Frontend sources and API schemas are unchanged. Local validation focused on
  Go, database and real-process behavior; the upstream workflow runs its normal
  frontend/code-generation/documentation gates as well.
