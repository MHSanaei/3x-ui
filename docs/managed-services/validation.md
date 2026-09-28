# Verification record

No new protocol or whole-system policy feature has passed acceptance yet.
This record distinguishes source review, arithmetic tests, integration and
real-client data-plane evidence. Skips are not passes.

## Environment

- Linux arm64, kernel 6.17.0-1018-oracle, UTC, isolated development checkout.
- Official Go 1.27.1 linux/arm64 SHA-256:
  `3450b45a3f9ee8568792736a5c5e70a1f2e9b36c35a8f74958c03e51d7d92bec`.
- Official Node v26.10.0 linux/arm64 SHA-256:
  `7a6353f63eb3d04765004b4adf172616243e4522434635cb1d26288658b04ab5`.
- `nft`, `tc`, `ip` are present; availability is not a privileged capability test.
- Docker and Go were absent initially. Go and Node installed under
  `/root/toolchains`, without changing host services.
- Surge Mac/iOS client and license are not present. Snell real interoperability
  remains unexecuted even after any future server/config test.

## Commands and observed evidence

| Command / check | Result |
|---|---|
| `git status --short --branch` on initial clone | clean main, no user changes |
| GitHub fork API / release API / `git ls-remote` | provenance and SHAs in audit.md |
| `git rev-list --count v3.8.5..HEAD` at baseline | 64 |
| Safe local credential classification | SSH private key, mode 0600; content never printed |
| SSH strict host check (first attempt) | failed: no known ED25519 host key |
| SSH with keys pinned from official HTTPS `/meta` | authenticated as fork owner; exit 1 is GitHub's expected no-shell response |
| `go mod download` | exit 0 |
| `make test-go` | restricted socket run failed; authorized rerun exit 0, 46 packages passed |
| `npm ci` | exit 0, 621 packages installed, npm reported 0 vulnerabilities |

Network access from the restricted shell failed DNS resolution initially;
authorized network tool runs succeeded. No host-key-check bypass was used.
No kernel networking changes, deployment, public release or default-branch
merge were performed.

## Acceptance not yet executed

All real-protocol bandwidth, quota, routing, authentication, recovery and
multi-node tests from requirements sections A–D remain open. Full static,
race, frontend, PostgreSQL and packaging checks also remain open until their
actual results are recorded. The implementation must establish pre-test burst,
sampling, cutoff and overshoot bounds; none is claimed for unbuilt adapters.

## Exact arithmetic milestone (not runtime billing integration)

- `go test ./internal/clientpolicy` first failed on missing arithmetic symbols.
- `go test -race -count=1 ./internal/clientpolicy`: passed, 1.244s.
- `go test -run '^$' -fuzz FuzzChargeMatchesArbitraryPrecision -fuzztime=10s ./internal/clientpolicy`:
  passed, 258,440 executions; checked against independent arbitrary-precision arithmetic.
- `go vet ./internal/clientpolicy`: executed before the arithmetic commit.
- Coverage includes exact multipliers 0.5/1/1.5/2/10, fractional carry across
  1,000,001 single-byte events, different batch sizes, segmented 10 GiB at 1×
  then 5 GiB at 2× = 20 GiB, quota allowance and checked int64 limits.
- No database persistence, data path, UI or backend feature is claimed from
  these arithmetic tests. Identity/migration and runtime integration follow.
- Frontend baseline `npm run typecheck`: exit 0. Full `npm test` still running.
- Git hook initially used host Node18 and failed on util.styleText; rerunning
  the commit with the repository-required task-local Node26 succeeded.
- First push of 4e2ff8c6 was rejected by automatic approval review because it
  did not recognize authorization for external data transfer/remote branch
  creation. No push occurred. Explicit user approval requested; local work
  continues. No authentication secret was copied into the repository.
