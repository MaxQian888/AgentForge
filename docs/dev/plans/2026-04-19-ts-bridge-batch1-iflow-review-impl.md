# TS Bridge Hotfix Batch 1 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land two linear commits on `feat/employee-trigger-foundation` that (1) delete the sunset `iflow` runtime everywhere it is still live and (2) strip the four regex-based built-in reviewers from Deep Review so it is strictly MCP-plugin-driven.

**Architecture:** No new code. Pure removal + one contract change (`DeepReviewRequest.dimensions` field removed, `ReviewDimension` union → `string`). Source of truth for runtime profiles is `src-go/internal/service/coding_agent_backend_profiles.json`; the bridge loads it at startup and overlays CLI launch / lifecycle metadata on top. Deleting iflow requires editing both the JSON and every consumer that case-splits on the runtime string. Review change is contained to `src-bridge/src/review/`.

**Tech Stack:** Bun + TypeScript (bridge), Go 1.x (orchestrator + IM bridge), Next.js 16 + React 19 + TypeScript (frontend), pnpm.

**Spec:** `docs/dev/specs/2026-04-19-ts-bridge-batch1-iflow-review-design.md`

**Branch:** `feat/employee-trigger-foundation` (already checked out). Three unrelated files are modified in the working tree (`src-go/internal/repository/workflow_pending_review_repo.go`, `src-go/internal/server/routes.go`, `src-go/internal/service/im_action_execution.go`) — **do not touch or stage them**. They belong to another stream of work.

---

## File Structure

### Phase 1 — iflow removal (commit 1)

**Delete / edit in `src-bridge/` (TypeScript)**
- Modify: `src-bridge/src/runtime/backend-profiles.ts:93-108` — delete the `iflow: {…}` entry from `CLI_RUNTIME_METADATA`.
- Modify: `src-bridge/src/runtime/registry.ts:453` — delete the `iflow: createCliRuntimeAdapter(...)` registry entry.
- Modify: `src-bridge/src/runtime/registry.ts:1380-1381` — delete the `case "iflow": return "iflow";` branch.
- Modify: `src-bridge/src/runtime/registry.ts:1611` — delete the `case "iflow": { … }` block (read surrounding code to determine the block's end).
- Modify: `src-bridge/src/runtime/registry.test.ts` — delete lines 210/225/259-262 sample config and the two dedicated iflow tests (`"launches iflow through documented non-interactive prompt path and publishes sunset guidance"` at line 678, `"rejects iflow after the published sunset date"` at line 772); remove iflow from any helper tables (`cases` arrays, runtime lists).
- Modify: `src-bridge/src/handlers/command-runtime.ts:18` — remove `| "iflow"` from the runtime union at the top of the file.
- Modify: `src-bridge/src/server.ts:1112` — remove the `|| runtime.request.runtime === "iflow"` branch (or equivalent disjunction — confirm by reading 3-line context).
- Modify: `src-bridge/src/server.ts:1690,1768,1784-1785,1794` — remove `| "iflow"` from all inline runtime union types and the `case "iflow": return "iflow";` branch at 1784-1785.
- Modify: `src-bridge/src/server.test.ts:351,376` — drop the `case "iflow":` branch and the `return "iflow-token";` line.
- Modify: `src-bridge/src/schemas.ts:106` — remove `"iflow"` from the Zod enum (`.enum(["claude_code", ...])`).
- Modify: `src-bridge/src/schemas.test.ts:105` — remove `"iflow"` from the literal array `["cursor", "gemini", "qoder", "iflow"] as const`.
- Modify: `src-bridge/src/types.ts:10` — remove `| "iflow"` from the `AgentRuntimeKey` union.
- Modify: `src-bridge/src/types.ts:526` — remove `| "iflow"` from the runtime union on that line.

**Delete / edit in `src-go/` (Go orchestrator)**
- Modify: `src-go/internal/service/coding_agent_backend_profiles.json:181-` — delete the entire `{ "key": "iflow", … }` object (and its trailing comma / brace — validate the JSON after editing).
- Modify: `src-go/internal/service/coding_agent_test.go:45-47` — delete the `"iflow"` row from the table-driven test cases; delete `"iflow"` from the key list at line 97; delete the dedicated iflow assertion at 109-110.
- Modify: `src-go/internal/service/cost_query_service_test.go:295` — remove `"iflow"` from the key list.
- Modify: `src-go/internal/handler/project_handler_test.go:311-389` — delete the entire iflow-focused test block (the settings JSON with `"runtime":"iflow"`, the `Runtimes` entry with `Key: "iflow"`, the `iflow` variable scan loop, and the diagnostic assertion). Preserve surrounding test structure.

**Delete / edit in `src-im-bridge/` (IM Bridge)**
- Modify: `src-im-bridge/commands/login.go:41` — change the help string to `"不支持的登录目标 %q。用法: /login status|codex|claude|opencode|cursor|gemini|qoder"` (drop `|iflow`).
- Modify: `src-im-bridge/commands/login.go:64-65` — delete `case "iflow": return "iflow"`.
- Modify: `src-im-bridge/commands/login.go:181-185` — delete the `case "iflow":` block with its iFlow-specific reply text.
- Modify: `src-im-bridge/commands/catalog.go:77-78` — drop `|iflow` from the `Usage` and `UsageText` strings.
- Modify: `src-im-bridge/commands/catalog.go:87` — delete the iflow subcommand entry.
- Modify: `src-im-bridge/commands/catalog.go:305` — delete the `{Mentions: []string{"@iflow"}, Runtime: "iflow", Provider: "iflow"}` row.

**Delete / edit in frontend (`app/`, `lib/`, `messages/`)**
- Modify: `app/(dashboard)/settings/page.tsx:39` — remove `"iflow"` from the `RUNTIME_KEYS` literal array.
- Modify: `app/(dashboard)/settings/_components/section-runtime-detail.tsx:125-134` — delete the iflow fallback profile object (all of the iflow block in the fallback catalog).
- Modify: `app/(dashboard)/settings/_components/settings-sidebar.tsx:52` — delete the `{ id: "runtime-iflow", … }` line.
- Modify: `messages/zh-CN/settings.json:27` — delete the `"runtimeIFlow": "iFlow CLI"` key (and fix the preceding comma).
- Modify: `messages/en/settings.json:27` — delete the `"runtimeIFlow": "iFlow CLI"` key (and fix the preceding comma).
- Modify: `lib/settings/project-settings-workspace.test.ts` — grep for `iflow` in this file and delete matching assertions (block-level if needed). Discover the exact lines during the task.
- Modify: `components/docs/live-blocks/insertion-dialogs.tsx` — grep for `iflow`, delete the matching reference. Discover the exact line during the task.

**Delete / edit in docs and live specs**
- Modify: `CLAUDE.md` — the sentence listing runtime adapters (`claude_code, codex, opencode, cursor, gemini, qoder, iflow`): remove `, iflow`.
- Modify: `README.md` — grep for `iflow` (case-insensitive), remove.
- Modify: `openspec/specs/cli-agent-runtime-adapters/spec.md` — remove iflow from enumerated runtime lists, capability rows, and any normative clauses that reference it.
- Modify: `openspec/specs/coding-agent-provider-management/spec.md` — same.
- Modify: `openspec/specs/bridge-agent-runtime-registry/spec.md` — same.
- Modify: `openspec/specs/agent-sdk-bridge-runtime/spec.md` — same.

**Explicitly left alone**
- `openspec/changes/archive/**` (historical changes)
- `docs/dev/specs/2026-04-16-*.md`, `docs/dev/plans/2026-04-16-*.md` (archived artifacts)
- `docs/architecture/**`, `docs/research/**`, `docs/guides/role-yaml-reference.md` (non-normative; separate doc-sweep follow-up)

### Phase 2 — Review orchestrator plugin-only (commit 2)

- Modify: `src-bridge/src/review/types.ts:1-5` — `ReviewDimension` becomes `export type ReviewDimension = string;` (keep the alias so downstream `ReviewDimension[]` references don't break, but widen it to string).
- Modify: `src-bridge/src/review/types.ts:34` — delete the `dimensions?: ReviewDimension[];` field from `DeepReviewRequest`.
- Modify: `src-bridge/src/review/orchestrator.ts` — delete lines 13-18 (`DEFAULT_DIMENSIONS`), 27-39 (`buildFinding`), 41-168 (the four reviewer functions), 170-175 (`reviewers` record). Rewrite the body of `createDeepReviewOrchestrator` to call plugins only (see Phase 2 Task 4 for the exact new body).
- Modify: `src-bridge/src/review/orchestrator.test.ts` — replace built-in-dimension tests with three new cases (no plugins / one success / one failure). Exact test bodies in Phase 2 Task 2.
- Modify: `src-bridge/src/review/aggregator.test.ts` — no logic change expected. Verify it still compiles after the `ReviewDimension` widening.
- Modify: `src-bridge/src/review/types.test.ts` — remove assertions that rely on the four-literal `ReviewDimension`; keep whatever remains meaningful after the widening.
- Modify: `src-bridge/src/schemas.ts` — find the `DeepReviewRequestSchema` (grep `DeepReviewRequestSchema` in that file); delete the `dimensions` field; if there is a Zod enum over the four literals, change it to `z.string()` or delete it.
- Modify: `src-bridge/src/schemas.test.ts` — drop any assertion that `dimensions` is accepted/validated against the four literals.

---

## Pre-Flight

- [ ] **Step 1: Confirm branch and unrelated modified files.**

Run:
```bash
rtk git status
rtk git log --oneline -3
```
Expected: on `feat/employee-trigger-foundation`, and exactly these three files modified (untouched by this plan): `src-go/internal/repository/workflow_pending_review_repo.go`, `src-go/internal/server/routes.go`, `src-go/internal/service/im_action_execution.go`. If the tree has any other modifications, stop and surface them.

- [ ] **Step 2: Baseline verification (know the starting state).**

Run:
```bash
rtk pnpm lint
rtk pnpm exec tsc --noEmit
cd src-bridge && rtk bun test
cd .. && cd src-go && rtk go test ./...
cd ..
```
Expected: all four should currently pass. If any of them is already red on this branch, note which failures are pre-existing and do not count them as regressions. Only new failures introduced by this plan count.

---

## Phase 1 — iflow removal

### Task 1: Delete the iflow profile from the Go-side JSON (source of truth)

**Files:**
- Modify: `src-go/internal/service/coding_agent_backend_profiles.json`

- [ ] **Step 1: Read the file around the iflow entry (lines 175-210) to find the exact object boundaries.**

```bash
rtk read src-go/internal/service/coding_agent_backend_profiles.json | head -210 | tail -45
```

- [ ] **Step 2: Delete the entire iflow object.**

Use the Edit tool to remove the `{ "key": "iflow", … }` object and fix any surrounding JSON punctuation (trailing comma before `]` must be absent; internal commas between siblings must remain).

- [ ] **Step 3: Validate JSON.**

Run:
```bash
node -e "JSON.parse(require('fs').readFileSync('src-go/internal/service/coding_agent_backend_profiles.json','utf8')); console.log('ok')"
```
Expected: `ok`.

### Task 2: Delete iflow from bridge TypeScript core files

**Files:**
- Modify: `src-bridge/src/runtime/backend-profiles.ts`
- Modify: `src-bridge/src/types.ts`
- Modify: `src-bridge/src/schemas.ts`
- Modify: `src-bridge/src/handlers/command-runtime.ts`

- [ ] **Step 1: Delete the iflow block from `backend-profiles.ts` (lines 93-108).**

Remove the `iflow: { cli_launch: {…}, lifecycle: {…} },` entry from `CLI_RUNTIME_METADATA`. Leave `qoder` as the last entry (no trailing comma issues — the object literal tolerates trailing commas in TS).

- [ ] **Step 2: Delete `| "iflow"` from `types.ts` line 10 (`AgentRuntimeKey` union) and line 526 (the inner runtime union).**

- [ ] **Step 3: Delete `"iflow"` from the Zod enum in `schemas.ts:106`.**

New value: `.enum(["claude_code", "codex", "opencode", "cursor", "gemini", "qoder"])`.

- [ ] **Step 4: Delete `| "iflow"` from `handlers/command-runtime.ts:18`.**

### Task 3: Delete iflow from bridge runtime registry

**Files:**
- Modify: `src-bridge/src/runtime/registry.ts`

- [ ] **Step 1: Read context around line 453 (the registry entry), 1380 (the case branch), and 1611 (the dispatch block) to see the surrounding code and find each block's exact boundaries.**

```bash
rtk read src-bridge/src/runtime/registry.ts | sed -n '450,460p;1375,1390p;1605,1640p'
```

- [ ] **Step 2: Delete the `iflow: createCliRuntimeAdapter(...)` entry around line 453.**

- [ ] **Step 3: Delete the `case "iflow": return "iflow";` branch around line 1380.**

- [ ] **Step 4: Delete the entire `case "iflow": { … }` block at lines 1611-1623 (13 lines; verify the exact span by reading 1605-1640 in Step 1 above).**

- [ ] **Step 5: Search the file for any remaining `iflow` occurrences.**

Run:
```bash
rtk grep iflow src-bridge/src/runtime/registry.ts
```
Expected: zero matches. If matches remain, read context and delete.

### Task 4: Update bridge server and its test

**Files:**
- Modify: `src-bridge/src/server.ts`
- Modify: `src-bridge/src/server.test.ts`

- [ ] **Step 1: In `server.ts`, remove `runtime.request.runtime === "iflow"` from the disjunction at line 1112.**

Read 3 lines of context first; if the result is a dangling `|| ` or empty condition, simplify accordingly.

- [ ] **Step 2: In `server.ts`, remove `| "iflow"` from every inline runtime union type. Confirmed hits: lines 1690, 1768, 1794.**

- [ ] **Step 3: In `server.ts`, delete the `case "iflow": return "iflow";` branch near line 1784-1785.**

- [ ] **Step 4: In `server.test.ts`, delete the `case "iflow":` branch at line 351 and the `return "iflow-token";` at line 376 (they are in the same mocked switch).**

### Task 5: Update bridge registry test

**Files:**
- Modify: `src-bridge/src/runtime/registry.test.ts`

- [ ] **Step 1: Delete the two entire test blocks: the `test("launches iflow through documented non-interactive prompt path and publishes sunset guidance", …)` block at line 678 and the `test("rejects iflow after the published sunset date", …)` block at line 772. Delete from the `test(` line through the matching closing `);`.**

- [ ] **Step 2: Remove iflow from any shared fixtures / helper tables earlier in the file (the case/return at 210/225 and the sample profile at 259-262).**

- [ ] **Step 3: Verify no remaining iflow in the file.**

```bash
rtk grep iflow src-bridge/src/runtime/registry.test.ts
```
Expected: zero matches.

### Task 6: Update bridge schema test

**Files:**
- Modify: `src-bridge/src/schemas.test.ts`

- [ ] **Step 1: Remove `"iflow"` from the literal array at line 105.**

New: `for (const runtime of ["cursor", "gemini", "qoder"] as const) {`.

### Task 7: Update Go orchestrator tests

**Files:**
- Modify: `src-go/internal/service/coding_agent_test.go`
- Modify: `src-go/internal/service/cost_query_service_test.go`
- Modify: `src-go/internal/handler/project_handler_test.go`

- [ ] **Step 1: In `coding_agent_test.go`, delete the `"iflow"` row from the table-driven cases (lines 45-47 are the row; remove the enclosing struct literal entirely including its trailing comma).**

- [ ] **Step 2: In `coding_agent_test.go`, remove `"iflow"` from the key slice at line 97 and delete the dedicated iflow assertion (`if got := runtimeByKey["iflow"].DefaultModel; …`) at 109-110.**

- [ ] **Step 3: In `cost_query_service_test.go:295`, remove `"iflow"` from the key slice.**

- [ ] **Step 4: In `project_handler_test.go`, delete the entire iflow-related test body spanning lines ~311-389.**

Read surrounding code to find the test function's boundaries. If the whole test function is iflow-specific, delete the function. If the test has multiple cases and iflow is one, delete only the iflow case and any helper block tied to it (the settings JSON, the `Runtimes[]` entry with `Key: "iflow"`, the `iflow` variable scan loop, and the dedicated assertion for `stale_default_selection` diagnostic).

### Task 8: Update IM bridge commands

**Files:**
- Modify: `src-im-bridge/commands/login.go`
- Modify: `src-im-bridge/commands/catalog.go`

- [ ] **Step 1: In `login.go:41`, change the Chinese help string to drop `|iflow`.**

New: `"不支持的登录目标 %q。用法: /login status|codex|claude|opencode|cursor|gemini|qoder"`.

- [ ] **Step 2: In `login.go:64-65`, delete the `case "iflow": return "iflow"` branch.**

- [ ] **Step 3: In `login.go:181-185`, delete the `case "iflow":` block entirely (through its closing `}` or until the next case).**

- [ ] **Step 4: In `catalog.go:77-78`, drop `|iflow` from `Usage` and `UsageText`.**

- [ ] **Step 5: In `catalog.go:87`, delete the `{Name: "iflow", …}` entry.**

- [ ] **Step 6: In `catalog.go:305`, delete the `{Mentions: []string{"@iflow"}, Runtime: "iflow", Provider: "iflow"}` row.**

### Task 9: Update frontend

**Files:**
- Modify: `app/(dashboard)/settings/page.tsx`
- Modify: `app/(dashboard)/settings/_components/section-runtime-detail.tsx`
- Modify: `app/(dashboard)/settings/_components/settings-sidebar.tsx`
- Modify: `messages/zh-CN/settings.json`
- Modify: `messages/en/settings.json`
- Modify: `lib/settings/project-settings-workspace.test.ts`
- Modify: `components/docs/live-blocks/insertion-dialogs.tsx`

- [ ] **Step 1: In `app/(dashboard)/settings/page.tsx:39`, remove `"iflow"` from `RUNTIME_KEYS`.**

- [ ] **Step 2: In `section-runtime-detail.tsx`, find and delete the full iflow profile object starting around line 125. Read ~20 lines around to find the object's braces.**

- [ ] **Step 3: In `settings-sidebar.tsx:52`, delete the `{ id: "runtime-iflow", … }` line.**

- [ ] **Step 4: In both `messages/zh-CN/settings.json` and `messages/en/settings.json`, remove the `"runtimeIFlow": "iFlow CLI"` key at line 27. Fix the comma on the preceding line so the JSON stays valid.**

- [ ] **Step 5: Locate and clean up `lib/settings/project-settings-workspace.test.ts`.**

```bash
rtk grep iflow lib/settings/project-settings-workspace.test.ts
```
Read each matching block; remove the iflow assertion/case. If removing it leaves an empty test, delete the test block entirely.

- [ ] **Step 6: Locate and clean up `components/docs/live-blocks/insertion-dialogs.tsx`.**

```bash
rtk grep iflow components/docs/live-blocks/insertion-dialogs.tsx
```
Remove the matching reference. Read ~5 lines of context to decide whether to drop a list entry, a switch case, etc.

### Task 10: Update docs and live OpenSpec specs

**Files:**
- Modify: `CLAUDE.md`
- Modify: `README.md`
- Modify: `openspec/specs/cli-agent-runtime-adapters/spec.md`
- Modify: `openspec/specs/coding-agent-provider-management/spec.md`
- Modify: `openspec/specs/bridge-agent-runtime-registry/spec.md`
- Modify: `openspec/specs/agent-sdk-bridge-runtime/spec.md`

- [ ] **Step 1: In `CLAUDE.md`, find the line listing runtime adapters and drop `, iflow`.**

```bash
rtk grep "iflow" CLAUDE.md
```

- [ ] **Step 2: In `README.md`, grep and remove iflow references.**

```bash
rtk grep -i iflow README.md
```

- [ ] **Step 3: For each of the four OpenSpec spec files, remove iflow from enumerated runtime lists, capability matrices, and normative clauses.**

```bash
rtk grep -i iflow openspec/specs/cli-agent-runtime-adapters/spec.md openspec/specs/coding-agent-provider-management/spec.md openspec/specs/bridge-agent-runtime-registry/spec.md openspec/specs/agent-sdk-bridge-runtime/spec.md
```

For each hit: read 5-10 lines of context; delete list entries, matrix rows, or whole clauses as appropriate. Do not rewrite unrelated paragraphs.

### Task 11: Phase 1 verification gate

- [ ] **Step 1: Run bridge and TypeScript checks.**

```bash
rtk pnpm lint
rtk pnpm exec tsc --noEmit
cd src-bridge && rtk bun test
```
Expected: all three green. Any iflow-related test failure means a reference was missed. Find and fix, repeat.

- [ ] **Step 2: Run Go tests.**

```bash
cd src-go && rtk go test ./...
```
Expected: green.

- [ ] **Step 3: Final iflow grep over live files.**

```bash
rtk grep -iE "iflow" src-bridge src-go src-im-bridge app components lib messages CLAUDE.md README.md openspec/specs
```
Expected: **zero** matches. A match under `openspec/changes/archive/**` or `docs/dev/{specs,plans}/**` is acceptable — this grep intentionally excludes them.

If any live match remains, clean it in the relevant task above and re-run verification.

### Task 12: Phase 1 commit

- [ ] **Step 1: Stage only the files this phase touched.**

Explicitly list the paths from tasks 1-10 in `rtk git add`. **Do not** use `rtk git add .` or `rtk git add -A` — the three unrelated pre-existing modifications must remain unstaged.

Example shape (expand with the actual list):
```bash
rtk git add \
  src-bridge/src/runtime/backend-profiles.ts \
  src-bridge/src/runtime/registry.ts \
  src-bridge/src/runtime/registry.test.ts \
  src-bridge/src/handlers/command-runtime.ts \
  src-bridge/src/server.ts \
  src-bridge/src/server.test.ts \
  src-bridge/src/schemas.ts \
  src-bridge/src/schemas.test.ts \
  src-bridge/src/types.ts \
  src-go/internal/service/coding_agent_backend_profiles.json \
  src-go/internal/service/coding_agent_test.go \
  src-go/internal/service/cost_query_service_test.go \
  src-go/internal/handler/project_handler_test.go \
  src-im-bridge/commands/login.go \
  src-im-bridge/commands/catalog.go \
  app/\(dashboard\)/settings/page.tsx \
  app/\(dashboard\)/settings/_components/section-runtime-detail.tsx \
  app/\(dashboard\)/settings/_components/settings-sidebar.tsx \
  lib/settings/project-settings-workspace.test.ts \
  components/docs/live-blocks/insertion-dialogs.tsx \
  messages/zh-CN/settings.json \
  messages/en/settings.json \
  CLAUDE.md \
  README.md \
  openspec/specs/cli-agent-runtime-adapters/spec.md \
  openspec/specs/coding-agent-provider-management/spec.md \
  openspec/specs/bridge-agent-runtime-registry/spec.md \
  openspec/specs/agent-sdk-bridge-runtime/spec.md
```

- [ ] **Step 2: Sanity-check status.**

```bash
rtk git status
```
Expected: the unrelated 3 files still shown as "Modified" (not staged); the 27-ish files above all staged.

- [ ] **Step 3: Commit.**

```bash
rtk git commit -m "$(cat <<'EOF'
refactor(bridge): remove iflow runtime (sunset 2026-04-17)

iFlow CLI reached its published sunset on 2026-04-17. Rather than
keep it as dead code behind a "sunsetting" label, delete the runtime
end-to-end: bridge adapter and tests, Go orchestrator profile JSON
and tests, IM bridge login/catalog commands, frontend settings
options and i18n keys, and live OpenSpec specs. Archived OpenSpec
change records and historical docs are intentionally left untouched.

Existing DB rows with backend_runtime="iflow" will fail at load with
an "unknown runtime" error; operators migrate them manually. Project
is in internal-testing stage where breaking contract changes are
permitted.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 2 — Review orchestrator plugin-only

### Task 1: Update `ReviewDimension` type and `DeepReviewRequest`

**Files:**
- Modify: `src-bridge/src/review/types.ts`

- [ ] **Step 1: Change the `ReviewDimension` alias to `string`.**

Replace lines 1-5:
```typescript
export type ReviewDimension =
  | "logic"
  | "security"
  | "performance"
  | "compliance";
```
with:
```typescript
export type ReviewDimension = string;
```

- [ ] **Step 2: Delete the `dimensions?: ReviewDimension[];` line from `DeepReviewRequest` (line 34).**

### Task 2: Rewrite orchestrator tests (TDD — write the new contract first)

**Files:**
- Modify: `src-bridge/src/review/orchestrator.test.ts`

- [ ] **Step 1: Read the current test file to see its imports, helpers, and existing structure.**

```bash
rtk read src-bridge/src/review/orchestrator.test.ts
```

- [ ] **Step 2: Replace the built-in-dimension tests with three new cases (no plugins / one success / one failure). Preserve the file's imports, shared fixtures, and any aggregator-integration assertions that are still meaningful under the new behavior.**

Example shape for the three new cases (adapt to the file's existing `describe` / `test` style — Bun's `bun:test`):

```typescript
import { test, expect } from "bun:test";
import { createDeepReviewOrchestrator } from "./orchestrator";
import type { DeepReviewRequest, ReviewExecutionResult } from "./types";

function baseRequest(overrides: Partial<DeepReviewRequest> = {}): DeepReviewRequest {
  return {
    review_id: "r-1",
    task_id: "t-1",
    pr_url: "https://example.com/pr/1",
    ...overrides,
  };
}

test("returns empty result with explicit summary when no plugins are configured", async () => {
  const run = createDeepReviewOrchestrator();
  const response = await run(baseRequest());

  expect(response.dimension_results).toEqual([]);
  expect(response.findings).toEqual([]);
  expect(response.summary).toBe("No review plugins configured.");
});

test("runs a single successful plugin and includes its findings", async () => {
  const pluginResult: ReviewExecutionResult = {
    dimension: "custom",
    source_type: "plugin",
    plugin_id: "demo",
    display_name: "Demo",
    status: "completed",
    findings: [
      {
        category: "custom",
        severity: "medium",
        message: "demo finding",
      },
    ],
    summary: "demo ran",
  };
  const run = createDeepReviewOrchestrator({
    executeReviewPlugin: async () => pluginResult,
  });
  const response = await run(
    baseRequest({
      review_plugins: [{ plugin_id: "demo", name: "Demo", entrypoint: "review", transport: "stdio" }],
    }),
  );

  expect(response.dimension_results).toHaveLength(1);
  expect(response.dimension_results[0]?.source_type).toBe("plugin");
  expect(response.findings[0]?.message).toBe("demo finding");
});

test("marks a failing plugin as failed without throwing", async () => {
  const run = createDeepReviewOrchestrator({
    executeReviewPlugin: async () => {
      throw new Error("plugin crashed");
    },
  });
  const response = await run(
    baseRequest({
      review_plugins: [{ plugin_id: "bad", name: "Bad", entrypoint: "review", transport: "stdio" }],
    }),
  );

  expect(response.dimension_results).toHaveLength(1);
  expect(response.dimension_results[0]?.status).toBe("failed");
  expect(response.dimension_results[0]?.error).toContain("plugin crashed");
});
```

- [ ] **Step 3: Run the tests to confirm they fail (expected — orchestrator still has built-ins, `dimensions` field still exists).**

```bash
cd src-bridge && rtk bun test src/review/orchestrator.test.ts
```
Expected: **FAIL**. Typical failures: type errors or the no-plugin case returning four built-in results. Note the output to confirm the change target.

### Task 3: Update schema

**Files:**
- Modify: `src-bridge/src/schemas.ts`
- Modify: `src-bridge/src/schemas.test.ts`

- [ ] **Step 1: Locate `DeepReviewRequestSchema` and `ReviewDimensionSchema` in `schemas.ts`.**

```bash
rtk grep -nE "DeepReviewRequestSchema|ReviewDimensionSchema" src-bridge/src/schemas.ts
```

Expected: `ReviewDimensionSchema` is defined as a Zod enum over the four literals and is used as `z.array(ReviewDimensionSchema).optional()` on line 288 inside `DeepReviewRequestSchema`.

- [ ] **Step 2: Delete the `dimensions` field from `DeepReviewRequestSchema` (the `z.array(ReviewDimensionSchema).optional()` line around 288).**

- [ ] **Step 3: Decide whether to delete `ReviewDimensionSchema` entirely or widen it to `z.string()`.**

```bash
rtk grep -n ReviewDimensionSchema src-bridge
```
If the schema is only referenced inside `DeepReviewRequestSchema` (likely), delete the export entirely. If other consumers exist, widen it to `z.string()` and keep the export name.

- [ ] **Step 4: In `schemas.test.ts`, remove any assertion that validates `dimensions` against the four literals.**

### Task 4: Delete built-in reviewers and simplify orchestrator body

**Files:**
- Modify: `src-bridge/src/review/orchestrator.ts`

- [ ] **Step 1: Delete these blocks, in order top-to-bottom:**
  - Lines 13-18: the `DEFAULT_DIMENSIONS` constant.
  - Lines 27-39: the `buildFinding` helper.
  - Lines 41-74: `reviewLogic`.
  - Lines 76-109: `reviewSecurity`.
  - Lines 111-144: `reviewPerformance`.
  - Lines 146-168: `reviewCompliance`.
  - Lines 170-175: the `reviewers` record.

- [ ] **Step 2: Rewrite the body of `createDeepReviewOrchestrator` so the returned function only runs plugins.**

Replace the current inner function (starting around line 182) with this shape (adjust imports and preserve anything outside `createDeepReviewOrchestrator` that still applies):

```typescript
export function createDeepReviewOrchestrator(options: DeepReviewOrchestratorOptions = {}) {
  const executeReviewPlugin =
    options.executeReviewPlugin ??
    (async (plugin: ReviewPluginExecution, request: DeepReviewRequest) =>
      executeMcpReviewPlugin(plugin, request));

  return async function runDeepReview(request: DeepReviewRequest): Promise<DeepReviewResponse> {
    const plugins = request.review_plugins ?? [];

    if (plugins.length === 0) {
      return {
        ...aggregateReviewResults([]),
        summary: "No review plugins configured.",
      };
    }

    const pluginSettled = await Promise.allSettled(
      plugins.map(async (plugin) => executeReviewPlugin(plugin, request)),
    );

    const pluginResults: ReviewExecutionResult[] = pluginSettled.map((result, index) => {
      const plugin = plugins[index]!;
      if (result.status === "fulfilled") {
        return {
          ...result.value,
          dimension: result.value.dimension || plugin.plugin_id,
          source_type: "plugin",
          plugin_id: result.value.plugin_id ?? plugin.plugin_id,
          display_name: result.value.display_name ?? plugin.name,
        };
      }
      return {
        dimension: plugin.plugin_id,
        source_type: "plugin",
        plugin_id: plugin.plugin_id,
        display_name: plugin.name,
        status: "failed",
        findings: [],
        summary: `${plugin.plugin_id} review failed`,
        error: result.reason instanceof Error ? result.reason.message : String(result.reason),
      };
    });

    return aggregateReviewResults(pluginResults);
  };
}
```

- [ ] **Step 3: Verify `aggregateReviewResults` handles `[]` input gracefully.**

Read `src-bridge/src/review/aggregator.ts` and confirm that passing `[]` does not throw and returns a sensible `DeepReviewResponse` shape. Then run the existing aggregator tests to check they still pass:

```bash
cd src-bridge && rtk bun test src/review/aggregator.test.ts
```

If aggregator throws or returns an object missing required fields when given `[]`, fix it in the same commit (minimum: handle the empty case by returning `{ risk_level: "low", findings: [], summary: "", recommendation: "approve", cost_usd: 0, dimension_results: [] }` or whatever matches the existing type — the orchestrator's `summary` override then applies on top).

- [ ] **Step 4: Verify no stale imports in `orchestrator.ts`.**

```bash
rtk pnpm exec tsc --noEmit
```
Clean up any now-unused imports reported by the compiler (`ReviewDimension`, `ReviewFinding`, `MCPClientHub` if still used by `executeMcpReviewPlugin`, etc.).

### Task 5: Clean up `types.test.ts`

**Files:**
- Modify: `src-bridge/src/review/types.test.ts`

- [ ] **Step 1: Read the file.**

```bash
rtk read src-bridge/src/review/types.test.ts
```

- [ ] **Step 2: If the file contains a `DeepReviewRequest` literal with a `dimensions: [...]` property (currently at line ~43 with `dimensions: ["logic", "security"]`), delete that property from the literal — after Phase 2 Task 1 removed the field from the interface, this becomes a TypeScript excess-property error. Also remove any assertion that relies specifically on the four-literal `ReviewDimension` union. Keep whatever remains meaningful (e.g., `ReviewFinding` shape, severity union).**

### Task 6: Phase 2 verification gate

- [ ] **Step 1: Run the review tests first (focused feedback).**

```bash
cd src-bridge && rtk bun test src/review/
```
Expected: green, including the three new cases from Task 2.

- [ ] **Step 2: Run full bridge test suite.**

```bash
cd src-bridge && rtk bun test
```
Expected: green.

- [ ] **Step 3: Full project checks.**

```bash
cd .. && rtk pnpm lint && rtk pnpm exec tsc --noEmit && cd src-go && rtk go test ./...
cd ..
```
Expected: all green.

- [ ] **Step 4: Acceptance grep — `dimensions` field is gone from the request type.**

```bash
rtk grep -nE "dimensions\??:" src-bridge/src/review src-bridge/src/schemas.ts
```
Expected: no match for `dimensions?:` or `dimensions:` on `DeepReviewRequest` / schema (there may still be references like `.dimension` singular on `ReviewExecutionResult` — those are fine).

- [ ] **Step 5: Acceptance grep — built-in reviewer functions are gone.**

```bash
rtk grep -nE "reviewLogic|reviewSecurity|reviewPerformance|reviewCompliance|DEFAULT_DIMENSIONS|function buildFinding" src-bridge/src/review
```
Expected: zero matches.

### Task 7: Phase 2 commit

- [ ] **Step 1: Stage the review-related files.**

```bash
rtk git add \
  src-bridge/src/review/orchestrator.ts \
  src-bridge/src/review/orchestrator.test.ts \
  src-bridge/src/review/types.ts \
  src-bridge/src/review/types.test.ts \
  src-bridge/src/schemas.ts \
  src-bridge/src/schemas.test.ts
```
If `src-bridge/src/review/aggregator.ts` was modified to handle empty input (Task 4 Step 3), include it. Do **not** include `aggregator.test.ts` unless its tests needed updates.

- [ ] **Step 2: Sanity-check.**

```bash
rtk git status
```
Expected: the three pre-existing unrelated modifications still unstaged; only review / schema files staged.

- [ ] **Step 3: Commit.**

```bash
rtk git commit -m "$(cat <<'EOF'
refactor(bridge/review): drop regex built-ins; Deep Review is plugin-only

The four built-in "reviewers" in src/review/orchestrator.ts were 5-10
line regex matches for TODO|FIXME, eval(, SELECT *, and console.log.
They did not constitute a code review and misrepresented what Deep
Review does. The orchestrator already supports MCP review plugins as
the real extension point — that is now the only execution path.

- Delete reviewLogic/reviewSecurity/reviewPerformance/reviewCompliance,
  DEFAULT_DIMENSIONS, reviewers record, buildFinding helper.
- Simplify the orchestrator to run plugins via Promise.allSettled;
  return an empty aggregated response with an explicit
  "No review plugins configured." summary when the plugin list is
  empty.
- Widen ReviewDimension from the four-literal union to string — plugins
  declare their own dimensions.
- Remove DeepReviewRequest.dimensions (no longer meaningful).
- Rewrite orchestrator tests around the new contract (no plugins,
  one success, one failure).

Follow-up: frontend Review UI still groups findings by the four
retired dimensions; adaptation is tracked separately.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Final Acceptance

- [ ] **Step 1: All four verification commands green.**

```bash
rtk pnpm lint
rtk pnpm exec tsc --noEmit
cd src-bridge && rtk bun test && cd ..
cd src-go && rtk go test ./... && cd ..
```

- [ ] **Step 2: Zero iflow hits in live files.**

```bash
rtk grep -iE "iflow" src-bridge src-go src-im-bridge app components lib messages CLAUDE.md README.md openspec/specs
```
Expected: no output.

- [ ] **Step 3: `DeepReviewRequest.dimensions` field is gone.**

```bash
rtk grep -nE "dimensions" src-bridge/src/review/types.ts src-bridge/src/schemas.ts
```
Expected: no `dimensions` on `DeepReviewRequest` or on the Zod schema.

- [ ] **Step 3b: `ReviewDimension` is widened to `string` (spec §7.4).**

```bash
rtk grep -n "ReviewDimension" src-bridge/src/review/types.ts
```
Expected: the line reads `export type ReviewDimension = string;` (or the alias is removed entirely and the type `string` is used directly).

- [ ] **Step 4: Two fresh commits on the branch.**

```bash
rtk git log --oneline -3
```
Expected: top two commits are the `refactor(bridge/review)` and `refactor(bridge)` commits, in that order.

- [ ] **Step 5: Unrelated files still unstaged and unmodified by this batch.**

```bash
rtk git status
```
Expected: the three pre-existing modifications (`src-go/internal/repository/workflow_pending_review_repo.go`, `src-go/internal/server/routes.go`, `src-go/internal/service/im_action_execution.go`) are still listed as `Modified`, unstaged. If any of them ended up in a commit, stop and surface it.

---

## Note on acceptance criterion §7.6

Spec §7.6 asks for an "integration smoke — POST a Deep Review request with zero `review_plugins` — returns a well-formed response with an empty findings array and a summary explicitly stating no plugins were configured". In this plan, that criterion is satisfied by the unit test added in Phase 2 Task 2 Case 1 (`returns empty result with explicit summary when no plugins are configured`), which exercises the orchestrator's entry point directly with an empty-plugin request. If a full HTTP smoke is also desired later, it is a follow-up.

---

## Reference

- Spec: `docs/dev/specs/2026-04-19-ts-bridge-batch1-iflow-review-design.md`
- Audit origin: TS Bridge audit conducted 2026-04-19 in brainstorming session
- Follow-up items (not in this plan): frontend Review UI dimension-grouping adaptation; CLI runtime high-level operation stubs (cursor/gemini/qoder); cost calculator hardcoded model name; opencode-runtime async-without-await audit; non-normative doc sweep for stale iflow mentions; Batch 2 (split `server.ts` and `runtime/registry.ts`).
