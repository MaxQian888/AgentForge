# TS Bridge Hotfix Batch 1 — iflow Removal + Review Plugin-Only — Spec

- **ID**: `2026-04-19-ts-bridge-batch1-iflow-review-design`
- **Author**: Claude (brainstorming) + Max Qian
- **Status**: draft-1
- **Scope**: `src-bridge/`, `src-go/internal/service/coding_agent_backend_profiles.json`, a bounded set of frontend/IM-bridge/i18n touch points, and current (non-archived) OpenSpec specs that reference iflow
- **Evidence snapshot date**: 2026-04-19
- **Source audit**: live read of `src-bridge/` on 2026-04-19; confirmed touch points by grep

---

## 1. Background

Audit of `src-bridge/` on 2026-04-19 surfaced two problems that are small enough to land as a single "hotfix" batch ahead of the larger file-split refactor (tracked as Batch 2):

1. **iflow runtime is sunset.** `src-bridge/src/runtime/backend-profiles.ts:103-106` declares `stage: "sunsetting"` with `sunset_at: "2026-04-17T00:00:00+08:00"`. The sunset date has passed. The runtime is retained only as dead code plus a misleading "sunsetting" label. Decision: delete outright. Not migrate, not sunset the label, not keep for backward compatibility — the project is in internal-testing stage where breaking changes are explicitly permitted.

2. **Deep Review built-in reviewers are regex theater.** `src-bridge/src/review/orchestrator.ts:41-168` implements four "dimensions" (`logic / security / performance / compliance`) as 5–10-line regex matches (`TODO|FIXME`, `eval(`, `SELECT *`, `console.log`). This is not a code review. The orchestrator already supports **MCP review plugins** (`executeMcpReviewPlugin`, lines 237–261) — that is the correct extension point for real review, whether AI-backed or rule-backed. The built-ins add noise, not value, and lie about what Deep Review does.

## 2. Goals and non-goals

### Goals
1. `iflow` is removed from every live code path, test, configuration, UI option, i18n bundle, and current OpenSpec spec. A user configuring a new agent cannot select it; a user with a legacy `backend_runtime="iflow"` record will see it reject at load time as "unknown runtime".
2. Deep Review becomes strictly plugin-driven. The four built-in regex reviewers, the `DEFAULT_DIMENSIONS` constant, the `reviewers` record, and the `buildFinding` helper are deleted. The orchestrator only runs `request.review_plugins`. With zero plugins the response is empty and the summary field explicitly says so.
3. `ReviewDimension` stops being a fixed literal union and becomes `string`; `DeepReviewRequest.dimensions` is deleted — plugins declare their own dimensions.
4. The project still builds, typechecks, lints, and passes tests on both Go and Bridge after each commit.

### Non-goals (explicitly out of this batch)
- `server.ts` (1917 LoC) and `runtime/registry.ts` (1939 LoC) splits — Batch 2.
- CLI runtime high-level operation stubs (fork/rollback/revert/... for cursor/gemini/qoder) — separate follow-up.
- `cost/calculator.ts` model-name hardcoding — separate follow-up.
- `handlers/opencode-runtime.ts` possible async-without-await leak — separate follow-up.
- Any LLM-backed built-in reviewer — real review logic is a plugin concern, not a bridge concern.
- Frontend Review UI adaptation to plugin-driven dimensions — separate follow-up task.
- Data migration for DB rows with `backend_runtime="iflow"`. They will fail at load with an "unknown runtime" error; operator updates them manually. Project is in internal-testing stage.
- Changes to archived OpenSpec changes (`openspec/changes/archive/**`), archived docs under `docs/dev/specs/` / `docs/dev/plans/` / `docs/research/` / `docs/architecture/`. Historical records are left intact.

## 3. Design — iflow Removal

### 3.1 Source of truth
`src-go/internal/service/coding_agent_backend_profiles.json` is the authoritative profile registry; `src-bridge/src/runtime/backend-profiles.ts:111-119` loads it at import time. The `iflow` entry is deleted from the JSON, and any inline fallback overlay in `backend-profiles.ts` is deleted as well.

### 3.2 Code touch list
All files below are modified in a single commit so the tree never sits in a half-deleted state:

**Bridge (TypeScript / Bun)**
- `src-bridge/src/runtime/backend-profiles.ts` — delete the `iflow` overlay block (lines ~80-108) and any references in the loader.
- `src-bridge/src/runtime/registry.ts` — delete the `iflow` case/branch in adapter selection; update any `switch (runtime)` default branches to not name iflow.
- `src-bridge/src/runtime/registry.test.ts` — drop iflow cases.
- `src-bridge/src/handlers/command-runtime.ts` — delete the iflow branch in the command-runtime dispatcher.
- `src-bridge/src/server.ts`, `src-bridge/src/server.test.ts` — remove any iflow-specific route, fixture, or expectation.
- `src-bridge/src/schemas.ts`, `src-bridge/src/schemas.test.ts` — remove `iflow` from enum unions; update test coverage of the enum.
- `src-bridge/src/types.ts` — remove `iflow` from type unions.

**Go orchestrator**
- `src-go/internal/service/coding_agent_backend_profiles.json` — delete iflow entry.
- `src-go/internal/service/coding_agent_test.go`, `src-go/internal/service/cost_query_service_test.go`, `src-go/internal/handler/project_handler_test.go` — remove iflow cases.

**IM Bridge (Go)**
- `src-im-bridge/commands/login.go`, `src-im-bridge/commands/catalog.go` — remove iflow from command registrations and switch statements.

**Frontend (Next.js)**
- `messages/zh-CN/settings.json`, `messages/en/settings.json` — delete iflow i18n keys.
- `app/(dashboard)/settings/page.tsx`, `app/(dashboard)/settings/_components/settings-sidebar.tsx`, `app/(dashboard)/settings/_components/section-runtime-detail.tsx` — remove iflow from option lists, sidebar entries, and detail branches.
- `lib/settings/project-settings-workspace.test.ts` — drop iflow assertions.
- `components/docs/live-blocks/insertion-dialogs.tsx` — remove iflow reference (likely documentation / dialog label).

**Docs and live specs**
- `CLAUDE.md` — delete the iflow mention in the runtime adapters paragraph.
- `README.md` — delete any iflow mention.
- `openspec/specs/cli-agent-runtime-adapters/spec.md`
- `openspec/specs/coding-agent-provider-management/spec.md`
- `openspec/specs/bridge-agent-runtime-registry/spec.md`
- `openspec/specs/agent-sdk-bridge-runtime/spec.md`

Each OpenSpec file is edited to drop `iflow` from any enumerated runtime list, capability matrix, or normative clause — **without** rewriting history or touching the archive.

### 3.3 Files explicitly left alone
- `openspec/changes/archive/**` — historical change records.
- `docs/dev/specs/2026-04-16-*`, `docs/dev/plans/2026-04-16-*` — archived design artifacts.
- `docs/architecture/cc-connect-reuse-guide.md`, `docs/research/market-research.md`, `docs/research/research-report.md`, `docs/guides/role-yaml-reference.md` — non-normative / research. A separate doc-sweep pass can revisit these later.

### 3.4 Data policy
No database migration. Existing `backend_runtime="iflow"` rows will hit the registry's unknown-runtime error path at load. Operators update them manually. This is acceptable under the project's current API-stability stage.

### 3.5 Commit
`refactor(bridge): remove iflow runtime (sunset 2026-04-17)`

## 4. Design — Review Orchestrator Plugin-Only

### 4.1 Code deletions in `src-bridge/src/review/orchestrator.ts`
- `reviewLogic` (41-74)
- `reviewSecurity` (76-109)
- `reviewPerformance` (111-144)
- `reviewCompliance` (146-168)
- `DEFAULT_DIMENSIONS` (13-18)
- `reviewers` record (170-175)
- `buildFinding` helper (27-39) — only used by the four built-ins.

### 4.2 New orchestrator behavior
`createDeepReviewOrchestrator` returns a function that:
1. Reads `request.review_plugins ?? []`.
2. Runs every plugin through `executeReviewPlugin` with `Promise.allSettled` (unchanged from today).
3. Wraps plugin failures into `ReviewExecutionResult` entries with `status: "failed"` (unchanged).
4. Feeds the list to `aggregateReviewResults` (unchanged).
5. If the plugin list is empty, the orchestrator does not invoke the aggregator with an empty array on autopilot — the aggregator is still called, and the response `summary` is set to `"No review plugins configured."` so the caller gets a clear signal rather than a silent empty object. (Aggregator must be verified to handle empty input correctly; if not, that is fixed here.)

### 4.3 Type and schema changes
- `src-bridge/src/review/types.ts`:
  - `ReviewDimension` changes from `"logic" | "security" | "performance" | "compliance"` to `string`.
  - `DeepReviewRequest.dimensions?: ReviewDimension[]` field is **removed**.
  - `ReviewExecutionResult.dimension: ReviewDimension` stays, but is now `string` — set by the plugin's own declared dimension or falls back to its `plugin_id` (existing fallback logic at orchestrator.ts:212-217 already covers this).
- `src-bridge/src/schemas.ts`:
  - `DeepReviewRequestSchema` drops the `dimensions` field.
  - Any Zod enum over the four literals becomes `z.string()` or is removed entirely.
- `src-bridge/src/schemas.test.ts`, `src-bridge/src/review/types.test.ts` — updated to match.

### 4.4 Test rewrite: `src-bridge/src/review/orchestrator.test.ts`
Existing tests that call `orchestrateDeepReview` with `dimensions: ["logic"]` (or other literals) and assert built-in regex outputs are removed. The replacement set covers exactly three cases:
1. **No plugins** → empty `dimension_results`, empty `findings`, summary = `"No review plugins configured."`
2. **One successful plugin** → plugin result becomes a single `dimension_results` entry with `source_type: "plugin"`; findings flow through the aggregator.
3. **One failing plugin** → entry with `status: "failed"` and an `error` string; does not throw; other plugins in the same request still succeed.

### 4.5 `src-bridge/src/review/aggregator.test.ts`
Already uses arbitrary string dimensions; verify tests still pass after the literal union → string change. No logic change expected.

### 4.6 Commit
`refactor(bridge/review): drop regex built-ins; Deep Review is plugin-only`

## 5. Delivery plan

### 5.1 Commit sequence (same branch, linear)
1. iflow removal commit (§3.5) — touches both `src-bridge`, `src-go`, `src-im-bridge`, frontend, docs, live OpenSpec specs.
2. Review orchestrator commit (§4.6) — `src-bridge/src/review/**` and its `schemas.ts` / `types.ts` neighbors.

Two commits are preferred over one so the commits remain focused, cherry-pickable, and individually revertable.

### 5.2 Verification gate after each commit
Run in this order; do not advance to the next commit if any step fails:
- `pnpm lint`
- `pnpm exec tsc --noEmit`
- `cd src-bridge && bun test`
- `cd src-go && go test ./...` (required after commit 1; commit 2 does not touch Go but running is cheap)

Snapshot updates (`bun test --update-snapshots`) are permitted and committed inside the same commit that caused the change.

### 5.3 Risk and mitigation
| Risk | Likelihood | Mitigation |
|---|---|---|
| DB has rows with `backend_runtime="iflow"` | Low (internal testing) | Accepted; "unknown runtime" load error is the signal for the operator |
| Frontend Review UI hard-codes the four dimensions | Medium | Flagged as a follow-up task; not in batch 1 scope |
| OpenSpec validation job fails when iflow is removed from a live spec | Low | If it triggers, adjust the spec's normative language in the same commit |
| Snapshot test churn after iflow removal | Medium | Update snapshots in-commit; review diffs before committing |
| A stray iflow reference survives under a case fold we missed | Low | Grep used `iflow|iFlow|IFLOW`; the final verification runs a fresh grep for these three spellings and fails the batch if any hit lands in a live (non-archived) file |

## 6. Follow-up tasks (not in this batch)

Registered here so they are not lost:
- [ ] Frontend Review UI: adapt to plugin-driven `dimension: string` (was fixed union of four labels).
- [ ] CLI runtime high-level operations (fork/rollback/revert/...) are stubs for `cursor / gemini / qoder`. Decide: implement, document as unsupported, or remove the surface area.
- [ ] `src-bridge/src/cost/calculator.ts:10` — hardcoded `"claude-sonnet-4"` model name; should track `ExecuteRequest.model`.
- [ ] `src-bridge/src/handlers/opencode-runtime.ts:78` — verify `sendPromptAsync` is awaited everywhere it is reachable; fix if not.
- [ ] Documentation sweep for stale iflow mentions in non-normative docs (`docs/architecture/`, `docs/research/`, `docs/guides/role-yaml-reference.md`).
- [ ] Batch 2: split `src-bridge/src/server.ts` (1917 LoC) and `src-bridge/src/runtime/registry.ts` (1939 LoC) by domain.

## 7. Acceptance criteria

The batch is complete when **all** of the following hold:
1. `grep -r -iE 'iflow' --include='*.ts' --include='*.tsx' --include='*.go' --include='*.json' --include='*.md' src-bridge src-go src-im-bridge app components lib messages CLAUDE.md README.md openspec/specs` returns **zero** matches. Matches inside `openspec/changes/archive/**` and `docs/dev/{specs,plans}/**` are ignored (historical).
2. `src-bridge/src/review/orchestrator.ts` contains no function named `reviewLogic`, `reviewSecurity`, `reviewPerformance`, `reviewCompliance`, `buildFinding`, and no `DEFAULT_DIMENSIONS` / `reviewers` record.
3. `DeepReviewRequest` has no `dimensions` field.
4. `ReviewDimension` is `string` (or the type alias is removed entirely in favor of `string`).
5. `pnpm lint && pnpm exec tsc --noEmit && (cd src-bridge && bun test) && (cd src-go && go test ./...)` is green.
6. An integration smoke — POST a Deep Review request with zero `review_plugins` — returns a well-formed response with an empty findings array and a summary explicitly stating no plugins were configured.
