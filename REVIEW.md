# Review instructions

3x-ui is a Go (Gin + GORM) web panel that generates configuration, share links
and subscriptions for other programs — Xray-core, mihomo, sing-box, mtg-multi —
and is deployed by operators who upgrade in place. Judge findings by what
breaks for those consumers and operators, not by style.

## Severity

Mark every finding with exactly one of these, at the start of the finding:

| Marker | Severity | Use it for |
| --- | --- | --- |
| 🔴 | Important | A defect this pull request introduces or makes worse, in one of the classes under "What Important means here". Worth fixing before it merges. |
| 🟡 | Nit | Style, naming, refactoring, and an ordinary `CLAUDE.md` violation the change introduces — a source comment block over two lines, a fix larger than the bug it removes, a test `CLAUDE.md` rejects outright. |
| 🟣 | Pre-existing | A real bug you hit while reading that this pull request neither introduced nor made worse. |

Not every `CLAUDE.md` rule is a nit. The three listed below — the dispatch
rule, the migration rule, the endpoint chain — are Important, because each one
passes every local test and breaks a real deployment.

Severity follows what this pull request did, not how alarming the defect looks
on its own. One the change worsens is 🔴 for the regression it added, not for
the whole defect; one it merely brought into view is 🟣.

Checking what this panel emits means reading far more code than the diff
changes, so pre-existing bugs surface on every review. One already on the base
branch stays 🟣 however bad it is: this pull request did not cause it, so it
cannot be a reason to hold this pull request. Say in one clause that it
predates the change. The exception is a live security hole on an exposed
surface — still 🟣, but open the summary with it.

## What Important means here

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
  executes pull-request code.

## Always check

- A new `g.POST`/`g.GET` in `internal/web/controller/` needs the whole chain:
  an entry in `frontend/src/pages/api-docs/endpoints.ts`, regenerated
  artefacts (`make gen`), any new API-boundary struct added to `StructAllow`
  in `tools/openapigen/main.go`, and `frontend/public/openapi.json` copied to
  `docs/public/openapi.json` with the docs MDX regenerated
  (`cd docs && pnpm gen:api`). CI checks the first three; the docs copy is
  checked by nothing — a missed copy is Important, not a nit.
- A bug fix carries a test that would fail without the fix. A test that cannot
  tell the broken behaviour from the fixed one passes before and after, so it
  certifies nothing and is itself the finding — asserting only `err != nil` or
  `len(x) > 0`, or going green by regenerating golden fixtures or Vitest
  snapshots.
- No second way to do a thing already decided: Go tests are stdlib `testing`
  (never testify), the panel is Ant Design (never Tailwind or shadcn). Neither
  golangci-lint nor oxlint forbids the import, so it passes CI clean.

## Do not report

- Anything CI already enforces: golangci-lint and gofumpt, oxlint, format
  and typecheck, govulncheck, and `npm audit --omit=dev --audit-level=high`.
  A dev-dependency advisory is out of scope on purpose: it ships to nobody.
- The contents of generated files (`frontend/src/generated/`,
  `frontend/public/openapi.json`, `docs/public/openapi.json`) or lock files.
  Those files being STALE after a source change is reportable; their style
  is not.
- Missing tests for getters, constants, renames or pure map lookups —
  `CLAUDE.md` rejects such tests outright.
- A missing or unreferenced i18n key.
  `frontend/src/test/i18n-dead-keys.test.ts` pins the 13 locale files in
  `internal/web/translation/` in both directions, so the `frontend` job is
  already red. Report the failing check, not the key.

## A higher bar, not silence

Everything named under "What Important means here" gets full scrutiny. Two
areas do not — they earn review, but report there only what you are
near-certain about and that actually breaks something:

- `docs/` — the standalone Fumadocs site, with its own CI and its own
  dependency tree. `docs/lib/xray/` is the exception and gets full scrutiny:
  it is the third link implementation.
- `internal/web/translation/` — the key set is CI's job and the wording of a
  translation is nobody's here.

## Verification bar

- A claim about behaviour needs a `file:line` citation from this repository,
  not an inference from a name.
- A claim about what the change does to a caller or a callee needs that file
  read, not inferred from the hunk. A dispatch-rule violation rarely shows
  inside the diff — the changed line calls an innocuous helper and the
  `internal/xray/api.go` call sits a frame outside it.
- A claim that a downstream client rejects or requires a wire-format detail —
  a config key, JSON tag, URI query parameter, YAML or TOML key, an encoding
  or hash choice — must name the upstream symbol that decides it (repository,
  file, identifier). If you cannot verify it, keep the finding but say
  explicitly that it is unverified instead of asserting it.
- "CI passed" is a claim too, and needs the same evidence: say it only of a
  run you actually read. A green one proves less here than it looks — only
  `postgres-durable-first` runs against PostgreSQL, `go-test` and `race` are
  SQLite, and `XRAY_E2E_BINARY` and `XUI_SCALE_TEST` are set by no job, so
  those tests have never run in CI at all. Where a change touches dialect,
  migration or Xray gRPC code that no job exercised, say it is unverified
  rather than repeating a green tick as proof.

## Cap the volume

🔴 findings are never capped. Report every one.

Report at most five 🟡 nits and at most three 🟣 pre-existing bugs. Past that,
say "plus N similar" in the summary instead of posting them.

A cap decides WHICH ones survive, so choose rather than truncate: the same nit
repeated across files is ONE finding with a count, not five slots; a nit in
code this pull request wrote outranks one in code it only moved; and a nit
nobody would act on does not deserve a slot at all.

After the first review of a pull request, report 🔴 findings only: a one-line
fix must not reach round seven on style.

## What the comment must show

Open with a one-line tally — `2 🔴 / 4 🟡 / 1 🟣` — so the author sees the
shape of the review before the detail. When nothing is 🔴, lead with
`No blocking issues` and put the tally after it.

Nothing pads the comment: no "Strengths" section, no restatement of what the
pull request does, no praise, no closing pleasantry. Padding is not neutral —
it buries the two lines someone actually has to act on.

The posted comment is the only part of a review anyone sees, so a bare "no
issues found" is a receipt, not a review: nothing in it says whether the diff
was read or the run died early. Every comment therefore ends with a short
coverage list — one line per area actually checked, naming what was examined
and what it turned out to be, plus the head SHA and the size of the diff it
covers. Say which claims could not be verified and why, including a check
this environment blocked. Keep that coverage list under ten lines; it is
evidence, not a retelling of the pull request.

## A finding is a report, not a patch

A finding says what is wrong, where (`file:line`), what triggers it and what
breaks. It never carries the fix: no `suggestion` block, no patch, no
replacement snippet, no rewritten function, no "suggested fix" section — in
the summary and in an inline comment alike. One clause naming WHERE the fix
belongs is the most it may add — a file, a function, a symbol, a layer — and
nothing about what happens there. Prose is a patch too the moment a verb
describes the change: "move the lookup inside the body", "spend the comment
on the invariant instead" hand it over as surely as a diff would, and so does
holding up an existing symbol as the model to copy. A clause the maintainer
could apply as written is the fix, however it is punctuated. The maintainer
decides the change; a review that writes it out puts unreviewed code one
click from the branch.

A 🔴 or 🟡 finding also says, in one clause, what this pull request did to
the code it is about — the line it added, the call it moved, the guard it
dropped — the way a 🟣 says that it predates the change. That clause reports
what the change did, never what it should have done. Nothing else in the
comment shows the marker was earned.
