# Review instructions

3x-ui is a Go (Gin + GORM) web panel that generates configuration, share links
and subscriptions for other programs — Xray-core, mihomo, sing-box, mtg-multi —
and is deployed by operators who upgrade in place. Judge findings by what
breaks for those consumers and operators, not by style.

## What a blocking finding means here

Reserve blocking severity for:

- Security on the exposed surfaces: `internal/web/controller/`, session and
  middleware code, the PUBLIC `internal/sub/` subscription server, and Xray
  config generation in `internal/xray/`.
- A state-changing inbound or client operation that bypasses `runtime.Runtime`
  (`internal/web/runtime/`) and calls `internal/xray/api.go` directly, or
  dispatches from a controller or cron job. It passes every local test and
  silently breaks every multi-node deployment.
- A schema or model change without a matching hand-written migration in
  `internal/database/db.go`, one that behaves differently on SQLite and
  PostgreSQL, or one that loses or overwrites operator data on upgrade or
  rollback. There are no migration files and no down-migrations.
- A change to what the panel emits on the wire — Xray config JSON, share
  links, subscription/Clash YAML, mtg-multi TOML — that a downstream client
  would reject or read differently, or that makes the three independent link
  implementations (Go `internal/util/link/` + `internal/sub/`, TS
  `frontend/src/lib/xray/`, TS `docs/lib/xray/`) diverge from one another.
- Any edit to `.github/workflows/`: this repository runs workflows with
  secrets against a public fork stream. Untrusted expression interpolation
  into `run:` blocks, broadened permissions, weakened guards, or a job that
  executes pull-request code is blocking.

Style, naming and refactoring suggestions are nits at most.

## Always check

- A new `g.POST`/`g.GET` in `internal/web/controller/` needs the whole chain:
  an entry in `frontend/src/pages/api-docs/endpoints.ts`, regenerated
  artefacts (`make gen`), any new API-boundary struct added to `StructAllow`
  in `tools/openapigen/main.go`, and `frontend/public/openapi.json` copied to
  `docs/public/openapi.json` with the docs MDX regenerated
  (`cd docs && pnpm gen:api`). CI checks the first three; the docs copy is
  checked by nothing — a missed copy is blocking, not a nit.
- A new i18n key exists in ALL 13 locale files in `internal/web/translation/`
  and is referenced from `frontend/src` or Go in the same PR.
- A bug fix carries a test that would fail without the fix. A test that
  passes either way, asserts only `err != nil` or `len(x) > 0`, or was made
  green by regenerating golden fixtures or Vitest snapshots is a real finding.

## Do not report

- Anything CI already enforces: golangci-lint and gofumpt, oxlint, format
  and typecheck, `npm audit`, govulncheck.
- The contents of generated files (`frontend/src/generated/`,
  `frontend/public/openapi.json`, `docs/public/openapi.json`) or lock files.
  Those files being STALE after a source change is reportable; their style
  is not.
- Missing tests for getters, constants, renames or pure map lookups —
  `CLAUDE.md` rejects such tests outright.

## Verification bar

- A claim about behaviour needs a `file:line` citation from this repository,
  not an inference from a name.
- A claim that a downstream client rejects or requires a wire-format detail —
  a config key, JSON tag, URI query parameter, YAML or TOML key, an encoding
  or hash choice — must name the upstream symbol that decides it (repository,
  file, identifier). If you cannot verify it, keep the finding but say
  explicitly that it is unverified instead of asserting it.

## Cap the nits

Report at most five nits per review and say "plus N similar" in the summary
for the rest. Lead the summary with "No blocking issues" when everything found
is a nit. After the first review of a PR, report blocking findings only.

## What the comment must show

The posted comment is the only part of a review anyone sees, so a bare "no
issues found" is a receipt, not a review: nothing in it says whether the diff
was read or the run died early. Every comment therefore ends with a short
coverage list — one line per area actually checked, naming what was examined
and what it turned out to be, plus the head SHA and the size of the diff it
covers. Say which claims could not be verified and why, including a check
this environment blocked. Keep it under ten lines; it is evidence, not a
retelling of the pull request.
