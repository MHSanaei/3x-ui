# Unified client policy and backend implementation plan

> For agentic workers: use superpowers:executing-plans inline. Keep this plan
> and validation evidence current; use a fresh whole-branch reviewer at the end.

**Goal:** Deliver every item in requirements.zh-CN.md with runnable integration,
real data-path acceptance evidence and verified feature-branch pushes.

**Architecture:** Existing ClientRecord/Runtime management with immutable policy
identity, exact accounting and shared data-path shaping. Authenticated backend
adapters preserve destination and identity into existing Xray routing.

**Tech stack:** Go 1.27.1, Gin/GORM, SQLite/PostgreSQL, React/Ant Design/Node 26,
pinned Xray and protocol backends.

**Spec:** [design.md](design.md), [semantics.md](semantics.md), and the complete
[user requirements](requirements.zh-CN.md) take precedence over this plan.

## Global constraints

- Preserve baseline 17d7dd46b512d0a9c22921a6094f30c672e436c9 and all existing features.
- Work on feat/unified-client-policy-backends; conventional logical commits;
  push only to the supplied fork; verify each remote SHA; no forced updates.
- Follow repository layering, code generation, i18n and two-line comment rules.
- Per-client aggregate rates use raw bytes/s, 0 unlimited; live updates ≤2s.
- Multiplier default 1×, positive [0.001,1000], precision 0.001; integer units.
- Missing required capability fails protected; no direct/unlimited fallback.
- Real clients and privileged tests cannot be replaced by mocks or skips.
- Never modify host management networking or deploy/reboot production.

## Review focus

1. Same email recreated after deletion must not inherit old source reports.
2. Late pre-change counters must not use a new multiplier; test policy epochs.
3. Disable/expiry/quota races with renewal must not re-enable another reason.
4. Hidden fast paths (Vision splice, UDP, backend egress) must not bypass policy.
5. Snapshot hierarchy/restores must not bill a source twice or issue extra credit.

## Task 1: Audit and reproducible baseline

Files: this directory; read existing models, Runtime, services, jobs, build/CI.

- [x] Preserve full requirement file; clone, inspect clean status and instructions.
- [x] Verify upstream release and exact newer fork baseline without downgrade.
- [x] Source-derived capability matrix and architecture alternatives/data paths.
- [x] Identify credential locally and verify SSH with pinned official host keys.
- [ ] Complete baseline Go/frontend/build checks and record failures/skips.
- [ ] Probe official backend binaries, licenses and isolated network capabilities.
- [ ] Commit audit and push branch; verify remote SHA.

## Task 2: Exact accounting arithmetic

Files: new `internal/clientpolicy/accounting.go` and `accounting_test.go`.
Produces `ParseMultiplier(string) (Multiplier,error)`, `Multiplier.String()`,
`Charge(raw int64, multiplier Multiplier, remainder int64) (whole,carry int64,err error)`,
`RawAllowance(remaining, remainder int64, multiplier Multiplier) (int64,error)`.
Consumes the fixed-point contract in semantics.md; no DB or clock dependency.

- [ ] Write failing tests for 0.5/1/1.5/2/10, three-decimal precision, invalid
  zero/negative/NaN/Infinity/exponents, and exact round-trip decimal output.
- [ ] Test one-byte batches versus unsplit 1,000,001-byte usage, carried
  fractions across 1×→2×, and 10 GiB + 5 GiB segment example (20 GiB billed).
- [ ] Test int64 extremes and intermediate product overflow; rejected input
  must return an exact typed sentinel and no usable partial charge.
- [ ] Test quota allowance for fractional remaining bytes and all multipliers.
- [ ] Run `go test ./internal/clientpolicy` and observe missing behavior RED.
- [ ] Implement checked quotient/remainder operations without floating point.
- [ ] Run package test, race and fuzz/property boundary checks; commit and push.

## Task 3: Durable accounting and identity

Files: `internal/database/model/client_policy.go`, `database/db.go`,
`database/migrate_data.go`, `web/service/client_policy.go` and their tests;
extend model/model.go conversions, portable exports and API generator.
Consumes Task 2; produces stable UUID policy identity and atomic ledger API.

- [ ] Write SQLite/Postgres migration tests retaining old raw usage at 1×.
- [ ] Implement transactional raw/billed/remainder/cursor/revision updates.
- [ ] Red→green replay/out-of-order/concurrent/reset/restart/deletion/restore tests.
- [ ] Integrate existing local and remote traffic paths exactly once per source.
- [ ] Implement multiplier boundary settlement before accepting a policy change.
- [ ] Carry new fields through all CRUD/bulk/import/export/backup/node paths.
- [ ] Verify DB backends and recovery; commit/push with evidence.

## Task 4: Shared live rate and quota data path

Files: `internal/clientpolicy` engine/stream/datagram files and E2E tests;
core dispatcher adapter plus Runtime apply/reconcile path.
Consumes immutable policy/ledger; produces cancellable per-client flow budget.

- [ ] Test two rates + unlimited, >1 connection, same-IP separate clients,
  bidirectional transfer, live changes, cancellation and bounded buffering.
- [ ] Implement shared token scheduling, durable quota reservation and reasons.
- [ ] Integrate actual Xray authenticated dispatcher; test Vision/splice/mux/UDP.
- [ ] Reconcile observed capability/revision and protect failed-policy services.
- [ ] Allocate global rate shares/byte credits with bounded node leases.
- [ ] Prove existing-flow cutoff and restart persistence; commit/push.

## Task 5: SSH vertical integration

Files: new `internal/sshtunnel`, existing protocol model/Runtime/service,
frontend protocol schemas/forms, translations, sub/export, deployment.

- [ ] Real OpenSSH -L/-D/-R tests first; independent keys and channels.
- [ ] Dedicated server; deny shell/exec/PTY/SFTP/agent and default -R.
- [ ] Target/listener restrictions; strict SSH upstream host key verification.
- [ ] Unified routed outbound bridge, counters/policy/live cutoff/health.
- [ ] Complete UI/API/backup/node/export/install; test faults; commit/push.

## Task 6: mieru vertical integration

Files: new `internal/mieru`, protocol registry/model/Runtime and existing UI.

- [ ] Pin v3.38.0 API/config and official client/server; test TCP and UDP.
- [ ] Native multiuser identity preserved into policy-aware dispatch/routing.
- [ ] Disable conflicting rolling native quotas; single panel billing owner.
- [ ] Complete CRUD/API/UI/export/lifecycle/logs/deployment/node/recovery paths.
- [ ] Run real-client route/rate/quota/auth/failure matrix; commit/push.

## Task 7: Snell v4/v5/v6 vertical integration

Files: new `internal/snell`, backend asset manifest/installer, namespace bridge,
protocol registry/model/Runtime/UI and export paths.

- [ ] Pin 4.1.1, 5.0.1, 6.0.0rc2 beta and matching real Surge combinations.
- [ ] Verify asset provenance/hash/architecture/license; no proprietary assets in Git.
- [ ] Per-client process/listener/egress isolation with actual target preservation.
- [ ] TCP, UDP-over-TCP, version-specific QUIC mode; no blanket template.
- [ ] Implement full client/policy/routing/lifecycle/node/deployment/export paths.
- [ ] Real Surge interoperability per target; record unavailable tests, never pass.
- [ ] Failure/cleanup/cost evidence; commit/push.

## Task 8: First-class port forwarding and remaining existing backends

Files: new `internal/portforward`, existing inlet selection/client flow,
port_conflict.go, Runtime, routing bridge; TUIC/MTProto/WG/AWG adapters.

- [ ] Evaluate nft/iptables family, permissions and offload in isolated namespace.
- [ ] Exclusive node+network+address+port client binding, TCP/UDP/combined.
- [ ] Multi-rule shared policy, source ACLs, IPv4/IPv6, DNS refresh behavior.
- [ ] Validate conntrack/return path/established revocation for any kernel mode.
- [ ] Replace TUIC aggregate-only meter with authenticated per-user path.
- [ ] Prove MTProto, WireGuard, AWG, HTTP/SOCKS, TUN/tunnel applicable paths.
- [ ] Full management/export/node/backup integration and real-client tests; push.

## Task 9: Cross-cutting delivery and full acceptance

- [ ] Audit every matrix row again for every protocol, including groups/LDAP,
  HWID, subscription host overrides, notifications and first-use expiry.
- [ ] Fork-safe install/update; reproducible images/packages; non-Linux build gates.
- [ ] Tested backup/restore, upgrade/rollback and only-owned-resource uninstall.
- [ ] `make verify`, `make race`, live PostgreSQL, parser fuzz and vulncheck.
- [ ] Full request A–E data-plane acceptance with independent observations,
  fixed tolerance, real versions/commands/durations; no skip counted as pass.
- [ ] Fresh whole-branch review; fix material findings with RED→GREEN tests.
- [ ] Requirement-by-requirement completion audit; final remote SHA verification.
- [ ] Final report with versions, complete/incomplete/inapplicable/unverified,
  commits, measured results, operations and remaining risks.
