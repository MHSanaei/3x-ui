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
| S1 | Over-broad (count-only) | internal/sub/service_dedup_test.go:59-64 | asserts only `len(links)==1` / `len(emails)==1`; dedup key (`strings.ToLower(client.Email)`) is unguarded | **fixed** (Phase C: added link-identity assert + `TestMatchingClients_DedupsCaseInsensitiveEmail`; mutation-sanity RED on `ToLower` drop) |
| S2 | Over-broad (`err!=nil` only) | internal/web/runtime/tls_client_test.go:118-122 | `TestHTTPClientForNodePinInvalid` asserts only that *an* error occurred; doesn't pin the error or cover empty-pin | **fixed** (Phase B: table-driven, asserts specific error + empty-pin case) |
| S3 | Over-broad (key-absence only) | internal/sub/clash_service_test.go | `TestBuildProxy_VLESSNoneEncryptionOmittedForClash` checks only `proxy["encryption"]` absence, not the rest of the proxy | **fixed** (Phase C: now also asserts type/server/port/uuid well-formedness) |
| S4 | Happy-path-only (substring) | internal/sub/service_flow_test.go | `TestGenVlessLink_*` only `strings.Contains(link,"flow=…")`; no full-link / field-mapping assertion exists | **fixed** (Phase C: added `service_sharelink_test.go` with full TLS + Reality field-mapping assertions; mutation-sanity RED on pbk/sid swap) |
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

## 2c. Phase C mutation audit

**Tooling note (gremlins on this host):** gremlins v0.6.0 is installed and runs, but is
**impractically slow on this Windows machine** — a `--dry-run` on the small `internal/web/runtime`
package produced no mutant list after >8 min, and a real `unleash` likewise buffers without
streaming. This matches the documented "slow on large modules" caveat. Per the plan's *if-blocked*
clause, the per-mutant loop is therefore driven by **manual mutation-sanity** (flip the guarded prod
line → confirm the test goes RED → revert) for the high-value targets, and gremlins is retained as a
scoped/nightly job (Phase F) to be measured on a faster Linux CI host. No package skipped silently.

**Strengthened (each verified by a witnessed RED under a targeted mutation, then reverted):**
- **Dedup key** (`internal/sub` `matchingClients`, service.go:130) — `TestMatchingClients_DedupsCaseInsensitiveEmail` + link-identity assert in the existing dedup test. RED when `strings.ToLower` dropped.
- **Share-link TLS mapping** (`applyShareTLSParams`, service.go:1029) — `TestGenVlessLink_TLSParamsMapped` asserts security/sni/fp/alpn/pcs.
- **Share-link Reality mapping** (`applyShareRealityParams`, service.go:1147) — `TestGenVlessLink_RealityParamsMapped` asserts security/sni/pbk/sid/fp/spx. RED when pbk/sid swapped.
- **Clash proxy well-formedness** (S3) — type/server/port/uuid now asserted.
- (Phase B already mutation-sanity'd the **TLS proxy+pin** injection.)

---

## 2d. Phase D property + fuzz reinforcement

**Property tests (`pgregory.net/rapid`):**
- `internal/sub/service_property_test.go` — `joinHostPort` bracketing (SplitHostPort round-trips
  host+port; IPv6 bracketed exactly once); `encodeUserinfo` round-trips through `net/url` for any
  password; `splitLinkLines` never emits empty/untrimmed lines and is a re-split fixed point.
- `internal/web/runtime/tls_client_property_test.go` — `DecodeCertPin` is format-agnostic over any
  32-byte pin (hex lower/upper, openssl colon-hex, base64 std/raw/url all decode equal).

**Fuzz targets (native `go test -fuzz`; seed corpora committed):**
- `FuzzParseLink` (internal/util/link) — the share-link parser. **Survived 25s / 6.2M execs, no
  panic**; the (result,error) contract holds. No finding (the parser's type assertions are guarded).
- `FuzzDecodeCertPin` (internal/web/runtime) — never panics; never returns a non-32-byte slice with a
  nil error, nor bytes alongside an error.

Both fuzz functions also run their seed corpus under plain `go test` (so CI exercises them green);
the time-boxed `-fuzztime` exploration is wired as a Phase F smoke step.

---

## 2e. Phase E frontend audit (lighter)

- **Coverage tooling wired:** `@vitest/coverage-v8` installed; `npx vitest run --coverage` works
  (overall ~53% stmts, dominated by untested UI components — not the audit's pure-logic target).
- **Assertion specificity:** frontend rejection tests use `safeParse().success`, not the
  "throws-anything" pattern (only one `.not.toThrow()`, which is legitimate). Most negative cases
  already pin a specific path. The clearest over-broad one — `InboundDbFieldsSchema` `subSortIndex`
  rejection (inbound-form-adapter.test.ts) — now asserts the error path is `subSortIndex` and adds a
  positive case (so a schema that rejects everything no longer passes).
- **StrykerJS:** skipped — the pure TS surface is trivial and the schema source is generated from Go
  (audited on the Go side); per plan this is optional.
- **Gate:** `npm run test` (534 passed) + `typecheck` + `lint` all green.

---

## 3. Equivalent-Mutant Ignore-List

Mutants that are semantically identical to the original (unkillable) — documented, not chased.

| Package | File:Line | Mutation | Why equivalent |
|---|---|---|---|
| _(none yet — populated during Phase C)_ | | | |

---

## 4. Mutation Scores (before → after)

Run scoped per package (see §5). `LIVED` = a fake/weak test or a gap.

**Measurement status:** gremlins did not complete a run on this Windows dev host (see §2c — a
dry-run on the smallest package exceeded 8 min with no output; a concurrent real run also corrupted
the build cache, crashing the Go linker). Numeric before/after scores are therefore **pending a run
on a faster Linux host** (the scoped commands in §5 are ready). In the interim, the load-bearing
branches the floors are meant to protect were each **verified by targeted manual mutation-sanity**
(flip the prod line → witness RED → revert), which is the per-mutant loop's actual purpose:

| Package | Floor | Load-bearing branches now guarded (mutation-sanity'd) |
|---|---|---|
| internal/sub | 85% | dedup key (`ToLower`), TLS param map, Reality param map (pbk/sid swap), clash well-formedness |
| internal/web/runtime | 85% | TLS proxy+pin injection, pin-error specificity, DecodeCertPin (property+fuzz) |
| internal/database (+ migration) | 75% | migration tag cleanup → **Finding #1** (witnessed RED) |
| internal/web/service (node) | 75% | covered by existing strong reconcile/dirty/origin tests (see §2 deep-dive) |

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

---

## 6. Phase F — gates locked into CI (.github/workflows/ci.yml)

- **`go-test`** (blocking) now runs `go test -shuffle=on -count=1` — order-dependence is a hard gate.
- **`race`** (new, **non-blocking** via `continue-on-error: true`) runs `go test -race -shuffle=on
  -count=1`. It is non-blocking *only* because of **Finding #2**; it surfaces the logger race in CI
  loudly. **Action:** once Finding #2 is fixed in prod, remove `continue-on-error` to make race a
  hard gate.
- **`fuzz-smoke`** (new, blocking) runs `FuzzParseLink` + `FuzzDecodeCertPin` for 30s each.
- **Mutation testing stays manual/nightly**, never per-commit (too slow) — run the scoped commands in
  §5 on a Linux host and record before/after in §4.

---

## 7. Status summary (definition of done)

- ✅ Order/shuffle hygiene: green across seeds; wired blocking into CI.
- ⚠️ `-race`: whole-repo clean **except Finding #2** (logger global) — wired non-blocking, awaiting prod fix.
- ✅ Over-broad/happy-path tests in scope strengthened (S1–S4, S7) and mutation-sanity'd; S6→Finding #1.
- ✅ Error/edge coverage gaps closed where a real contract existed (TLS proxy+pin, pin errors, migration tag).
- ✅ Property + fuzz added for link/userinfo/pin/parser; seed corpora inline; fuzz smoke in CI.
- ✅ Frontend: rejection-assertion specificity tightened; coverage tooling wired; gate green.
- ⏳ gremlins numeric scores: pending a run on a faster Linux host (commands ready in §5).
- ✅ **No test weakened. No production code changed to satisfy a test.** Two prod bugs surfaced as
  findings (#1 migration tag no-op, #2 logger data race), neither silently fixed.

### Open items for the maintainer (separate PRs, out of audit scope)
1. **Finding #1** — `internal/web/service/inbound_migration.go:247`: `tx.Raw(tagCleanup)` →
   `tx.Exec(tagCleanup)`; then remove the `t.Skip` from `TestMigrationRequirements_CleansLegacyZeroAddrTag`.
2. **Finding #2** — `internal/logger/logger.go`: guard `logBuffer` with a mutex in `addToBuffer` +
   `GetLogs`; then drop `continue-on-error` from the CI `race` job.
