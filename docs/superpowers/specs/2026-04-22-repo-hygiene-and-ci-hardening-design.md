---
title: Repo hygiene files and CI hardening
date: 2026-04-22
status: draft
owners: "@<MAINTAINER_HANDLE>"
supersedes: []
related:
  - CI_CD.md
  - CONTRIBUTING.md
  - .github/dependabot.yml
---

# Repo hygiene files and CI hardening

## Motivation

AgentForge already has most of the scaffolding a mature OSS repo expects —
README / CONTRIBUTING / TESTING / LICENSE (MIT © AstroAir 2024) / CHANGELOG /
mkdocs / 10 workflows / Dependabot with groupings. Independent verification of
workflow files at spec-authoring time showed the CI pipeline is also more
advanced than `CI_CD.md` reflects: `ci.yml` / `quality.yml` / `test.yml` /
`go-ci.yml` / `release.yml` all declare `concurrency:` groups today, `pnpm`
install caching is on in three frontend workflows, Go caching is on in every
Go setup, `Swatinem/rust-cache@v2` is wired into both Tauri workflows, and
`desktop-tauri-logic.yml` is a hard `needs:` dependency of `build-tauri` in
both `ci.yml` and `release.yml`.

That means the remaining gaps are narrower than they appeared:

1. **Community standards files are absent.** `SECURITY.md`,
   `CODE_OF_CONDUCT.md`, and `CODEOWNERS` do not exist. Private vulnerability
   disclosure has no clear channel, and PR review routing relies on the
   author remembering to add reviewers.
2. **Four workflows still lack concurrency.** `build-tauri.yml`,
   `desktop-tauri-logic.yml`, `agent-review.yml`, `review-layer2.yml` have no
   `concurrency:` block; `deploy.yml` also lacks one but is being archived in
   this spec anyway.
3. **`deploy.yml` is dead code.** Hard-coded `DEPLOY_ENABLED: false`; Vercel
   steps fully commented out; not called from `ci.yml`. It reads as live
   infra when it is not.
4. **Lint / audit / outdated output is not surfaced in the PR UI.**
   `quality.yml` runs all three with `continue-on-error: true` and the only
   way to learn the result is to open the raw job log. No SARIF upload, no
   job summary.
5. **The Bun install in `ci.yml`'s inline `bridge-typecheck` job has no
   cache.** Every PR re-installs `src-bridge/node_modules` cold. (The other
   inline job, `im-bridge-build`, already passes `cache: true` to
   `setup-go@v6`.)
6. **`CI_CD.md` is stale.** It still says `desktop-tauri-logic.yml` is "not
   currently wired into `ci.yml`" (line 219), which has been false for some
   time. It also doesn't mention the existing concurrency groups or caches.
   A spec that claims to harden CI must leave `CI_CD.md` as an accurate
   mirror of post-change reality.
7. **There's no one-liner local verification.** Contributors run
   `pnpm lint && pnpm test && pnpm build && ...` as muscle memory instead
   of a documented script.

Release automation (release-please / changesets), CodeQL, SBOM, notarization,
Renovate, and any time-bound vulnerability-response SLA are **explicitly out
of scope** — per MEMORY the project is in internal testing with breaking
changes freely permitted, and those investments should follow a move toward
external consumers, not precede it.

## Goals

- Raise GitHub community-profile health to ≥ 85%.
- Add `concurrency:` groups to the four workflows that lack them:
  `build-tauri.yml`, `desktop-tauri-logic.yml`, `agent-review.yml`,
  `review-layer2.yml`.
- Add a Bun install cache to `ci.yml`'s inline `bridge-typecheck` job.
- Archive `deploy.yml` out of `.github/workflows/` while preserving future
  reference.
- Surface lint / audit / outdated output as PR annotations (SARIF for lint)
  and job summaries (for audit / outdated), without flipping any of them to
  blocking yet.
- Plant a `CI_LINT_STRICT` env switch that a future spec can flip to true
  without further infrastructure changes.
- Give contributors `pnpm verify` and `pnpm verify:all` as documented
  one-liners.
- Update `CI_CD.md` so it reflects post-change reality (including fixing
  its stale claim about `desktop-tauri-logic`).

## Non-Goals

- **Flip lint from non-blocking to blocking.** Deferred to a follow-up spec.
  The switch is planted; the flip is not.
- **Release automation** (release-please, changesets, semantic-release).
- **Non-native security tooling** (CodeQL, Snyk, SBOM, cosign).
- **Replace Dependabot** with Renovate.
- **Platform code-signing** (macOS notarization, Windows cert flows beyond
  what `build-tauri.yml` already optionally supports).
- **Commit to a `security@` email or time-bound disclosure SLA.** `SECURITY.md`
  routes reporters through GitHub Security Advisory; no email contact is
  added and no response time is promised.
- **A public security issue template.** Security reports must flow through
  the private advisory channel.
- **`.github/FUNDING.yml`.**
- **Re-adding caches that already exist.** pnpm caches in `quality.yml` /
  `test.yml` / `build-tauri.yml`, Go caches in every `setup-go@v6` call,
  Next.js build cache in `test.yml`, and `rust-cache` in both Tauri
  workflows all stay untouched.
- **Changing `desktop-tauri-logic.yml`'s wiring.** It is already a hard
  `needs:` dependency of `build-tauri` in both `ci.yml` (line 96) and
  `release.yml` (line 57). Narrowing this to a path-filtered gate is an
  optimization, not a paper-tiger fix, and is deferred to a follow-up.
- **Reviewing `agent-review.yml` / `review-layer2.yml` permissions or
  secret handling.** They are the highest-risk workflows in the repo but
  their design is out of scope for this pass; this spec only adds
  concurrency groups to them.

## Current State (verified 2026-04-22 against working tree)

### Present and accurate

- **Community**: `LICENSE` (MIT © AstroAir 2024), `README.md`, `README_zh.md`,
  `CONTRIBUTING.md`, `CHANGELOG.md`, `AGENTS.md`, `CI_CD.md`, `TESTING.md`.
- **`.github/`**: `dependabot.yml` (npm + cargo + actions with groups), PR
  template, three issue templates (bug / feature / config), copilot
  instructions.
- **Workflows (10)**: `ci.yml`, `quality.yml`, `test.yml`, `go-ci.yml`,
  `build-tauri.yml`, `desktop-tauri-logic.yml`, `release.yml`, `deploy.yml`,
  `agent-review.yml`, `review-layer2.yml`.
- **Concurrency groups exist today** in `ci.yml:35-37`, `quality.yml:14-16`,
  `test.yml:17-19`, `go-ci.yml:18-23` (conditional form),
  `release.yml:25-27` (with `cancel-in-progress: false`).
- **pnpm install cache** is on in `quality.yml:56`, `test.yml:50`,
  `build-tauri.yml:80`.
- **Next.js build cache** is on in `test.yml:52-60`.
- **Go cache** is on in `go-ci.yml:42-43` (go-unit), `go-ci.yml:103-104`
  (go-integration), `build-tauri.yml:111-113`, `ci.yml:88-90`
  (im-bridge-build).
- **`Swatinem/rust-cache@v2`** is on in `build-tauri.yml:87-91` and
  `desktop-tauri-logic.yml:22-26`.
- **`desktop-tauri-logic.yml`** is called from `ci.yml:60-62` and
  `release.yml:50-52`; it is a `needs:` dep of `build-tauri` in both
  (`ci.yml:96`, `release.yml:57`).
- **Scripts convention**: every script directory under `scripts/` has
  `.test.ts` (or `.test.mjs`) siblings next to `.js` implementations —
  verified in `scripts/build/`, `scripts/dev/`, `scripts/i18n/`,
  `scripts/plugin/`, `scripts/release/`, `scripts/skills/`.

### Missing

- `SECURITY.md`
- `CODE_OF_CONDUCT.md`
- `.github/CODEOWNERS`
- Concurrency group in `build-tauri.yml`, `desktop-tauri-logic.yml`,
  `agent-review.yml`, `review-layer2.yml`.
- Bun install cache in `ci.yml`'s inline `bridge-typecheck` job (lines
  65–75): `bun install --frozen-lockfile` runs cold on every run.
- SARIF upload and job-summary reporting for `pnpm lint` / `pnpm audit` /
  `pnpm outdated`.
- `CI_LINT_STRICT` env switch in `quality.yml`.
- Aggregate local-verify scripts in `package.json`.

### Known "paper tiger" behaviors

- `quality.yml:63` — `pnpm lint` with `continue-on-error: true`.
- `quality.yml:70` — `pnpm audit --audit-level=moderate` with
  `continue-on-error: true`.
- `quality.yml:74` — `pnpm outdated` with `continue-on-error: true`.
- `deploy.yml:45` — `DEPLOY_ENABLED: false` hard-coded; Vercel steps
  commented out; not called from `ci.yml`.

### Known stale documentation

- `CI_CD.md:219` — "This workflow is callable and manually runnable. It is
  not currently wired into `.github/workflows/ci.yml`." False today
  (`ci.yml:60-62`).
- `CI_CD.md` general job-graph section — does not mention existing
  concurrency groups or caches.

## Design

### 1. Repository hygiene files

#### 1.1 `SECURITY.md` (repo root)

One-page, scoped to current project posture.

Sections:

- **Supported versions** — "AgentForge is pre-1.0 and under MIT License
  (see LICENSE). The `master` branch receives security fixes. No LTS
  branch is maintained during internal testing."
- **Reporting a vulnerability** — Route **exclusively** through
  GitHub → Security → Report a vulnerability (private advisory). Do not
  open public issues for security reports. No email address is listed;
  adding one would contradict the non-goal on SLAs.
- **Scope** — Go orchestrator (`src-go/`), TS bridge (`src-bridge/`), IM
  bridge (`src-im-bridge/`), Tauri desktop shell (`src-tauri/`), Next.js
  frontend (`app/`, `components/`, `lib/`), marketplace microservice
  (`src-marketplace/`).
- **Out of scope** — Third-party plugin vulnerabilities (report to plugin
  author), upstream library CVEs not yet patched (report upstream first),
  self-hosted deployment misconfiguration, social-engineering attacks on
  maintainers.
- **Response expectations** — "Best-effort triage; no time-bound SLA
  during internal testing."
- **Disclosure policy** — Coordinated disclosure preferred; reporters who
  follow the private channel are credited in the fix commit.

#### 1.2 `CODE_OF_CONDUCT.md` (repo root)

- Verbatim **Contributor Covenant v2.1** (stable, well-known text).
- Contact block: a single line using the **GitHub handle form** —
  placeholder `@<MAINTAINER_HANDLE>` filled in during implementation. No
  email address. This keeps §1.2 consistent with the "no `security@`"
  non-goal by using only in-platform channels.

#### 1.3 `.github/CODEOWNERS`

Minimal-but-useful seed. New maintainers slot into sub-paths later without
a schema overhaul.

```
# Default owner
*                           @<MAINTAINER_HANDLE>

# Infra & CI
/.github/                   @<MAINTAINER_HANDLE>
/scripts/                   @<MAINTAINER_HANDLE>

# Sub-stacks
/src-go/                    @<MAINTAINER_HANDLE>
/src-bridge/                @<MAINTAINER_HANDLE>
/src-im-bridge/             @<MAINTAINER_HANDLE>
/src-tauri/                 @<MAINTAINER_HANDLE>
/src-marketplace/           @<MAINTAINER_HANDLE>

# Docs
/docs/                      @<MAINTAINER_HANDLE>
```

Risk considered: Dependabot PRs auto-request the CODEOWNER, producing a
short-term review queue. Mitigations:

- Accept the queue — it is bounded by the existing `open-pull-requests-limit`
  in `dependabot.yml` (10 for npm, 5 each for cargo and actions).
- Not chosen: excluding `package.json` / lockfiles from CODEOWNERS. It
  would skip routing for security-relevant dep bumps, which is the exact
  review we want.

**Placeholder substitution is an implementation gate.** Every occurrence
of `<MAINTAINER_HANDLE>` / `<MAINTAINER_CONTACT>` in the merged PR diff
must be a real handle. The acceptance criteria include an explicit grep
check.

### 2. CI workflow hardening

#### 2.1 Concurrency groups on the four workflows that lack them

Add to `build-tauri.yml`, `desktop-tauri-logic.yml`, `agent-review.yml`,
`review-layer2.yml` (but not to `release.yml`, which already has one;
not to `deploy.yml`, which is being archived in §2.5; not to the five
workflows that already have concurrency):

```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: ${{ github.event_name == 'pull_request' }}
```

Rationale:

- For `build-tauri.yml`: It is `workflow_call`-only today — but when
  called from `ci.yml` on a PR, a concurrency group means rapid pushes
  to the same PR don't burn the 4-platform Tauri matrix multiple times
  in parallel. Because `build-tauri.yml` is invoked via `workflow_call`
  from both `ci.yml` and `release.yml`, the group key must use
  `github.workflow` (the **caller** workflow name after GitHub's
  concurrency rules) — this matches the approach `go-ci.yml` already
  takes with its `workflow_call` conditional form.
  **Caveat**: `workflow_call`-reusable workflows inherit the caller's
  concurrency context in some cases; an implementation note in the plan
  will cross-check behavior against `go-ci.yml`'s existing conditional
  form to avoid accidentally killing release builds.
- For `desktop-tauri-logic.yml`: Same `workflow_call` reasoning.
- For `agent-review.yml`: PR-triggered; rapid pushes should not queue
  multiple Claude reviews.
- For `review-layer2.yml`: Triggered by `workflow_run`; the group should
  key on the upstream run's head SHA so back-to-back layer-1 completions
  don't queue. Implementation detail for the plan.

`cancel-in-progress` is gated on `pull_request` so `master` pushes,
tag pushes, and the release path never lose an in-flight run.

**Reviewed risk**: `build-tauri.yml` is the most expensive matrix in the
repo. Cancellation of an older PR build is the point, so no mitigation is
needed there. But for `desktop-tauri-logic.yml` called from `release.yml`,
the group expression MUST NOT cancel the release-path invocation. The
plan will verify by inspecting how `go-ci.yml:22-23` handles this:
`group: ${{ github.event_name == 'workflow_call' && github.run_id || format('go-ci-{0}', github.ref) }}` keeps workflow_call runs in
unique groups so parallel callers never cancel each other. The four new
concurrency blocks will use the same conditional pattern.

#### 2.2 Bun install cache in `ci.yml`'s `bridge-typecheck` job

Current state (`ci.yml:65-75`): `oven-sh/setup-bun@v2` plus
`bun install --frozen-lockfile` — no cache, cold every run.

Change:

```yaml
bridge-typecheck:
  name: Bridge Typecheck
  runs-on: ubuntu-latest
  defaults:
    run:
      working-directory: src-bridge
  steps:
    - uses: actions/checkout@v6
    - uses: oven-sh/setup-bun@v2
    - name: Cache Bun install
      uses: actions/cache@v4
      with:
        path: |
          ~/.bun/install/cache
          src-bridge/node_modules
        key: ${{ runner.os }}-bun-${{ hashFiles('src-bridge/bun.lockb') }}
        restore-keys: |
          ${{ runner.os }}-bun-
    - run: bun install --frozen-lockfile
    - run: bun run typecheck
```

Risks considered:

- **Stale `node_modules` after Bun version upgrade.** Mitigation: include
  `bun --version` output in the cache key via a step that runs before
  the `actions/cache` step and sets an env var. If this complicates the
  plan, the `restore-keys:` tier is enough to force a miss on a lockfile
  change — a Bun upgrade normally touches `bun.lockb` through
  `package.json` pinning anyway.
- **Cache pollution with root `node_modules`.** Mitigation: path is
  explicitly `src-bridge/node_modules`, not `./node_modules`. Distinct
  cache key prefix (`bun-`) keeps it separate from the pnpm cache key.

Out of scope for this change: adding similar caching to `im-bridge-build`
(its `setup-go@v6` already has `cache: true`).

#### 2.3 Lint / audit / outdated visibility (non-blocking, with strict switch)

Rewrite `quality.yml` steps 61-74. Add the `security-events: write`
permission at the job level (required for `github/codeql-action/upload-sarif`).

```yaml
permissions:
  contents: read
  pull-requests: write
  checks: write
  security-events: write

# ... existing Setup pnpm / Setup Node.js / Install steps ...

- name: ESLint (annotated SARIF)
  id: lint-soft
  continue-on-error: true
  run: |
    pnpm lint \
      --format @microsoft/eslint-formatter-sarif \
      --output-file eslint.sarif || true

- name: Upload ESLint SARIF
  if: always()
  uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: eslint.sarif
    category: eslint

- name: ESLint summary
  if: always()
  run: node scripts/ci/summarize-lint.js eslint.sarif >> "$GITHUB_STEP_SUMMARY"

# CI_LINT_STRICT flip: a SEPARATE step without continue-on-error.
# When the future follow-up spec sets the repo variable to "true", the
# lint-strict step fails the job. Until then it is a no-op.
- name: ESLint (strict, gated by CI_LINT_STRICT)
  if: ${{ vars.CI_LINT_STRICT == 'true' }}
  run: pnpm lint

# TypeScript check is unchanged and remains blocking.
- name: TypeScript type checking
  run: pnpm exec tsc --noEmit

- name: pnpm audit (JSON)
  id: audit-soft
  continue-on-error: true
  run: pnpm audit --audit-level=moderate --json > audit.json || true

- name: Audit summary
  if: always()
  run: node scripts/ci/summarize-audit.js audit.json >> "$GITHUB_STEP_SUMMARY"

- name: pnpm outdated (JSON)
  id: outdated-soft
  continue-on-error: true
  run: pnpm outdated --format json > outdated.json || true

- name: Outdated summary
  if: always()
  run: node scripts/ci/summarize-outdated.js outdated.json >> "$GITHUB_STEP_SUMMARY"
```

Key design points:

- **Strict mode is a SEPARATE step without `continue-on-error`.** The
  previous draft had a correctness bug where the strict branch inherited
  the soft step's `continue-on-error: true`; the two-step design avoids
  that entirely. When `vars.CI_LINT_STRICT == 'true'`, the strict step
  runs `pnpm lint` directly, has no `continue-on-error`, and fails the
  job on violations.
- **Summary scripts always exit 0.** They are reporting, not gating. A
  summary-generation bug must not turn a green CI red. The `.test.ts`
  siblings enforce this explicitly with a malformed-input test.
- **`security-events: write` is granted at job scope** (not workflow
  scope) to minimize the blast radius — only the `quality` job can write
  security events.

New scripts (each requires a `.test.ts` sibling; convention mirrors
`scripts/build/*.test.ts`):

| Script | Reads | Emits to `$GITHUB_STEP_SUMMARY` |
|---|---|---|
| `scripts/ci/summarize-lint.js` | ESLint SARIF | total count, top-5 files, top-5 rules |
| `scripts/ci/summarize-audit.js` | `pnpm audit --json` | severity counts, top 5 advisories by severity |
| `scripts/ci/summarize-outdated.js` | `pnpm outdated --format json` | compact Markdown table of name / current / wanted / latest / type |

Each `.test.ts` must cover:

- Happy path: realistic input → expected Markdown fragment.
- Empty input: no violations / no outdated packages → neutral success message.
- Malformed input: invalid JSON or unexpected schema → fallback message and
  exit code 0 (explicit fail-safe test).

#### 2.4 `deploy.yml` archival

- `git mv .github/workflows/deploy.yml docs/ci-examples/deploy.example.yml`.
- Prepend a docblock to the moved file:

```yaml
# ⚠️ Reference example — NOT an active workflow.
# This file was archived from .github/workflows/deploy.yml on 2026-04-22.
# To reactivate: copy back to .github/workflows/deploy.yml, set
# DEPLOY_ENABLED to true, and configure the VERCEL_* secrets.
# See CI_CD.md for context.
```

- Remove all references to `deploy.yml` from `CI_CD.md`.
- Add a "Disabled examples" section in `CI_CD.md` pointing to
  `docs/ci-examples/deploy.example.yml`.
- Confirm no other file under `.github/`, `scripts/`, or `docs/` references
  `deploy.yml`. The plan will include a grep step.

### 3. Local developer experience

#### 3.1 Aggregate verify scripts

Add to `package.json → scripts`. Ordering puts lint last because the
project currently has lint violations (the reason CI's `pnpm lint` is
`continue-on-error: true`); typecheck / test / build are stable and
should run first so a contributor's `pnpm verify` surfaces real failures
before the "known noisy" one.

```json
"verify": "pnpm exec tsc --noEmit && pnpm test && pnpm build && pnpm lint",
"verify:frontend": "pnpm verify",
"verify:strict": "pnpm exec tsc --noEmit && pnpm test && pnpm build && pnpm lint --max-warnings=0",
"verify:go": "cd src-go && go vet ./... && go test ./...",
"verify:bridge": "cd src-bridge && bun run typecheck && bun test",
"verify:all": "pnpm verify && pnpm verify:go && pnpm verify:bridge"
```

Rationale for the double-script shape:

- `pnpm verify` is the everyday script. Lint runs last and any remaining
  violation warnings are informational in the exit-code sense only if
  they were already present before this PR — `pnpm lint` in the current
  repo does exit non-zero on violations, so this DOES fail. That's
  intentional: the contributor learns lint is noisy today and moves on
  after their real work passes.
- `pnpm verify:strict` is what the future `CI_LINT_STRICT=true` world
  looks like. Contributors and CI both will eventually default to this.

Rules:

- CI does **not** call these aggregate scripts. CI keeps its parallel job
  graph for wall-clock efficiency; aggregate scripts are for local dev.
- `CONTRIBUTING.md` gets a "Before pushing" subsection naming
  `pnpm verify` and `pnpm verify:all`, and pointing at the relevant
  per-surface commands for deeper work.

#### 3.2 Pre-push hook

**Not added.** `.husky/pre-commit` already runs `lint-staged` +
`tsc --noEmit`, and CI re-runs everything. A pre-push hook duplicates CI
slowly and discourages fast iteration.

### 4. Documentation sync

Three coordinated edits, grouped by the PR that lands them (see Rollout):

- `CI_CD.md` — PR #2 owns the caching/archival updates (remove
  `deploy.yml` references; add "Disabled examples" section); PR #1 owns
  the concurrency updates (note that four workflows newly have
  concurrency, keeping the existing conditional pattern from `go-ci.yml`
  for workflow_call compatibility); PR #3 owns the SARIF/summaries
  paragraph. Clear sequencing avoids merge conflicts in `CI_CD.md`.
  Additionally, **PR #1 must also fix the stale "desktop-tauri-logic is
  not wired" claim** (line 219) to unblock the truthfulness contract
  downstream edits rely on.
- `CONTRIBUTING.md` — PR #2 adds a "Before pushing" subsection naming
  `pnpm verify` / `pnpm verify:strict` / `pnpm verify:all`.
- `README.md` **and** `README_zh.md` — PR #1 adds a "Community" section
  (or appends to an existing Contributing section if present) with the
  three-link list `SECURITY.md` / `CODE_OF_CONDUCT.md` / `CODEOWNERS`.
  Both READMEs get the same list; the plan will include an explicit
  check-in-sync step.

## Acceptance Criteria

### Phase 1 — Hygiene + concurrency + README sync (PR #1)

| Check | Command / observation | Pass criterion |
|---|---|---|
| Community profile health | `gh api repos/${GITHUB_REPOSITORY}/community/profile --jq .health_percentage` | ≥ 85 |
| Placeholder substitution | `git grep -n '<MAINTAINER_HANDLE>\|<MAINTAINER_CONTACT>' SECURITY.md CODE_OF_CONDUCT.md .github/CODEOWNERS` | no matches |
| CODEOWNERS parsed | Open a PR touching `src-go/` | GitHub auto-requests the configured owner |
| Concurrency on new 4 | `grep -l 'concurrency:' .github/workflows/{build-tauri,desktop-tauri-logic,agent-review,review-layer2}.yml` | 4 matches |
| Concurrency cancels on PR | Two rapid pushes to a PR branch | Older run of `build-tauri` / `desktop-tauri-logic` / `agent-review` shows `cancelled` |
| Release path not cancelled | Tag push simulation (or visual review of the conditional expression) | The group expression for `workflow_call` uses `github.run_id`, not `github.ref`, matching `go-ci.yml:22` |
| README sync | `diff <(grep -A5 'Community\|Contributing' README.md) <(grep -A5 'Community\|Contributing' README_zh.md)` | both READMEs list the three community files |
| `CI_CD.md` stale claim fixed | `git grep 'not currently wired into' CI_CD.md` | no matches |

### Phase 2 — deploy archival + Bun cache + verify scripts (PR #2)

| Check | Command / observation | Pass criterion |
|---|---|---|
| `deploy.yml` gone | `ls .github/workflows/deploy*` | no match |
| Archive preserved | `ls docs/ci-examples/deploy.example.yml` | file exists, starts with the `⚠️` docblock |
| No lingering references | `git grep -l 'deploy.yml' .github/ scripts/ docs/` | no hits except `docs/ci-examples/deploy.example.yml` itself and the `CI_CD.md` "Disabled examples" pointer |
| `CI_CD.md` Disabled examples section | `grep -c 'Disabled examples' CI_CD.md` | ≥ 1 |
| Bun cache hit | Second PR run with no `src-bridge/bun.lockb` change | `actions/cache` step logs `Cache restored from key: Linux-bun-...` |
| `pnpm verify` available | `pnpm run verify --help`-style introspection via `pnpm run` | `verify`, `verify:frontend`, `verify:strict`, `verify:go`, `verify:bridge`, `verify:all` all listed |
| `CONTRIBUTING.md` updated | `grep -n 'pnpm verify' CONTRIBUTING.md` | ≥ 1 hit under a "Before pushing" heading |

### Phase 3 — SARIF / summaries / strict switch (PR #3)

| Check | Command / observation | Pass criterion |
|---|---|---|
| `security-events: write` present | `grep -A5 'permissions:' .github/workflows/quality.yml` | `security-events: write` listed at job scope |
| ESLint annotations on PR | Open a PR with an intentional lint violation | Violation renders as a line-level annotation in the PR Files Changed tab |
| Lint job summary | View `quality` job's Summary on a PR run | Shows total violation count, top files, top rules |
| Audit job summary | Same location | Shows severity counts |
| Outdated job summary | Same location | Shows compact package table |
| Summary scripts exit 0 on malformed input | `node scripts/ci/summarize-lint.js /dev/null` (and audit/outdated equivalents) | Exit code 0 with a "no data" fallback line |
| `.test.ts` siblings exist | `ls scripts/ci/summarize-{lint,audit,outdated}.test.ts` | 3 files |
| All three `.test.ts` pass | `pnpm test scripts/ci/` | green |
| `CI_LINT_STRICT` flip works | Set repo variable `CI_LINT_STRICT=true`, re-run `quality` on a PR with a known lint violation | `quality` job fails at the "ESLint (strict, gated)" step |
| `CI_LINT_STRICT` default behavior | Same PR, variable unset or `false` | `quality` job succeeds; SARIF is still uploaded; summary still renders |
| `CI_CD.md` updated | `grep -n 'SARIF\|step summary' CI_CD.md` | ≥ 1 hit each |

### Phase 4 (future spec, explicitly NOT in this spec)

- Flip `CI_LINT_STRICT` default to `true` and clean up existing lint
  violations in a preparatory PR. That spec will cite this one's switch
  as its preparation and will coordinate `.github/workflows/quality.yml`
  edits with an eslint-rule cleanup PR landed in the same sequence.

## Rollout

Three PRs, staged in order, each independently mergeable:

1. **PR #1 — Community files + four new concurrency groups + README sync
   + stale-claim fix in `CI_CD.md`.**
   - Low blast radius; no runtime behavior change except for PR-level
     run cancellation.
   - Minimum `CI_CD.md` edit: fix the stale "not currently wired" line.
2. **PR #2 — `deploy.yml` archival + Bun cache in `bridge-typecheck` +
   `pnpm verify*` scripts + `CONTRIBUTING.md` "Before pushing" section +
   `CI_CD.md` "Disabled examples" section.**
   - Touches developer ergonomics and cleans up dead code.
3. **PR #3 — SARIF upload + summary scripts + `scripts/ci/*.test.ts` +
   `security-events: write` permission + `CI_LINT_STRICT` switch +
   `CI_CD.md` SARIF/summary paragraph.**
   - Introduces new scripts and permissions; most review-heavy.

Sequencing reasoning: `CI_CD.md` is edited in all three PRs, but each
PR owns a disjoint section — stale-claim fix in #1, "Disabled examples"
in #2, SARIF paragraph in #3. Merge conflicts are structural only and
resolvable mechanically.

## Risks and mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Dependabot PR review queue spikes after CODEOWNERS | Medium | Low | Queue is bounded by existing `open-pull-requests-limit`; review in a batch weekly |
| Bun cache pollutes `src-bridge/node_modules` after upstream Bun change | Low | Low | Key on lockfile hash with `restore-keys:` tier; a Bun upgrade normally moves the lockfile hash |
| `build-tauri`/`desktop-tauri-logic` concurrency group accidentally cancels a release-path invocation | Medium | **High** — would break releases | Match `go-ci.yml:22-23`'s conditional form, which uses `github.run_id` on `workflow_call` so parallel callers stay in unique groups; acceptance criterion explicitly checks this |
| `review-layer2.yml` concurrency group mis-keys on `workflow_run` | Low | Low | Key on the upstream run's head SHA, not `github.ref`; plan will validate with a dry-run on a test PR |
| SARIF upload fails with permission error | Low | Medium | Acceptance criterion checks `permissions: security-events: write` exists at job scope |
| Summary script bug turns green CI red | Low | Low | `.test.ts` malformed-input case locks in exit-0 fail-safe |
| `pnpm verify` frustrates contributors because lint always fails | Medium | Low | Ordering puts lint LAST so real failures surface first; `verify:strict` documents the future expectation |
| Placeholders ship unsubstituted | Low | Medium | Acceptance criterion greps for both `<MAINTAINER_HANDLE>` and `<MAINTAINER_CONTACT>` in merged diff |
| CODEOWNERS placeholder syntax silently ignored by GitHub | Low | Low | Grep check (above) catches literal placeholders in merge |

## Consistency constraints

- Every new `scripts/ci/*.js` file MUST have a `scripts/ci/*.test.ts`
  sibling mirroring the convention in `scripts/build/` / `scripts/dev/`
  / `scripts/release/` / `scripts/skills/` / `scripts/i18n/` /
  `scripts/plugin/`.
- `CI_CD.md` is the canonical runtime truth for the CI graph. Any
  workflow change in this spec's implementation MUST also update
  `CI_CD.md` in the same PR. PR #1 must additionally fix the known
  stale line about `desktop-tauri-logic` being unwired.
- No new manifest files, no new lockfiles, no new version-management
  tool configs. A future release-automation spec gets its own review pass.
- No changes to `agent-review.yml` / `review-layer2.yml` beyond adding
  their concurrency blocks. Broader review of those workflows' secret
  handling is explicitly deferred.
- No changes to existing caches. pnpm / Go / Next.js / rust-cache all
  stay as-is.

## Open questions (for implementer to resolve)

- Exact value for `<MAINTAINER_HANDLE>` and the Code of Conduct's
  `<MAINTAINER_CONTACT>` handle.
- Whether the `review-layer2.yml` concurrency group keys on the upstream
  `workflow_run.head_sha` or another property — to be nailed down in the
  plan after reading `actions/workflow_run`-event payload shape.
- Whether `CHANGELOG.md` should get a "Repo hygiene" section summarizing
  the three PRs; default stance is "yes, one entry per PR under an
  Unreleased → Infrastructure header." Confirm during implementation.

## References

- Repo-root `CI_CD.md` (existing CI reality, partially stale as noted)
- Repo-root `CONTRIBUTING.md` (current contributor flow)
- Repo-root `LICENSE` (MIT © AstroAir 2024)
- `.github/dependabot.yml` (how automated dep updates are grouped today)
- `.github/workflows/ci.yml` (existing concurrency at line 35-37; existing
  desktop-tauri-logic wiring at lines 60-62 and 96)
- `.github/workflows/go-ci.yml:22-23` (template for the workflow_call-safe
  concurrency expression)
- `.github/workflows/quality.yml` (lint/audit/outdated current steps)
- GitHub docs: [Adding a SECURITY.md](https://docs.github.com/en/code-security/getting-started/adding-a-security-policy-to-your-repository)
- [Contributor Covenant v2.1](https://www.contributor-covenant.org/version/2/1/code_of_conduct/)
- [GitHub SARIF upload](https://docs.github.com/en/code-security/code-scanning/integrating-with-code-scanning/sarif-support-for-code-scanning)
- [ESLint SARIF formatter](https://www.npmjs.com/package/@microsoft/eslint-formatter-sarif)
