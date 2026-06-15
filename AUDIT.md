# Test-Quality Audit Ledger

Running ledger for the test-quality audit of the existing test suites in the critical packages.
Goal: stronger tests (a test that fails when the behavior is wrong), not green tests.

**Cardinal rule:** strengthen/tighten assertions, never weaken or delete. If strengthening a test
reveals a real production bug, that is a **finding** logged here — production is *not* patched and
the test expectation is *not* relaxed during this audit.

Critical packages (priority): `internal/sub` → `internal/web/runtime` → `internal/database` +
migration → `internal/web/service` (node) → `internal/util/link` → `frontend/`.

Tooling: gremlins v0.6.0 (mutation), pgregory.net/rapid v1.3.0 (property), native `go test -fuzz`,
@vitest/coverage-v8 4.1.8 (frontend).

---

## 1. Smell Inventory

Status legend: `open` (not yet fixed) · `fixed` (strengthened) · `finding` (revealed a prod bug,
see §2) · `wontfix` (justified — note why).

| # | Smell | Location | Detail | Status |
|---|---|---|---|---|
| S1 | Over-broad (count-only) | internal/sub/service_dedup_test.go:59-64 | asserts only `len(links)==1` / `len(emails)==1`; dedup key (`strings.ToLower(client.Email)`) is unguarded | open |
| S2 | Over-broad (`err!=nil` only) | internal/web/runtime/tls_client_test.go:118-122 | `TestHTTPClientForNodePinInvalid` asserts only that *an* error occurred; doesn't pin the error or cover empty-pin | **fixed** (Phase B: table-driven, asserts specific error + empty-pin case) |
| S3 | Over-broad (key-absence only) | internal/sub/clash_service_test.go | `TestBuildProxy_VLESSNoneEncryptionOmittedForClash` checks only `proxy["encryption"]` absence, not the rest of the proxy | open |
| S4 | Happy-path-only (substring) | internal/sub/service_flow_test.go | `TestGenVlessLink_*` only `strings.Contains(link,"flow=…")`; no full-link / field-mapping assertion exists | open |
| S5 | t.Skip never runs in CI | internal/database/migrate_data_test.go:18,68 | both `MigrateData` tests skip without `XUI_TEST_PG_DSN` → effectively dead coverage of the migration batch loop | open |
| S6 | Coverage gap hiding a bug | internal/web/service/inbound_migration_test.go | seeds `inbound-0.0.0.0:30002` precondition but never asserts the tag cleanup → see Finding #1 | **finding** (Phase B: added `TestMigrationRequirements_CleansLegacyZeroAddrTag`; witnessed RED, then `t.Skip("FINDING #1")`) |
| S7 | Untested security branch | internal/web/runtime/tls_client.go:35-53 | `HTTPClientForNode` proxy+pin path (incl. `transport.TLSClientConfig = tlsCfg` pin injection) has zero coverage | **fixed** (Phase B: added proxy+pin & proxy+verify tests; mutation-sanity confirmed RED when pin injection dropped) |

(Additional smells appended as the static scan in Phase A completes.)

---

## 2. Findings (production bugs surfaced by strengthening)

### Finding #1 — legacy `0.0.0.0:` tag-cleanup migration never executes
- **Prod location:** [internal/web/service/inbound_migration.go:247](internal/web/service/inbound_migration.go#L247)
- **Bug:** `err = tx.Raw(tagCleanup).Error`. GORM's `.Raw()` *builds* but does not *execute* a
  non-SELECT statement — it requires a terminal `.Scan`/`.Exec`/`.Row`/`.Find`. Every other
  mutation in this file uses `tx.Exec(...)` (line 72) or `tx.Model().Update()` (line 234); the only
  working `.Raw()` (line 207) runs because it chains `.Scan(&externalProxy)`. So the legacy tag
  cleanup (`UPDATE inbounds SET tag = REPLACE(tag,'0.0.0.0:','') WHERE …`) is silently a no-op.
- **Why the test missed it:** `inbound_migration_test.go` seeds the exact precondition tag
  `inbound-0.0.0.0:30002` but only asserts `externalProxy` presence + the client_traffics backfill —
  never that the tag became `inbound-30002`.
- **Suggested fix (separate PR, NOT in this audit):** `tx.Exec(tagCleanup)`.
- **Audit action:** strengthen the test to assert `tag == "inbound-30002"`; it will fail (correctly),
  exposing the bug. Land it as `t.Skip("FINDING #1: tag cleanup uses tx.Raw (no-op); see AUDIT.md")`
  so it's visible and not hidden.
- **Status:** confirmed; test-strengthening + skip pending (Phase B).

### Finding #2 — global log ring buffer is an unsynchronized data race
- **Prod location:** [internal/logger/logger.go](internal/logger/logger.go) — the package-global
  `logBuffer` (decl ~line 34), written by `addToBuffer` (append/slice at lines 196-209) and read by
  `GetLogs` (lines 213-220). **No mutex anywhere** in the file.
- **Bug:** `addToBuffer` is called by every `logger.Info/Debugf/Warningf/Error*` and the global logger
  is used concurrently across the whole app (cron jobs, the websocket `Hub.Run` goroutine, HTTP
  handlers, the xray log writer). Concurrent `append`/reslice on `logBuffer` (and concurrent reads in
  `GetLogs`) is a classic Go data race → undefined behavior (lost/corrupted entries, torn reads).
- **How it surfaced:** `go test -race -shuffle=on` — the `internal/web/websocket` Hub tests each run
  `go h.Run()`, and `Hub.runOnce`/`shutdown` log from that goroutine; two Hubs logging concurrently
  trip the detector on `logBuffer`. All 5 websocket failures share this single root cause; the rest of
  the repo is race-clean and order-independent.
- **The tests are correct** — they exercise real concurrent Hub behaviour and the detector flags the
  genuine prod defect. They are **not** weakened or skipped (cardinal rule 2).
- **Suggested fix (separate PR, NOT this audit):** guard `logBuffer` with a `sync.Mutex` (lock in
  `addToBuffer` and `GetLogs`). Small and safe.
- **Impact on deliverables:** the full `go test -race ./...` gate (Phase A) and the "-race in CI" gate
  (Phase F) **cannot be green until this prod fix lands.** Phase F therefore stages the `-race` CI gate
  as blocked-on-Finding-#2 rather than weakening it.
- **Status:** confirmed; awaiting a prod fix decision from the maintainer.

---

## 2a. Phase A hygiene result

- `go test -shuffle=on -count=1 ./...` — **green** across two independent seeds → no order/state
  dependence anywhere in the audited tree.
- `go test -race -shuffle=on -count=1 ./...` — **one failing package**, `internal/web/websocket`, all
  failures = the single Finding #2 race. No other races. No `-race`-only flakes elsewhere.
- Static smell scan complete → inventory in §1 (S1-S7). No assertion-free, tautological, or
  mock-asserting tests found — the suite is disciplined on those axes; the real weaknesses are
  over-broad/happy-path assertions (strengthened in Phases B-C) and the two findings above.

---

## 2b. Phase B coverage gaps closed (error/edge paths)

- **Finding #1 (migration tag cleanup):** added `TestMigrationRequirements_CleansLegacyZeroAddrTag`
  (internal/web/service/inbound_migration_test.go). Asserts a `inbound-0.0.0.0:30002` tag becomes
  `inbound-30002`. Witnessed RED (tag unchanged) → confirms the `tx.Raw` no-op → landed `t.Skip`.
- **TLS proxy+pin branch (S7):** added `TestHTTPClientForNode_ProxyPinPreservesPinEnforcement` (asserts
  the pin `VerifyConnection` is installed on the proxy transport) + `…_ProxyVerifyNoPin`. Manual
  mutation-sanity: dropping `transport.TLSClientConfig = tlsCfg` → test RED → reverted.
- **Pin-error specificity (S2):** `TestHTTPClientForNodePinInvalid` now table-driven, asserting the
  exact error for garbage vs empty pin (covers the `DecodeCertPin` empty-string branch).

---

## 3. Equivalent-Mutant Ignore-List

Mutants that are semantically identical to the original (unkillable) — documented, not chased.

| Package | File:Line | Mutation | Why equivalent |
|---|---|---|---|
| _(none yet — populated during Phase C)_ | | | |

---

## 4. Mutation Scores (before → after)

Run scoped per package (see §5). `LIVED` = a fake/weak test or a gap.

| Package | Before (killed %) | After (killed %) | Floor | Notes |
|---|---|---|---|---|
| internal/sub | TBD | TBD | 85% | highest priority |
| internal/web/runtime | TBD | TBD | 85% | security |
| internal/database (+ migration) | TBD | TBD | 75% | |
| internal/web/service (node) | TBD | TBD | 75% | scoped, excludes large/scale files |

---

## 5. How to run (mutation testing policy)

Mutation testing is **scoped + manual/nightly**, never a blocking per-commit CI gate (too slow).

```
# per package, scoped to keep runs tractable
gremlins unleash ./internal/sub/
gremlins unleash ./internal/web/runtime/
gremlins unleash -E 'dump_sqlite\.go' ./internal/database/
gremlins unleash -E 'server\.go|xray\.go|inbound\.go|client_bulk\.go|inbound_traffic\.go|.*_postgres_test\.go' ./internal/web/service/
```

Cheap gates that DO run in CI (see Phase F): `go test -race -shuffle=on -count=1 ./...` + a brief
fuzz smoke (`-fuzztime=30s`) on the critical parsers.
