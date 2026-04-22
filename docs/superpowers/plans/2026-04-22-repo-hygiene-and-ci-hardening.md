# Repo Hygiene + CI Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close three narrow gaps in AgentForge's repo hygiene and CI posture: add missing community standards files (SECURITY / CoC / CODEOWNERS), add `concurrency:` to the four workflows that lack it, archive the dead `deploy.yml`, cache the one uncached Bun install, add local `pnpm verify*` scripts, and add ESLint-SARIF / audit / outdated visibility layers with a planted `CI_LINT_STRICT` switch.

**Architecture:** Three disjoint PRs landed in order. PR #1 is low-risk community-files + concurrency. PR #2 is developer-ergonomics and dead-code cleanup. PR #3 introduces three new small Node scripts (with `.test.ts` siblings) and rewrites `quality.yml` with per-step SARIF / JSON reporting.

**Tech Stack:** YAML (GitHub Actions), Markdown, Node.js (for `scripts/ci/*.js`), Jest + ts-jest (for `.test.ts` siblings), pnpm 10, Next.js 16 root, Tauri 2 desktop, Go 1.25 backends, Bun for `src-bridge/`. No new tools or deps.

**Source of truth spec:** `docs/superpowers/specs/2026-04-22-repo-hygiene-and-ci-hardening-design.md`

---

## Pre-flight (once, before Task 1)

### Task 0: Nail down the two placeholders

The spec uses two placeholders that MUST be substituted before any merge. Decide the values now and write them into a scratch note you'll reference.

**Files:** none edited yet.

- [ ] **Step 1: Identify GitHub handle**

  Read `.git/config` and `package.json` to confirm the repo's canonical org/repo (expect `Arxtect/AgentForge`). Decide the GitHub handle (likely `@MaxQian` or an org team like `@Arxtect/maintainers`). If unsure, ask the user.

  Run: `rtk git remote -v`

  Record: `MAINTAINER_HANDLE=@<the-handle>` (without the angle brackets in the actual value — e.g. `@MaxQian`).

- [ ] **Step 2: Identify CoC contact handle**

  Per spec §1.2, this MUST be a GitHub handle form (no email). It can be the same handle as step 1 or a different one (e.g. a `@Arxtect/security` team).

  Record: `MAINTAINER_CONTACT=@<the-handle>`.

- [ ] **Step 3: Sanity-check the handles resolve**

  Run: `gh api users/<handle-without-@>` for each handle and verify no 404. For a team handle like `@Arxtect/maintainers`, use `gh api orgs/Arxtect/teams/maintainers`.

  Expected: 200 response.

**No commit for Task 0.** The values are recorded in-memory for the rest of the plan.

---

## PR #1 — Community files + concurrency + README sync + CI_CD stale fix

**Branch:** `ci/repo-hygiene-phase-1-community-and-concurrency`

### Task 1: Write `SECURITY.md`

**Files:**
- Create: `SECURITY.md` (repo root)

- [ ] **Step 1: Write the file**

  ````markdown
  # Security Policy

  ## Supported Versions

  AgentForge is pre-1.0 and licensed under MIT (see [LICENSE](./LICENSE)).
  The `master` branch receives security fixes. No LTS branch is maintained
  during internal testing.

  | Branch / Tag | Status |
  | --- | --- |
  | `master` | Actively maintained |
  | Any tagged release | Not individually patched; upgrade to latest `master` |

  ## Reporting a Vulnerability

  **Do not open a public GitHub issue for security reports.**

  Use GitHub's private vulnerability reporting:

  1. Navigate to the repository's **Security** tab.
  2. Click **Report a vulnerability**.
  3. Describe the issue, the affected surface, and reproduction steps.

  No email address is provided. All reports flow through the GitHub
  Security Advisory channel.

  ## Scope

  Security reports are accepted for:

  - Go orchestrator (`src-go/`)
  - TypeScript bridge (`src-bridge/`)
  - IM bridge (`src-im-bridge/`)
  - Tauri desktop shell (`src-tauri/`)
  - Next.js frontend (`app/`, `components/`, `lib/`)
  - Marketplace microservice (`src-marketplace/`)

  ## Out of Scope

  - Third-party plugin vulnerabilities — report to the plugin author.
  - Upstream library CVEs that are not yet patched upstream — report
    upstream first; we will update once an upstream fix ships.
  - Self-hosted deployment misconfiguration.
  - Social-engineering attacks on maintainers.

  ## Response Expectations

  Best-effort triage. No time-bound SLA is promised during internal
  testing.

  ## Disclosure Policy

  Coordinated disclosure preferred. Reporters who follow the private
  channel above are credited in the fix commit message unless they
  request anonymity.
  ````

- [ ] **Step 2: Verify structure**

  Run: `rtk grep "^## " SECURITY.md`

  Expected output includes: Supported Versions, Reporting a Vulnerability, Scope, Out of Scope, Response Expectations, Disclosure Policy.

- [ ] **Step 3: Commit**

  ```bash
  rtk git add SECURITY.md
  rtk git commit -m "docs: add SECURITY.md routing reports through GitHub advisory"
  ```

### Task 2: Write `CODE_OF_CONDUCT.md`

**Files:**
- Create: `CODE_OF_CONDUCT.md` (repo root)

- [ ] **Step 1: Write the file**

  Use the verbatim Contributor Covenant v2.1 text from <https://www.contributor-covenant.org/version/2/1/code_of_conduct/>. In the **Enforcement** section (the third-to-last heading), set the contact line to:

  ```markdown
  Instances of abusive, harassing, or otherwise unacceptable behavior may be
  reported to the community leaders responsible for enforcement at <MAINTAINER_CONTACT>.
  All complaints will be reviewed and investigated promptly and fairly.
  ```

  Substitute `<MAINTAINER_CONTACT>` with the value recorded in Task 0 Step 2 (a GitHub handle form, e.g. `@MaxQian`). Do **not** use an email address — this is constrained by the spec's no-SLA Non-Goal.

- [ ] **Step 2: Verify no placeholder remains**

  Run: `rtk grep -n "MAINTAINER_CONTACT\|<handle>\|example@\|security@" CODE_OF_CONDUCT.md`

  Expected: no matches. If anything is returned, substitution was incomplete.

- [ ] **Step 3: Commit**

  ```bash
  rtk git add CODE_OF_CONDUCT.md
  rtk git commit -m "docs: add Contributor Covenant v2.1 code of conduct"
  ```

### Task 3: Write `.github/CODEOWNERS`

**Files:**
- Create: `.github/CODEOWNERS`

- [ ] **Step 1: Write the file**

  ```
  # CODEOWNERS for AgentForge
  # Reviewers listed here are automatically requested on PRs touching the
  # matching paths. Keep this minimal; future maintainers can slot in per
  # sub-stack without a schema overhaul.

  # Default owner — covers everything not matched below
  *                           <MAINTAINER_HANDLE>

  # Infra & CI
  /.github/                   <MAINTAINER_HANDLE>
  /scripts/                   <MAINTAINER_HANDLE>

  # Sub-stacks
  /src-go/                    <MAINTAINER_HANDLE>
  /src-bridge/                <MAINTAINER_HANDLE>
  /src-im-bridge/             <MAINTAINER_HANDLE>
  /src-tauri/                 <MAINTAINER_HANDLE>
  /src-marketplace/           <MAINTAINER_HANDLE>

  # Docs
  /docs/                      <MAINTAINER_HANDLE>
  ```

  Substitute `<MAINTAINER_HANDLE>` everywhere with the value from Task 0 Step 1 (e.g. `@MaxQian`). Leading `@` is required for a user; no `@` for a team `@org/team` path — check `gh` format.

- [ ] **Step 2: Verify no placeholder remains**

  Run: `rtk grep -n "MAINTAINER_HANDLE" .github/CODEOWNERS`

  Expected: no matches.

- [ ] **Step 3: Verify GitHub parses it**

  Run: `gh api repos/:owner/:repo/codeowners/errors 2>/dev/null || true`

  (Only available after push; this step is a reminder — don't block on it. If the PR is opened and GitHub shows a CODEOWNERS parse error comment, treat that as a Task 3 regression and fix before merging.)

- [ ] **Step 4: Commit**

  ```bash
  rtk git add .github/CODEOWNERS
  rtk git commit -m "ci: add .github/CODEOWNERS with minimal per-stack ownership"
  ```

### Task 4: Add concurrency to `build-tauri.yml`

**Files:**
- Modify: `.github/workflows/build-tauri.yml` (insert between `on:` block and `jobs:` block — after line 42)

- [ ] **Step 1: Read the file to find the insertion point**

  Run: Read `.github/workflows/build-tauri.yml` lines 40-46.

  Expected: line 42 ends `workflow_dispatch:`; line 43 is blank; line 44 is `jobs:`.

- [ ] **Step 2: Insert the concurrency block**

  Add this block after the `workflow_dispatch:` line and before `jobs:` (mirror `go-ci.yml:18-23`'s conditional form so `workflow_call` invocations from `release.yml` don't get cancelled by PR invocations from `ci.yml`):

  ```yaml
  concurrency:
    # For PR: group by ref so older runs are cancelled.
    # For workflow_call (release.yml invocation): use run_id so parallel
    # callers never cancel each other, matching go-ci.yml:22.
    group: ${{ github.event_name == 'workflow_call' && github.run_id || format('build-tauri-{0}', github.ref) }}
    cancel-in-progress: ${{ github.event_name == 'pull_request' }}
  ```

- [ ] **Step 3: Verify YAML parses**

  Run: `node -e "const yaml = require('yaml'); yaml.parse(require('fs').readFileSync('.github/workflows/build-tauri.yml', 'utf8'))"`

  Expected: exit 0, no output.

- [ ] **Step 4: Commit**

  ```bash
  rtk git add .github/workflows/build-tauri.yml
  rtk git commit -m "ci(build-tauri): add workflow_call-safe concurrency group"
  ```

### Task 5: Add concurrency to `desktop-tauri-logic.yml`

**Files:**
- Modify: `.github/workflows/desktop-tauri-logic.yml` (insert between `on:` and `jobs:` — after line 5)

- [ ] **Step 1: Read the file**

  Read `.github/workflows/desktop-tauri-logic.yml` lines 1-10.

  Expected: `on:` block ends line 5 (`workflow_dispatch:`); line 6 blank; line 7 `jobs:`.

- [ ] **Step 2: Insert the same conditional concurrency block**

  ```yaml
  concurrency:
    group: ${{ github.event_name == 'workflow_call' && github.run_id || format('desktop-tauri-logic-{0}', github.ref) }}
    cancel-in-progress: ${{ github.event_name == 'pull_request' }}
  ```

- [ ] **Step 3: Verify YAML parses**

  Same `node -e` check as Task 4 Step 3, pointed at this file.

- [ ] **Step 4: Commit**

  ```bash
  rtk git add .github/workflows/desktop-tauri-logic.yml
  rtk git commit -m "ci(desktop-tauri-logic): add workflow_call-safe concurrency group"
  ```

### Task 6: Add concurrency to `agent-review.yml`

**Files:**
- Modify: `.github/workflows/agent-review.yml` (insert between `on:` and `jobs:` — after line 5)

- [ ] **Step 1: Read the file**

  Read `.github/workflows/agent-review.yml` lines 1-10.

- [ ] **Step 2: Insert the concurrency block** (no `workflow_call` path here — simpler form)

  ```yaml
  concurrency:
    group: agent-review-${{ github.ref }}
    cancel-in-progress: ${{ github.event_name == 'pull_request' }}
  ```

- [ ] **Step 3: Verify YAML parses**

  Same check.

- [ ] **Step 4: Commit**

  ```bash
  rtk git add .github/workflows/agent-review.yml
  rtk git commit -m "ci(agent-review): add concurrency group cancelling superseded PR reviews"
  ```

### Task 7: Add concurrency to `review-layer2.yml`

**Files:**
- Modify: `.github/workflows/review-layer2.yml` (insert between `on:` and `jobs:` — after line 6)

- [ ] **Step 1: Read the file**

  Read `.github/workflows/review-layer2.yml` lines 1-10. Note: trigger is `workflow_run`, so `github.ref` is the ref of the upstream run; key on `github.event.workflow_run.head_sha` to correctly group by the PR commit, not by the ref the workflow-run event carries.

- [ ] **Step 2: Insert the concurrency block**

  ```yaml
  concurrency:
    group: review-layer2-${{ github.event.workflow_run.head_sha }}
    cancel-in-progress: true
  ```

  Rationale: `workflow_run` events don't have a `pull_request` event_name so the conditional cancel-in-progress guard doesn't apply. Cancelling in-progress is safe here because a later layer-1 completion for the same head SHA means the earlier layer-2 would operate on stale metadata anyway.

- [ ] **Step 3: Verify YAML parses**

  Same check.

- [ ] **Step 4: Commit**

  ```bash
  rtk git add .github/workflows/review-layer2.yml
  rtk git commit -m "ci(review-layer2): group concurrency by upstream head SHA"
  ```

### Task 8: Fix `CI_CD.md` stale claims (PR #1's share)

**Files:**
- Modify: `CI_CD.md` (two edits)

- [ ] **Step 1: Locate and fix the stale `desktop-tauri-logic` claim**

  Run: `rtk grep -n "not currently wired into" CI_CD.md`

  Expected: one hit around line 219.

  Edit that block (section "### 7. Desktop Tauri Logic", paragraph "Current role in the pipeline") to replace:

  ```
  - This workflow is callable and manually runnable.
  - It is not currently wired into `.github/workflows/ci.yml`.
  - The repo-level scripts `pnpm test:tauri` and `pnpm test:tauri:coverage`
    mirror this validation seam for local use.
  ```

  with:

  ```
  - This workflow is callable, manually runnable, and wired into the main
    pipeline: `ci.yml` calls it at step 4 (see §"Main CI Orchestrator")
    and `release.yml` calls it before `build-tauri`. It is a hard
    `needs:` dependency of the desktop build in both workflows.
  - The repo-level scripts `pnpm test:tauri` and `pnpm test:tauri:coverage`
    mirror this validation seam for local use.
  ```

- [ ] **Step 2: Locate and update the main job-graph section to note concurrency**

  Run: `rtk grep -n "The current job graph is:" CI_CD.md`

  Find the "Main CI Orchestrator" section. Add a short subsection at the end of that section:

  ```markdown
  ### Concurrency

  All workflows (except `release.yml` which uses `cancel-in-progress: false`
  intentionally) declare a `concurrency:` group. For the `workflow_call`-
  reusable workflows (`quality.yml`, `test.yml`, `go-ci.yml`,
  `build-tauri.yml`, `desktop-tauri-logic.yml`), the group key is
  conditional on `github.event_name` so a `ci.yml` invocation cannot
  cancel a parallel `release.yml` invocation (see `go-ci.yml:22` for the
  canonical form).
  ```

  Also update the job-graph list to name `desktop-tauri-logic` in step 4 if it's not already listed (per `ci.yml:60-62` today, it should be). If the list still shows "1. quality 2. test 3. go-ci 4. bridge-typecheck 5. im-bridge-build 6. build-tauri", update to:

  ```
  1. `quality`
  2. `test`
  3. `go-ci`
  4. `desktop-tauri-logic`
  5. `bridge-typecheck`
  6. `im-bridge-build`
  7. `build-tauri`

  `build-tauri` only starts after the first six jobs succeed.
  ```

- [ ] **Step 3: Verify no "not currently wired" phrase remains**

  Run: `rtk grep "not currently wired" CI_CD.md`

  Expected: no matches.

- [ ] **Step 4: Commit**

  ```bash
  rtk git add CI_CD.md
  rtk git commit -m "docs(ci): correct stale desktop-tauri-logic wiring claim + document concurrency"
  ```

### Task 9: Sync Community section into `README.md` + `README_zh.md`

**Files:**
- Modify: `README.md`
- Modify: `README_zh.md`

- [ ] **Step 1: Check existing structure in both READMEs**

  Run: `rtk grep -n "^## \|^### " README.md | head -40`
  Run: `rtk grep -n "^## \|^### " README_zh.md | head -40`

  Identify whether a "Contributing" or "Community" section exists. If one exists, append to it. If neither exists, add a new `## Community` section near the end (before License).

- [ ] **Step 2: Add the three-link block to `README.md`**

  Append to the identified section (or create new):

  ```markdown
  ## Community

  - **[Security Policy](./SECURITY.md)** — how to report vulnerabilities privately via GitHub Security Advisory.
  - **[Code of Conduct](./CODE_OF_CONDUCT.md)** — Contributor Covenant v2.1.
  - **[Contributing](./CONTRIBUTING.md)** — how to develop, test, and submit changes.
  - **[Code Owners](./.github/CODEOWNERS)** — per-stack review routing.
  ```

- [ ] **Step 3: Add the same block to `README_zh.md` in Chinese**

  ```markdown
  ## 社区

  - **[安全策略](./SECURITY.md)** — 通过 GitHub Security Advisory 私下报告漏洞。
  - **[行为准则](./CODE_OF_CONDUCT.md)** — Contributor Covenant v2.1。
  - **[贡献指南](./CONTRIBUTING.md)** — 开发、测试与提交变更。
  - **[代码所有者](./.github/CODEOWNERS)** — 各子栈的评审路由。
  ```

- [ ] **Step 4: Verify sync — both READMEs name all three files**

  Run: `rtk grep -c "SECURITY.md\|CODE_OF_CONDUCT.md\|CODEOWNERS" README.md`
  Run: `rtk grep -c "SECURITY.md\|CODE_OF_CONDUCT.md\|CODEOWNERS" README_zh.md`

  Expected: both return ≥ 3 (one hit per file).

- [ ] **Step 5: Commit**

  ```bash
  rtk git add README.md README_zh.md
  rtk git commit -m "docs(readme): link Security / CoC / CODEOWNERS from both READMEs"
  ```

### Task 10: PR #1 acceptance checks + push + open PR

**Files:** none modified.

- [ ] **Step 1: Run every Phase-1 acceptance check from the spec**

  ```bash
  # 1. Placeholder substitution
  rtk grep -n '<MAINTAINER_HANDLE>\|<MAINTAINER_CONTACT>' SECURITY.md CODE_OF_CONDUCT.md .github/CODEOWNERS
  # Expected: no matches

  # 2. Concurrency on the four new workflows
  for f in build-tauri desktop-tauri-logic agent-review review-layer2; do
    rtk grep -l 'concurrency:' ".github/workflows/${f}.yml"
  done
  # Expected: 4 file paths

  # 3. Stale claim fixed
  rtk grep 'not currently wired' CI_CD.md
  # Expected: no matches

  # 4. README sync
  rtk grep -c 'SECURITY\.md\|CODE_OF_CONDUCT\.md\|CODEOWNERS' README.md
  rtk grep -c 'SECURITY\.md\|CODE_OF_CONDUCT\.md\|CODEOWNERS' README_zh.md
  # Expected: ≥ 3 both
  ```

- [ ] **Step 2: Validate all touched YAML parses**

  ```bash
  for f in build-tauri desktop-tauri-logic agent-review review-layer2; do
    node -e "require('yaml').parse(require('fs').readFileSync('.github/workflows/${f}.yml', 'utf8'))" && echo "${f} OK"
  done
  # Expected: 4 OK lines
  ```

- [ ] **Step 3: Push the branch**

  ```bash
  rtk git push -u origin ci/repo-hygiene-phase-1-community-and-concurrency
  ```

- [ ] **Step 4: Open the PR**

  ```bash
  gh pr create --title "ci: repo hygiene phase 1 — community files + concurrency" --body "$(cat <<'EOF'
  ## Summary

  - Adds `SECURITY.md`, `CODE_OF_CONDUCT.md` (Contributor Covenant v2.1), and `.github/CODEOWNERS`.
  - Adds `concurrency:` groups to the four workflows that lacked them: `build-tauri.yml`, `desktop-tauri-logic.yml`, `agent-review.yml`, `review-layer2.yml`. Uses the conditional `workflow_call`-safe form (matching `go-ci.yml:22`) for the two reusable workflows so `release.yml` invocations are not cancelled by `ci.yml` invocations.
  - Corrects `CI_CD.md`'s stale claim that `desktop-tauri-logic.yml` is not wired into `ci.yml` (it has been a hard `needs:` dep of `build-tauri` for some time) and documents the repo's existing concurrency baseline.
  - Links the three community files from both `README.md` and `README_zh.md`.

  Spec: `docs/superpowers/specs/2026-04-22-repo-hygiene-and-ci-hardening-design.md`

  ## Test plan

  - [ ] CI green on this PR.
  - [ ] `gh api repos/Arxtect/AgentForge/community/profile --jq .health_percentage` ≥ 85.
  - [ ] Push a second commit to this branch → confirm the older `build-tauri` / `desktop-tauri-logic` / `agent-review` run shows `cancelled` in the Actions UI.
  - [ ] `git grep 'not currently wired' CI_CD.md` returns nothing.

  🤖 Generated with [Claude Code](https://claude.com/claude-code)
  EOF
  )"
  ```

- [ ] **Step 5: Monitor for GitHub CODEOWNERS parse errors**

  After push, open PR page; if GitHub comments that CODEOWNERS has a parse error, fix the handle format and amend.

---

## PR #2 — `deploy.yml` archival + Bun cache + verify scripts + CONTRIBUTING

**Branch:** `ci/repo-hygiene-phase-2-cleanup-and-ergonomics`

(Branch from `master` after PR #1 is merged, to keep the `CI_CD.md` edits ordered without conflicts.)

### Task 11: Archive `deploy.yml`

**Files:**
- Move: `.github/workflows/deploy.yml` → `docs/ci-examples/deploy.example.yml`

- [ ] **Step 1: Create target directory**

  ```bash
  mkdir -p docs/ci-examples
  ```

- [ ] **Step 2: Move the file**

  ```bash
  rtk git mv .github/workflows/deploy.yml docs/ci-examples/deploy.example.yml
  ```

- [ ] **Step 3: Prepend the docblock**

  Open `docs/ci-examples/deploy.example.yml` and insert before line 1 (before `name: Deploy`):

  ```yaml
  # ⚠️ Reference example — NOT an active workflow.
  # This file was archived from .github/workflows/deploy.yml on 2026-04-22.
  # To reactivate: copy back to .github/workflows/deploy.yml, set
  # DEPLOY_ENABLED to true, and configure the VERCEL_* secrets.
  # See CI_CD.md §"Disabled examples" for context.
  ```

- [ ] **Step 4: Grep for lingering references**

  ```bash
  rtk grep -l 'deploy\.yml' .github scripts docs
  ```

  Expected output: only `docs/ci-examples/deploy.example.yml` (itself) and possibly `CI_CD.md` (which we'll fix in Task 12). If `ci.yml` or `release.yml` still references `deploy.yml`, that's a surprise — investigate before continuing.

- [ ] **Step 5: Commit**

  ```bash
  rtk git add docs/ci-examples/deploy.example.yml .github/workflows/deploy.yml
  rtk git commit -m "ci(deploy): archive disabled workflow to docs/ci-examples/"
  ```

### Task 12: Update `CI_CD.md` — remove deploy section + add "Disabled examples" pointer

**Files:**
- Modify: `CI_CD.md`

- [ ] **Step 1: Remove the entire "Deploy Flow" section**

  Run: `rtk grep -n "^## Deploy Flow\|^## Review Automation" CI_CD.md`

  Delete lines from `## Deploy Flow` up to (but not including) `## Review Automation`.

- [ ] **Step 2: Remove `deploy.yml` from the workflow inventory list**

  Run: `rtk grep -n "deploy\.yml" CI_CD.md`

  Remove each bullet that still references it (e.g. the inventory entry under "Core Delivery Workflows").

- [ ] **Step 3: Add a "Disabled examples" section near the end (before "Operational Notes")**

  ```markdown
  ## Disabled examples

  `docs/ci-examples/deploy.example.yml` — a preserved reference copy of
  the old Vercel-deploy workflow. Not loaded by GitHub Actions. To
  reactivate it, copy back to `.github/workflows/deploy.yml`, set
  `DEPLOY_ENABLED` to `true`, and configure the `VERCEL_*` secrets.
  ```

- [ ] **Step 4: Verify**

  ```bash
  rtk grep 'deploy\.yml' CI_CD.md
  ```

  Expected: only mentions under "Disabled examples" (one hit).

- [ ] **Step 5: Commit**

  ```bash
  rtk git add CI_CD.md
  rtk git commit -m "docs(ci): remove deploy.yml section, add Disabled examples pointer"
  ```

### Task 13: Add Bun install cache to `ci.yml` `bridge-typecheck` job

**Files:**
- Modify: `.github/workflows/ci.yml` (around lines 65-75)

- [ ] **Step 1: Read current job definition**

  Read `.github/workflows/ci.yml` lines 64-76.

- [ ] **Step 2: Insert cache step between `setup-bun` and `bun install`**

  Replace the step block:

  ```yaml
  steps:
    - uses: actions/checkout@v6
    - uses: oven-sh/setup-bun@v2
    - run: bun install --frozen-lockfile
    - run: bun run typecheck
  ```

  with:

  ```yaml
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

- [ ] **Step 3: Verify YAML parses**

  ```bash
  node -e "require('yaml').parse(require('fs').readFileSync('.github/workflows/ci.yml', 'utf8'))"
  ```

  Expected: exit 0.

- [ ] **Step 4: Commit**

  ```bash
  rtk git add .github/workflows/ci.yml
  rtk git commit -m "ci(bridge-typecheck): cache Bun install (src-bridge/node_modules + ~/.bun/install/cache)"
  ```

### Task 14: Add `pnpm verify*` scripts to `package.json`

**Files:**
- Modify: `package.json`

- [ ] **Step 1: Read current scripts block**

  Read `package.json` scripts section.

- [ ] **Step 2: Insert the six new keys**

  Add these entries to the `scripts` object (recommended placement: after `test:tauri:coverage` and before `build:backend`):

  ```json
  "verify": "pnpm exec tsc --noEmit && pnpm test && pnpm build && pnpm lint",
  "verify:frontend": "pnpm verify",
  "verify:strict": "pnpm exec tsc --noEmit && pnpm test && pnpm build && pnpm lint --max-warnings=0",
  "verify:go": "cd src-go && go vet ./... && go test ./...",
  "verify:bridge": "cd src-bridge && bun run typecheck && bun test",
  "verify:all": "pnpm verify && pnpm verify:go && pnpm verify:bridge",
  ```

  **Ordering matters:** lint is LAST so real failures (typecheck/test/build) surface before the known-noisy lint step. This matches spec §3.1.

- [ ] **Step 3: Verify all six scripts are parseable**

  ```bash
  node -e "const p = require('./package.json'); for (const k of ['verify','verify:frontend','verify:strict','verify:go','verify:bridge','verify:all']) { if (!p.scripts[k]) { console.error('missing: ' + k); process.exit(1) } else console.log('OK: ' + k) }"
  ```

  Expected: 6 OK lines.

- [ ] **Step 4: Smoke-test `pnpm verify:frontend` locally (typecheck + test + build)**

  ```bash
  rtk pnpm exec tsc --noEmit
  ```

  Expected: exits 0. (If it doesn't, the failure is pre-existing and unrelated to this task — flag for the user but don't abandon the task.)

- [ ] **Step 5: Commit**

  ```bash
  rtk git add package.json
  rtk git commit -m "chore(scripts): add pnpm verify aggregate scripts for local preflight"
  ```

### Task 15: Add "Before pushing" section to `CONTRIBUTING.md`

**Files:**
- Modify: `CONTRIBUTING.md`

- [ ] **Step 1: Find insertion point**

  Run: `rtk grep -n "^## " CONTRIBUTING.md`

  Find the "Local Quality Gates" section. Insert the new subsection after it (before "Testing Expectations").

- [ ] **Step 2: Add the subsection**

  ```markdown
  ## Before Pushing

  Before opening a PR, run the aggregate verify script matching the surface
  you touched:

  ```bash
  # Frontend (typecheck + test + build + lint)
  pnpm verify

  # Strict variant — what the future CI_LINT_STRICT world looks like
  pnpm verify:strict

  # Go backend
  pnpm verify:go

  # TypeScript bridge (src-bridge)
  pnpm verify:bridge

  # All workspaces in one shot
  pnpm verify:all
  ```

  These scripts exist for local use only; CI continues to run its parallel
  job graph directly. For per-surface deep testing, see
  [TESTING.md](./TESTING.md).
  ```

- [ ] **Step 3: Verify**

  ```bash
  rtk grep -n "Before Pushing\|pnpm verify" CONTRIBUTING.md
  ```

  Expected: at least one `Before Pushing` heading and multiple `pnpm verify*` mentions.

- [ ] **Step 4: Commit**

  ```bash
  rtk git add CONTRIBUTING.md
  rtk git commit -m "docs(contributing): document pnpm verify scripts under Before Pushing"
  ```

### Task 16: PR #2 acceptance checks + push + open PR

**Files:** none modified.

- [ ] **Step 1: Run every Phase-2 acceptance check**

  ```bash
  # 1. deploy.yml gone from workflows
  ls .github/workflows/deploy* 2>/dev/null || echo "OK: deploy.yml removed"

  # 2. Archive preserved with docblock
  head -5 docs/ci-examples/deploy.example.yml
  # Expected: first line starts with "# ⚠️ Reference example"

  # 3. No lingering deploy.yml references
  rtk grep -l 'deploy\.yml' .github scripts docs | grep -v 'deploy\.example\.yml' | grep -v 'CI_CD\.md' || echo "OK: no stray references"

  # 4. Disabled examples section exists
  rtk grep -c 'Disabled examples' CI_CD.md
  # Expected: ≥ 1

  # 5. All six verify scripts present
  node -e "const s=require('./package.json').scripts; ['verify','verify:frontend','verify:strict','verify:go','verify:bridge','verify:all'].forEach(k=>console.log(k+': '+(s[k]?'OK':'MISSING')))"

  # 6. CONTRIBUTING updated
  rtk grep -n 'pnpm verify' CONTRIBUTING.md
  ```

- [ ] **Step 2: Validate ci.yml YAML still parses**

  ```bash
  node -e "require('yaml').parse(require('fs').readFileSync('.github/workflows/ci.yml', 'utf8'))"
  ```

- [ ] **Step 3: Push & open PR**

  ```bash
  rtk git push -u origin ci/repo-hygiene-phase-2-cleanup-and-ergonomics
  gh pr create --title "ci: repo hygiene phase 2 — deploy archival + Bun cache + verify scripts" --body "$(cat <<'EOF'
  ## Summary

  - Archives `deploy.yml` (`DEPLOY_ENABLED: false`, commented-out Vercel steps, not called from `ci.yml`) to `docs/ci-examples/deploy.example.yml` with a reactivation docblock.
  - Adds `actions/cache` to `ci.yml`'s inline `bridge-typecheck` job — the only uncached install path in the pipeline.
  - Adds six `pnpm verify*` scripts for local preflight. CI does not call them.
  - Documents the scripts under a new "Before Pushing" section in `CONTRIBUTING.md`.
  - Updates `CI_CD.md`: removes the Deploy Flow section and adds a "Disabled examples" pointer to the archived workflow.

  Spec: `docs/superpowers/specs/2026-04-22-repo-hygiene-and-ci-hardening-design.md`

  ## Test plan

  - [ ] CI green on this PR.
  - [ ] Second push to this branch → the `bridge-typecheck` job shows a `Cache restored from key: Linux-bun-…` line.
  - [ ] `ls .github/workflows/deploy*` returns nothing.
  - [ ] `pnpm verify` runs locally on a clean `master` checkout (lint step may still be noisy — documented).

  🤖 Generated with [Claude Code](https://claude.com/claude-code)
  EOF
  )"
  ```

---

## PR #3 — SARIF / summaries / strict switch

**Branch:** `ci/repo-hygiene-phase-3-sarif-and-summaries`

(Branch from `master` after PR #2 is merged.)

### Task 17: TDD `scripts/ci/summarize-lint.js`

**Files:**
- Create: `scripts/ci/summarize-lint.js`
- Create: `scripts/ci/summarize-lint.test.ts`

- [ ] **Step 1: Create directory**

  ```bash
  mkdir -p scripts/ci
  ```

- [ ] **Step 2: Write the failing test first**

  Create `scripts/ci/summarize-lint.test.ts`:

  ```ts
  /** @jest-environment node */

  import { execFileSync } from "node:child_process";
  import * as fs from "node:fs";
  import * as os from "node:os";
  import * as path from "node:path";

  const SCRIPT = path.join(process.cwd(), "scripts/ci/summarize-lint.js");

  function run(sarifPath: string): { stdout: string; status: number } {
    try {
      const stdout = execFileSync("node", [SCRIPT, sarifPath], {
        encoding: "utf8",
        stdio: ["ignore", "pipe", "pipe"],
      });
      return { stdout, status: 0 };
    } catch (err: unknown) {
      const e = err as { stdout?: Buffer; status?: number };
      return { stdout: e.stdout?.toString() ?? "", status: e.status ?? 1 };
    }
  }

  function writeTmp(name: string, content: string): string {
    const p = path.join(os.tmpdir(), `summarize-lint-${Date.now()}-${name}`);
    fs.writeFileSync(p, content);
    return p;
  }

  describe("summarize-lint", () => {
    test("empty SARIF emits a neutral success line and exits 0", () => {
      const sarif = JSON.stringify({ version: "2.1.0", runs: [{ results: [] }] });
      const { stdout, status } = run(writeTmp("empty.sarif", sarif));
      expect(status).toBe(0);
      expect(stdout).toMatch(/no lint (violations|issues)/i);
    });

    test("non-empty SARIF reports totals and exits 0", () => {
      const sarif = JSON.stringify({
        version: "2.1.0",
        runs: [
          {
            results: [
              { ruleId: "no-unused-vars", locations: [{ physicalLocation: { artifactLocation: { uri: "a.ts" } } }] },
              { ruleId: "no-unused-vars", locations: [{ physicalLocation: { artifactLocation: { uri: "a.ts" } } }] },
              { ruleId: "no-console", locations: [{ physicalLocation: { artifactLocation: { uri: "b.ts" } } }] },
            ],
          },
        ],
      });
      const { stdout, status } = run(writeTmp("hits.sarif", sarif));
      expect(status).toBe(0);
      expect(stdout).toMatch(/Total violations:\s*3/);
      expect(stdout).toMatch(/a\.ts/);
      expect(stdout).toMatch(/no-unused-vars/);
    });

    test("malformed input exits 0 with a fallback line (fail-safe)", () => {
      const { stdout, status } = run(writeTmp("bad.sarif", "not-json"));
      expect(status).toBe(0);
      expect(stdout).toMatch(/unable to parse|could not read/i);
    });

    test("missing file exits 0 with a fallback line", () => {
      const { stdout, status } = run("/nonexistent/path-to-nowhere.sarif");
      expect(status).toBe(0);
      expect(stdout).toMatch(/unable to parse|could not read/i);
    });
  });
  ```

- [ ] **Step 3: Run the test to confirm it fails**

  ```bash
  rtk pnpm exec jest scripts/ci/summarize-lint.test.ts
  ```

  Expected: FAIL — "Cannot find module" or "ENOENT" for `summarize-lint.js`.

- [ ] **Step 4: Write the minimal implementation**

  Create `scripts/ci/summarize-lint.js`:

  ```js
  #!/usr/bin/env node
  // Reads an ESLint SARIF file and prints a Markdown summary to stdout.
  // Fail-safe: any parse/IO error produces a fallback message and exit 0.

  "use strict";

  const fs = require("node:fs");

  function readSarif(filePath) {
    try {
      const raw = fs.readFileSync(filePath, "utf8");
      return JSON.parse(raw);
    } catch {
      return null;
    }
  }

  function collectResults(sarif) {
    if (!sarif || !Array.isArray(sarif.runs)) return [];
    const out = [];
    for (const run of sarif.runs) {
      if (!run || !Array.isArray(run.results)) continue;
      for (const r of run.results) out.push(r);
    }
    return out;
  }

  function countBy(items, keyFn) {
    const map = new Map();
    for (const it of items) {
      const k = keyFn(it);
      if (!k) continue;
      map.set(k, (map.get(k) ?? 0) + 1);
    }
    return [...map.entries()].sort((a, b) => b[1] - a[1]);
  }

  function locationUri(r) {
    const loc = r?.locations?.[0]?.physicalLocation?.artifactLocation?.uri;
    return typeof loc === "string" ? loc : null;
  }

  function main() {
    const [, , filePath] = process.argv;
    if (!filePath) {
      console.log("### ESLint\n\n_unable to parse: no file path argument_");
      return;
    }
    const sarif = readSarif(filePath);
    if (!sarif) {
      console.log("### ESLint\n\n_unable to parse SARIF (could not read or invalid JSON)_");
      return;
    }
    const results = collectResults(sarif);
    const total = results.length;
    if (total === 0) {
      console.log("### ESLint\n\n✅ no lint violations.");
      return;
    }
    const byFile = countBy(results, locationUri).slice(0, 5);
    const byRule = countBy(results, (r) => r.ruleId).slice(0, 5);

    const lines = [];
    lines.push("### ESLint");
    lines.push("");
    lines.push(`**Total violations:** ${total}`);
    lines.push("");
    if (byFile.length > 0) {
      lines.push("**Top files:**");
      lines.push("");
      for (const [file, count] of byFile) lines.push(`- \`${file}\` — ${count}`);
      lines.push("");
    }
    if (byRule.length > 0) {
      lines.push("**Top rules:**");
      lines.push("");
      for (const [rule, count] of byRule) lines.push(`- \`${rule}\` — ${count}`);
    }
    console.log(lines.join("\n"));
  }

  main();
  ```

- [ ] **Step 5: Run the tests to confirm they pass**

  ```bash
  rtk pnpm exec jest scripts/ci/summarize-lint.test.ts
  ```

  Expected: 4 passed, 0 failed.

- [ ] **Step 6: Commit**

  ```bash
  rtk git add scripts/ci/summarize-lint.js scripts/ci/summarize-lint.test.ts
  rtk git commit -m "ci(scripts): add summarize-lint.js + Jest suite with fail-safe fallbacks"
  ```

### Task 18: TDD `scripts/ci/summarize-audit.js`

**Files:**
- Create: `scripts/ci/summarize-audit.js`
- Create: `scripts/ci/summarize-audit.test.ts`

- [ ] **Step 1: Write the failing test**

  Create `scripts/ci/summarize-audit.test.ts`. Mirror the structure of `summarize-lint.test.ts` with these test cases:
  - **empty advisories → neutral success**: input `{ "advisories": {} }` → stdout matches `/no known vulnerabilities/i`.
  - **advisories present → severity counts**: input with three entries of different severities (low, high, critical) → stdout contains `critical: 1`, `high: 1`, `low: 1`.
  - **malformed JSON → fallback, exit 0**.
  - **missing file → fallback, exit 0**.

  The exact `pnpm audit --json` schema uses `{ advisories: { <id>: { severity, title, module_name, url }, ... }, metadata: {...} }`. Test with this shape.

- [ ] **Step 2: Run, confirm FAIL**

  ```bash
  rtk pnpm exec jest scripts/ci/summarize-audit.test.ts
  ```

- [ ] **Step 3: Write minimal implementation**

  Mirror `summarize-lint.js`. Key logic:
  - Read + parse JSON; fail-safe to `console.log("### pnpm audit\n\n_unable to parse…_")` and return.
  - If `advisories` is an empty object, print "✅ no known vulnerabilities.".
  - Otherwise group by `severity`, print `| Severity | Count |` Markdown table, plus top-5 advisories by severity-rank.

- [ ] **Step 4: Run tests to confirm PASS**

- [ ] **Step 5: Commit**

  ```bash
  rtk git add scripts/ci/summarize-audit.js scripts/ci/summarize-audit.test.ts
  rtk git commit -m "ci(scripts): add summarize-audit.js for pnpm audit JSON output"
  ```

### Task 19: TDD `scripts/ci/summarize-outdated.js`

**Files:**
- Create: `scripts/ci/summarize-outdated.js`
- Create: `scripts/ci/summarize-outdated.test.ts`

- [ ] **Step 1: Write the failing test**

  `pnpm outdated --format json` emits: `{ "<package>": { current, wanted, latest, dependencyType, isDeprecated }, ... }`.

  Test cases:
  - **empty object → "✅ all dependencies up to date."**
  - **non-empty → Markdown table with Name / Current / Wanted / Latest / Type columns**.
  - **malformed JSON → fallback, exit 0**.
  - **missing file → fallback, exit 0**.

- [ ] **Step 2: Run, confirm FAIL**

- [ ] **Step 3: Write minimal implementation**

  Mirror the previous two. Emit Markdown table with one row per package, cap at 20 rows with a "…and N more" footer line if exceeded.

- [ ] **Step 4: Run tests to confirm PASS**

- [ ] **Step 5: Commit**

  ```bash
  rtk git add scripts/ci/summarize-outdated.js scripts/ci/summarize-outdated.test.ts
  rtk git commit -m "ci(scripts): add summarize-outdated.js for pnpm outdated JSON output"
  ```

### Task 20: Rewrite `quality.yml` — permissions + SARIF + summaries + strict switch

**Files:**
- Modify: `.github/workflows/quality.yml`

- [ ] **Step 1: Add/verify the SARIF formatter is installed**

  The `@microsoft/eslint-formatter-sarif` is a dev-dep. Check:

  ```bash
  rtk grep '"@microsoft/eslint-formatter-sarif"' package.json
  ```

  If absent, add it:

  ```bash
  rtk pnpm add -D @microsoft/eslint-formatter-sarif
  ```

  Verify `package.json` and `pnpm-lock.yaml` are updated, then stage them (they'll be committed with the workflow change).

- [ ] **Step 2: Rewrite `quality.yml` — add `permissions:` block and replace lint/audit/outdated steps**

  Current job has no `permissions:` block at the job level. Add one. Then replace the three `continue-on-error: true` steps with the new SARIF + summary + strict-switch pattern.

  Read the current file first (`rtk read .github/workflows/quality.yml`). Target state:

  ```yaml
  name: Code Quality

  on:
    workflow_call:
    workflow_dispatch:

  concurrency:
    group: quality-${{ github.ref }}
    cancel-in-progress: true

  jobs:
    quality:
      name: Code Quality & Security
      runs-on: ubuntu-latest
      permissions:
        contents: read
        pull-requests: write
        checks: write
        security-events: write  # required for SARIF upload

      steps:
        - name: Checkout code
          uses: actions/checkout@v6

        - name: Display CI/CD Configuration
          # ... (keep existing summary display block unchanged) ...

        - name: Setup pnpm
          uses: pnpm/action-setup@v6
          with:
            version: 10

        - name: Setup Node.js
          uses: actions/setup-node@v6
          with:
            node-version: 22.x
            cache: "pnpm"

        - name: Install dependencies
          run: pnpm install --frozen-lockfile

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

        # Strict gate: separate step, NO continue-on-error.
        # When repo variable CI_LINT_STRICT=true, failures here fail the job.
        - name: ESLint (strict, gated by CI_LINT_STRICT)
          if: ${{ vars.CI_LINT_STRICT == 'true' }}
          run: pnpm lint

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

  Use Edit operations to keep the existing "Display CI/CD Configuration" block and the `Setup pnpm` / `Setup Node.js` / `Install dependencies` blocks unchanged. Only the three `continue-on-error` steps (original lines 61-74) are being rewritten, plus the new `permissions:` block and new strict step.

- [ ] **Step 3: Verify YAML parses**

  ```bash
  node -e "require('yaml').parse(require('fs').readFileSync('.github/workflows/quality.yml', 'utf8'))"
  ```

- [ ] **Step 4: Verify the permission is present at job scope**

  ```bash
  rtk grep -A5 "permissions:" .github/workflows/quality.yml | rtk grep "security-events"
  ```

  Expected: at least one hit.

- [ ] **Step 5: Dry-run the summary scripts locally**

  ```bash
  echo '{"version":"2.1.0","runs":[{"results":[]}]}' > /tmp/eslint.sarif
  node scripts/ci/summarize-lint.js /tmp/eslint.sarif
  echo '{"advisories":{}}' > /tmp/audit.json
  node scripts/ci/summarize-audit.js /tmp/audit.json
  echo '{}' > /tmp/outdated.json
  node scripts/ci/summarize-outdated.js /tmp/outdated.json
  ```

  Expected: each prints a neutral-success Markdown line; all exit 0.

- [ ] **Step 6: Commit**

  ```bash
  rtk git add .github/workflows/quality.yml package.json pnpm-lock.yaml
  rtk git commit -m "ci(quality): SARIF upload + job summaries + CI_LINT_STRICT switch"
  ```

### Task 21: Update `CI_CD.md` SARIF paragraph (PR #3's share)

**Files:**
- Modify: `CI_CD.md`

- [ ] **Step 1: Find the "Quality" section**

  Run: `rtk grep -n "^### 1\. Quality" CI_CD.md`

- [ ] **Step 2: Append a paragraph to that section**

  ```markdown
  #### Visibility layer

  The lint, audit, and outdated steps are non-blocking by default
  (`continue-on-error: true`) but are now reported via:

  - **ESLint SARIF upload** — violations appear as PR-level annotations on
    the offending line, using `github/codeql-action/upload-sarif@v3`.
  - **Job summaries** — `scripts/ci/summarize-{lint,audit,outdated}.js`
    render a Markdown table into `$GITHUB_STEP_SUMMARY` on every run.
  - **`CI_LINT_STRICT` switch** — set the repository variable
    `CI_LINT_STRICT=true` (Actions → Variables) and the strict gate step
    will fail the job on any lint violation. The default is off.

  The strict step is a SEPARATE step without `continue-on-error`, so the
  switch is genuinely binding when flipped.
  ```

- [ ] **Step 3: Commit**

  ```bash
  rtk git add CI_CD.md
  rtk git commit -m "docs(ci): document SARIF / summaries / CI_LINT_STRICT in quality section"
  ```

### Task 22: PR #3 acceptance checks + push + open PR

**Files:** none modified.

- [ ] **Step 1: Run every Phase-3 acceptance check**

  ```bash
  # 1. security-events: write present
  rtk grep -A10 "permissions:" .github/workflows/quality.yml | rtk grep "security-events: write"

  # 2. Strict step exists and has no continue-on-error
  rtk grep -B1 -A3 "CI_LINT_STRICT" .github/workflows/quality.yml
  # Verify the block does NOT contain "continue-on-error"

  # 3. All three scripts + tests exist
  ls scripts/ci/summarize-{lint,audit,outdated}.js scripts/ci/summarize-{lint,audit,outdated}.test.ts

  # 4. All three test suites pass
  rtk pnpm exec jest scripts/ci/

  # 5. Malformed-input fail-safe
  node scripts/ci/summarize-lint.js /dev/null
  node scripts/ci/summarize-audit.js /dev/null
  node scripts/ci/summarize-outdated.js /dev/null
  echo "All three should have printed a fallback line with exit 0; verify with $?"

  # 6. CI_CD.md updated
  rtk grep -n "SARIF\|step summary\|CI_LINT_STRICT" CI_CD.md
  ```

- [ ] **Step 2: Push & open PR**

  ```bash
  rtk git push -u origin ci/repo-hygiene-phase-3-sarif-and-summaries
  gh pr create --title "ci: repo hygiene phase 3 — SARIF annotations + job summaries + strict switch" --body "$(cat <<'EOF'
  ## Summary

  - Rewrites `quality.yml`'s lint / audit / outdated steps to produce SARIF (for lint) and JSON-parsed Markdown summaries (for all three). ESLint violations now appear as line-level PR annotations.
  - Adds `permissions: security-events: write` at job scope in `quality.yml` (required for SARIF upload, minimal blast radius).
  - Plants the `CI_LINT_STRICT` repository-variable switch. Default is off — behavior is unchanged. When flipped to `true`, a separate strict step (no `continue-on-error`) fails the job on lint violations. A follow-up spec will cover the flip + lint debt cleanup.
  - Adds three small Node scripts under `scripts/ci/` with Jest `.test.ts` siblings that explicitly cover empty / non-empty / malformed / missing-file cases. All scripts fail-safe to exit 0 to avoid summary bugs turning green CI red.
  - Adds `@microsoft/eslint-formatter-sarif` as a devDependency.
  - Documents the visibility layer + switch in `CI_CD.md`'s Quality section.

  Spec: `docs/superpowers/specs/2026-04-22-repo-hygiene-and-ci-hardening-design.md`

  ## Test plan

  - [ ] CI green on this PR.
  - [ ] Introduce an intentional lint violation in a draft follow-up PR → confirm it renders as a line-level annotation in the PR Files Changed tab.
  - [ ] View the `quality` job's Summary tab on a CI run → confirm three Markdown tables (lint / audit / outdated).
  - [ ] Set repo variable `CI_LINT_STRICT=true` in a test branch → confirm the "ESLint (strict, gated)" step runs AND fails the job on a violation.
  - [ ] Unset the variable → confirm the step is skipped and the job succeeds.

  🤖 Generated with [Claude Code](https://claude.com/claude-code)
  EOF
  )"
  ```

---

## Post-merge: update MEMORY

After all three PRs are merged, the workflow baseline has changed meaningfully. Update auto-memory:

- [ ] **Step 1: Update or create a project memory entry noting:**
  - `CI_LINT_STRICT` switch exists and is off by default
  - Community profile is now ≥ 85%
  - Follow-up spec (flipping `CI_LINT_STRICT`) is the next piece of planned work in this area

  Write to `C:\Users\qwdma\.claude\projects\D--Project-AgentForge\memory\project_ci_strict_switch.md` and add a one-line index entry to `MEMORY.md`.

---

## Global reminders

- **Frequent commits.** Every task has a commit. Don't batch.
- **DRY / YAGNI.** Don't add flags / variables / scripts that aren't in this plan.
- **TDD for the three scripts.** Test first, fail, implement, pass. The spec explicitly requires exit-0 fail-safe tests.
- **No changes to the files listed under the spec's "Non-Goals".** If a touched file neighbors a Non-Goal concern (e.g. editing `quality.yml` tempts you to also change `continue-on-error: false`), leave it.
- **Substitute placeholders before push.** The Task 10 acceptance check greps for any remaining `<MAINTAINER_HANDLE>` / `<MAINTAINER_CONTACT>` strings — if it finds one, the PR is not ready.
