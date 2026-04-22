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

AgentForge today has most of the scaffolding a mature OSS repo expects —
README / CONTRIBUTING / TESTING / LICENSE / CHANGELOG / mkdocs / 10 workflows /
Dependabot with groupings — but three categories of gaps remain visible:

1. **Community standards files are incomplete.** `SECURITY.md`,
   `CODE_OF_CONDUCT.md`, and `CODEOWNERS` are absent. Without them the GitHub
   community profile is < 85%, private vulnerability disclosure has no clear
   channel, and PR review routing relies on the author remembering to add
   reviewers.
2. **CI/CD has "paper-tiger" gates.** `pnpm lint`, `pnpm audit`, and
   `pnpm outdated` in `quality.yml` are all `continue-on-error: true`, so
   regressions can enter `master` silently. `desktop-tauri-logic.yml` has an
   80% coverage gate for the Tauri crate but is not wired into `ci.yml`.
   `deploy.yml` is hard-coded to `DEPLOY_ENABLED=false` and reads as live
   infra when it is actually dead code.
3. **CI budget is spent on cold caches and duplicated runs.** No workflow
   declares a `concurrency` group, so a PR with ten rapid pushes keeps ten
   full runs alive. Rust, Bun, and pnpm caches are not configured, so every
   Tauri build re-compiles cold.

The fix is not dramatic — it is a coherent pass that removes current friction
without committing the project to maintenance it cannot sustain in its
current "internal testing, breaking changes freely permitted" stage (per
MEMORY).

Release automation (release-please / changesets), CodeQL, SBOM, notarization,
Renovate, and a `security@` SLA are **explicitly out of scope** — they are
the right investments once the project is courting external consumers, not
during internal iteration.

## Goals

- Raise GitHub community-profile health to ≥ 85%.
- Remove the dead `deploy.yml` while preserving future reference.
- Cut average CI wall-clock time on no-op PR pushes by cancelling superseded
  runs via `concurrency`.
- Cut Tauri cold-build time via `rust-cache` and pnpm/bun caching.
- Surface lint and audit violations as PR annotations + job summaries
  without flipping the gates to blocking (blocking is a separate future
  spec).
- Wire `desktop-tauri-logic.yml` into `ci.yml` so the Tauri coverage gate
  actually runs on the relevant PRs.
- Give contributors a one-liner local verification (`pnpm verify`).

## Non-Goals

- Flip `pnpm lint` from `continue-on-error: true` to blocking. Deferred to a
  follow-up spec so that this spec can land without a master red-light
  period. An env switch (`CI_LINT_STRICT`) is planted for the follow-up.
- Introduce release automation (release-please, changesets, semantic-release).
- Introduce security tooling beyond what GitHub ships natively (CodeQL,
  Snyk, SBOM, cosign).
- Replace Dependabot with Renovate.
- Introduce macOS notarization or Windows code-signing certificates.
- Commit to a `security@` email or a time-bound disclosure SLA.
- Add `.github/FUNDING.yml`.
- Introduce a public issue template for security reports (reports must flow
  through GitHub Security Advisory, not public issues).

## Current State (Repo Truth — 2026-04-22)

### Present
- Community: LICENSE (MIT/Apache? — TBD at implementation), README,
  README_zh, CONTRIBUTING, CHANGELOG, AGENTS, CI_CD, TESTING.
- `.github/`: dependabot.yml (npm + cargo + actions with groups), PR
  template, three issue templates (bug/feature/config), copilot
  instructions, 10 workflows (ci / quality / test / go-ci / build-tauri /
  desktop-tauri-logic / release / deploy / agent-review / review-layer2).
- Scripts: `scripts/` has build/dev/plugin/release/skills/i18n/smoke
  subtrees, each with `.test.ts` or `.test.mjs` coverage.
- Dev workflow: `pnpm dev:all` / `pnpm dev:backend` families with
  status/stop/logs/watch/verify/restart.

### Missing
- `SECURITY.md`
- `CODE_OF_CONDUCT.md`
- `.github/CODEOWNERS`
- Any `concurrency:` group in any workflow
- Rust cargo cache, Bun install cache, pnpm store cache
- Lint/audit output visibility in PR UI (currently only visible in raw
  job logs)
- A wiring between `desktop-tauri-logic.yml` and `ci.yml`
- An aggregate local-verify script

### Known "paper tiger" behaviors
- `pnpm lint` step in `quality.yml`: `continue-on-error: true`
- `pnpm audit --audit-level=moderate`: `continue-on-error: true`
- `pnpm outdated`: `continue-on-error: true`
- `deploy.yml`: `DEPLOY_ENABLED` hard-coded to `false`; Vercel steps
  commented out; not called from `ci.yml`.

## Design

### 1. Repository hygiene files

#### 1.1 `SECURITY.md` (repo root)

One-page, scoped to current project posture.

Sections:

- **Supported versions** — "AgentForge is pre-1.0. The `master` branch
  receives security fixes. No LTS branch is maintained."
- **Reporting a vulnerability** — Route through
  GitHub → Security → Report a vulnerability (private advisory). Do **not**
  open public issues for security reports.
- **Scope** — Go orchestrator (`src-go/`), TS bridge (`src-bridge/`), IM
  bridge (`src-im-bridge/`), Tauri desktop shell (`src-tauri/`), Next.js
  frontend (`app/`, `components/`, `lib/`), marketplace microservice
  (`src-marketplace/`).
- **Out of scope** — Third-party plugin vulnerabilities (report to plugin
  author), upstream library CVEs not yet patched (report upstream first),
  self-hosted deployment misconfiguration, social-engineering attacks on
  maintainers.
- **Response expectations** — "Best-effort triage; no time-bound SLA
  during internal testing." Explicit rather than implicit.
- **Disclosure policy** — Coordinated disclosure preferred; reporters who
  follow the private channel are credited in the fix commit.

#### 1.2 `CODE_OF_CONDUCT.md` (repo root)

- Verbatim **Contributor Covenant v2.1** (stable well-known text).
- Contact block: placeholder `<MAINTAINER_CONTACT>` filled during
  implementation (GitHub handle via `@mention` or a private email — your
  choice at impl time).

#### 1.3 `.github/CODEOWNERS`

Minimal-but-useful seed. New maintainers can slot into sub-paths later
without a schema overhaul.

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

Risk: Dependabot PRs auto-request the CODEOWNER. Mitigations considered:

- Leave the default `*` owner covering Dependabot PRs — accept a short
  review backlog uptick (chosen default).
- Alternatively exclude `package.json` / `pnpm-lock.yaml` / `go.sum` /
  `Cargo.lock` from CODEOWNERS by listing them after the default with no
  owner. **Not chosen** — it defeats the point of routing for security-
  relevant dependency bumps.

### 2. CI workflow hardening

#### 2.1 Concurrency groups

Every workflow except `release.yml` gets:

```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: ${{ github.event_name == 'pull_request' }}
```

Rationale:

- `github.workflow` + `github.ref` means a PR branch's `ci` runs are
  grouped, its `quality` runs are separately grouped, and so on — no
  accidental cancellation across unrelated workflows.
- `cancel-in-progress` only fires on `pull_request`. On `master` or tag
  pushes, every run completes so we never lose the authoritative
  integration/release record.
- `release.yml` is excluded: tag pushes are rare, and cancelling a
  half-built release is worse than the cost of letting it run.

Affected files:
`ci.yml`, `quality.yml`, `test.yml`, `go-ci.yml`, `build-tauri.yml`,
`desktop-tauri-logic.yml`, `agent-review.yml`, `review-layer2.yml`.

#### 2.2 Caching matrix

| Ecosystem | Today | After |
|---|---|---|
| Node + pnpm | `pnpm install --frozen-lockfile` cold each run | `actions/setup-node@v4` with `cache: 'pnpm'`; `pnpm/action-setup@v4` to pin version |
| Go (`src-go`, `src-im-bridge`) | No explicit cache | `actions/setup-go@v5` with `cache: true` for each module (two independent setups) |
| Bun (`src-bridge`) | Cold install every run | `actions/cache@v4` keyed on `hashFiles('src-bridge/bun.lockb')`, paths: `~/.bun/install/cache`, `src-bridge/node_modules` |
| Rust (Tauri) | No cache | `Swatinem/rust-cache@v2` in `build-tauri.yml` and `desktop-tauri-logic.yml`, workspace-scoped |

Known risk: Bun cache restoration on a stale cache key can leave stale
`node_modules`. Mitigation: key includes both lockfile hash AND
`bun --version` output via `restore-keys` tier, so a Bun upgrade invalidates
the cache.

Target impact: the `build-tauri` matrix currently takes 20–25 min per
platform from cold. `rust-cache` alone should reduce warm runs by
30–50% on top of whatever upstream caching is already implicit.

#### 2.3 Lint / audit visibility (non-blocking)

Rewrite the lint step in `quality.yml`:

```yaml
- name: Lint (soft, annotated)
  id: lint
  continue-on-error: true
  env:
    # Future: set CI_LINT_STRICT=true to flip this gate to blocking
    CI_LINT_STRICT: ${{ vars.CI_LINT_STRICT || 'false' }}
  run: |
    pnpm lint \
      --format @microsoft/eslint-formatter-sarif \
      --output-file eslint.sarif || echo "lint exited non-zero"
    if [ "$CI_LINT_STRICT" = "true" ]; then
      pnpm lint
    fi

- name: Upload lint SARIF
  if: always()
  uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: eslint.sarif
    category: eslint

- name: Lint summary
  if: always()
  run: node scripts/ci/summarize-lint.js eslint.sarif >> "$GITHUB_STEP_SUMMARY"
```

Same pattern for `pnpm audit` and `pnpm outdated`, each producing a job
summary fragment via a small script.

New scripts:

- `scripts/ci/summarize-lint.js` — reads an ESLint SARIF file, emits
  Markdown for `$GITHUB_STEP_SUMMARY` (total count, top-N files, top-N
  rules).
- `scripts/ci/summarize-audit.js` — reads `pnpm audit --json` output,
  emits severity counts + top advisories.
- `scripts/ci/summarize-outdated.js` — reads `pnpm outdated --json`,
  emits a compact table (name, current, wanted, latest, type).

Each script requires a `.test.ts` covering:

- happy-path input
- empty input (no violations)
- malformed input (graceful fallback, exit 0 so summary never breaks CI)

Why fail-safe (exit 0) on malformed input: these scripts are reporting,
not gating. A summary-generation bug must not turn a green CI red. The
tests make the fallback explicit so it isn't drift.

#### 2.4 Wire `desktop-tauri-logic.yml` into `ci.yml`

Current state: `desktop-tauri-logic.yml` exists and enforces an 80%
line+function coverage gate on the `src-tauri/` crate using
`cargo-llvm-cov`. It is only runnable manually (`workflow_dispatch`) and
via `workflow_call` — nothing in `ci.yml` calls it.

Change: add a `desktop-logic` job to `ci.yml`:

```yaml
desktop-logic:
  needs: [quality, test]
  if: |
    contains(github.event.pull_request.changed_files, 'src-tauri/') ||
    contains(github.event.pull_request.changed_files, 'scripts/build/') ||
    github.event_name == 'push'
  uses: ./.github/workflows/desktop-tauri-logic.yml
```

*(The final path-filter shape will use `dorny/paths-filter` or
`changed-files` action — the one-liner above is indicative. See "Rollout"
below.)*

`build-tauri` keeps its existing `needs:` chain; `desktop-logic` runs
parallel to it, not as a gate on it. This keeps end-to-end CI time from
regressing.

Path filter must include:

- `src-tauri/**`
- `scripts/build/build-backend*` (sidecar compilation feeds the desktop
  logic tests indirectly)
- `scripts/build/build-bridge*`
- `scripts/build/build-im-bridge*`

#### 2.5 `deploy.yml` decommission

- `git mv .github/workflows/deploy.yml docs/ci-examples/deploy.example.yml`
- Prefix the example file with a docblock explaining "this is a disabled
  reference; to activate, copy back to `.github/workflows/deploy.yml`,
  flip `DEPLOY_ENABLED=true`, add required secrets".
- Remove any reference to `deploy.yml` from `CI_CD.md`; add a note under
  a new "Disabled examples" section pointing to `docs/ci-examples/`.

### 3. Local developer experience

#### 3.1 Aggregate verify scripts

Add to `package.json → scripts`:

```json
"verify": "pnpm lint && pnpm exec tsc --noEmit && pnpm test && pnpm build",
"verify:frontend": "pnpm verify",
"verify:go": "cd src-go && go vet ./... && go test ./...",
"verify:bridge": "cd src-bridge && bun run typecheck && bun test",
"verify:all": "pnpm verify:frontend && pnpm verify:go && pnpm verify:bridge"
```

Rules:

- CI **does not** call `verify*` scripts. CI keeps its parallel job graph.
- Contributor-facing: one command before opening a PR.
- `CONTRIBUTING.md` gets a "Before pushing" subsection that names
  `pnpm verify` and `pnpm verify:all`.

#### 3.2 Pre-push hook

**Not added.** `pre-commit` already runs `lint-staged` + `tsc --noEmit`,
and CI will re-run everything. A pre-push hook duplicates CI slowly and
discourages fast iteration.

### 4. Documentation sync

- `CI_CD.md` — rewrite the job-graph section to reflect:
  - concurrency groups exist
  - `desktop-logic` is wired
  - `deploy.yml` is archived (with pointer)
  - caching matrix exists
- `CONTRIBUTING.md` — add a "Before pushing" subsection naming
  `pnpm verify` / `pnpm verify:all` and pointing at the relevant
  per-surface commands.
- `SECURITY.md` link from README's header table (if README has one) or
  from a "Security" section (to be checked at impl time).
- `README.md` / `README_zh.md` — add the three new community files to
  whatever docs index is already there.

## Acceptance Criteria

### Phase 1 — Hygiene + plumbing (first landing)

| Check | Command / observation | Pass criterion |
|---|---|---|
| Community profile | `gh api repos/Arxtect/AgentForge/community/profile --jq .health_percentage` | ≥ 85 |
| CODEOWNERS is parsed | Open PR touching `src-go/` | GitHub auto-requests `@<MAINTAINER_HANDLE>` |
| Concurrency cancels | `git push` twice on same branch within 30s | Older run shows `cancelled` in Actions UI |
| pnpm cache hit | Second PR run | `Cache restored from key` line present; install step < 30s |
| Rust cache hit | Second `build-tauri` run on same PR | `rust-cache` step logs a restore |
| `deploy.yml` gone | `ls .github/workflows/deploy*` | no match |
| `deploy.example.yml` preserved | `ls docs/ci-examples/deploy.example.yml` | file exists |

### Phase 2 — Visibility

| Check | Command / observation | Pass criterion |
|---|---|---|
| ESLint annotations on PR | Open PR with an intentional lint violation | Violation is rendered as PR-level annotation on the offending line |
| Lint job summary | View the `quality` job on a run | Summary section shows total count + top files |
| `desktop-logic` runs on Tauri PR | PR touching `src-tauri/**` | `desktop-logic` job appears in checks list |
| `desktop-logic` skipped on FE-only PR | PR touching only `components/**` | `desktop-logic` job skipped or absent |
| `CI_LINT_STRICT=true` flips behavior | Set the repo variable to `true`, re-run `quality` | `quality` job fails on the same violation that previously passed |

### Phase 3 (future spec, not this one)

- Flip `CI_LINT_STRICT` default to `true`.
- Clean up existing lint violations in a preparatory PR.
- Spec will cite this spec's `CI_LINT_STRICT` switch as its preparation.

## Rollout

Three-PR staging, all on `master`:

1. **PR #1 — Community files + concurrency.** Lowest risk, zero runtime
   behavior change. New files + concurrency blocks in every workflow.
2. **PR #2 — Caching matrix + `deploy.yml` archival + `pnpm verify*` +
   `CONTRIBUTING.md` update + `CI_CD.md` update.** Behavior change is
   CI-internal (faster runs); local-dev ergonomics improve.
3. **PR #3 — SARIF / summaries + `scripts/ci/*` with tests +
   `desktop-logic` wire-in.** Introduces the new scripts with tests.

Keeping them separate PRs is deliberate:

- If the path-filter for `desktop-logic` misbehaves, it's a PR-3 revert,
  not a PR-1 revert.
- Community-file additions are the cleanest diff to cite in the
  announcement/commit message and help anyone auditing the history.

## Risks and mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Dependabot PR review backlog after CODEOWNERS lands | High | Low | Accept the ~5-10 PR queue; revisit if it grows past ~20 |
| Bun cache restoration picks up stale `node_modules` | Medium | Medium | Key on lockfile hash + `bun --version`; explicit `restore-keys` tier |
| `desktop-logic` path filter misses a case | Medium | Low | Include `scripts/build/**` in filter; ship `workflow_dispatch` as escape hatch |
| ESLint SARIF formatter drops a key under certain versions | Low | Low | `summarize-lint.js` treats missing fields as zero and tests cover malformed input |
| `pnpm verify` fails on a developer's machine due to a new Go/Bun prerequisite | Low | Low | `verify` covers frontend only by default; `verify:all` is opt-in and documented |
| `SARIF upload` quota / size limit | Low | Low | SARIF is emitted only for ESLint; project size is well within GitHub's limit |

## Consistency constraints

- Every new `scripts/ci/*.js` file MUST have a `scripts/ci/*.test.ts` sibling
  mirroring the convention in `scripts/build/` / `scripts/dev/`.
- `CI_CD.md` is the canonical runtime truth for the CI graph. Any workflow
  change in this spec's implementation MUST also update `CI_CD.md` in the
  same PR.
- No new manifest files, no new lockfiles, no new version-management tool
  configs. If a future spec chooses release-please, it gets its own spec
  and its own reviewer pass.

## Open questions (for implementer to resolve)

- Exact `<MAINTAINER_HANDLE>` and `<MAINTAINER_CONTACT>` values.
- Exact path-filter action — `dorny/paths-filter@v3` vs
  `tj-actions/changed-files@v45` vs the native
  `on.pull_request.paths` filter (the simplest). Pick at impl time
  based on whether `desktop-logic` needs to run on non-PR events.
- Whether to include or exclude `docs/**` from `desktop-logic`'s path
  filter (default: exclude; desktop-logic doesn't read docs).

## References

- Repo-root `CI_CD.md` (current CI reality)
- Repo-root `CONTRIBUTING.md` (current contributor flow)
- `.github/dependabot.yml` (how automated dep updates are grouped today)
- GitHub docs: [Adding a SECURITY.md](https://docs.github.com/en/code-security/getting-started/adding-a-security-policy-to-your-repository)
- [Contributor Covenant v2.1](https://www.contributor-covenant.org/version/2/1/code_of_conduct/)
- [`Swatinem/rust-cache`](https://github.com/Swatinem/rust-cache)
- [GitHub SARIF upload](https://docs.github.com/en/code-security/code-scanning/integrating-with-code-scanning/sarif-support-for-code-scanning)
