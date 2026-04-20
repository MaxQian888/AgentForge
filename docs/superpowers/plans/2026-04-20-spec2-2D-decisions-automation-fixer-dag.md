# Spec 2D — Findings Decision API + Automation Rule + code_fixer DAG + Role + FE Diff Viewer

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地 Spec 2 §5 S2-C automation + S2-D code_fixer DAG + S2-F decision handler + §9 Trace B 用户交互闭环 + §13 FE diff viewer。

**Architecture:** review.completed → automation rule 为每条可修复 finding 发可点击 IM 卡片（沿用 1E im_send + card_action）；用户点 Apply 经 1E card_action_router 路由到 findings_decision_handler.Decide → spawn 系统 code_fixer DAG（trigger→http_call fetch→condition→llm_agent role='code_fixer'→validate→execute→card）；FE 加 diff viewer + per-finding Approve/Dismiss/Defer 按钮 + suggested_patch 预览。

**Tech Stack:** Go (workflow nodes 复用 Spec 1, role manifest YAML), TypeScript (bridge findings/v2 schema), Next.js + react-diff-viewer-continued (or monaco-diff).

**Depends on:** Spec 1E (im_send + card_action_router + wait_event), Spec 2A (vcs.Provider for fetch_file http_call header), Spec 2B (review_findings 扩展 columns)

**Parallel with:** 2C, 2E — disjoint files

**Unblocks:** 2E（其 fix_runner 端点是本 plan 中 DAG 的 execute 节点的目标）

---

## Coordination Notes (read first)

- **Spec 2B columns required**: `review_findings.suggested_patch text NULL`, `decision varchar(16) NOT NULL DEFAULT 'pending'`, `dismissed boolean NOT NULL DEFAULT false` are migrated by Plan 2B. This plan only consumes them — never re-add migrations.
- **fix_runs table**: also from 2B; this plan only reads `fix_runs WHERE finding_id = ?` for the per-finding history pane (Task 16). Wiring writes is 2E's responsibility.
- **fix_runner endpoints** (`POST /api/v1/internal/fix-runs/dry-run`, `POST /api/v1/internal/fix-runs/execute`): wired by Plan 2E. Seed the DAG with these URLs as placeholders; the DAG seeds and tests must be green even if the endpoints return 501 today.
- **vcs.Provider headers**: `fetch_file` http_call uses `vcs.Provider.GetRawFile(...)` indirectly via an internal endpoint exposed by 2A. If 2A names the route differently than `/api/v1/internal/vcs/raw`, fix the seed in Task 9 only.
- **Spec 1E `imcards` package**: this plan's automation card path MUST go through `imcards.SendActionCard(...)` exposed by 1E (Step E5). If 1E's helper has a different export name (e.g. `SendCard` / `EmitCard`), align in Task 6 only.
- **card_action_router automation branch**: 1E's router currently dispatches by `(execution_id, node_id) → wait_event resumer`. This plan adds a sibling branch: when a correlation row's `execution_id IS NULL`, dispatch to `automation_action_handler` (added here in Task 7).
- **`reviews.head_branch` / `head_sha`**: Plan 2A adds these denormalised columns onto `reviews`. Task 5 reads them; if 2A puts them on `review_executions` instead, fix Task 5's selector.
- **Frontend diff viewer library**: `package.json` does NOT currently include `react-diff-viewer-continued`, `monaco-editor`, `@monaco-editor/react`, or `diff` — Task 14 must add `react-diff-viewer-continued` (lighter weight, no monaco bundle bloat). Confirm with `pnpm view react-diff-viewer-continued version` before pinning.

---

## Tasks

### Phase 1 — Bridge SDK schema upgrade (foundation, no Go deps)

- [ ] **Task 1: failing test — findings/v2 schema parses with `suggested_patch`**
  - File: `src-bridge/src/plugin-sdk/review.test.ts`
  - Add cases:
    1. `createReviewResult` invoked with `suggestedPatch` on a finding emits `structuredContent.format === "findings/v2"` and the patch field round-trips.
    2. `createReviewResult` with NO patches stays on `format: "findings/v1"` (back-compat — old plugins keep their wire shape).
    3. Mixed: at least one finding with patch ⇒ envelope upgrades to v2; v1 findings get `suggested_patch: null` filled by the helper.
  - Run: `rtk pnpm --filter @agentforge/bridge test -- review.test` — fails (no `suggested_patch` field plumbed).

- [ ] **Task 2: extend `ReviewFindingInput` + `FindingsPayload` to v2**
  - File: `src-bridge/src/plugin-sdk/review.ts`
  - Changes:
    1. `ReviewFindingInput` gains optional `suggestedPatch?: string` (camelCase to match SDK convention; serialize as `suggested_patch`).
    2. `createReviewFinding` copies `suggestedPatch` onto the output finding under snake_case key `suggested_patch` (or extend `ReviewFinding` type in `src-bridge/src/review/types.ts` to include it).
    3. `createReviewResult` walks findings; if any has a non-empty `suggested_patch`, set `format: "findings/v2"`; otherwise keep `findings/v1`. Always normalize missing patches to `null` on v2.
  - Edit `src-bridge/src/review/types.ts` — add `suggested_patch?: string | null` on `ReviewFinding`. Keep existing fields untouched.
  - Run: `rtk pnpm --filter @agentforge/bridge test -- review.test` — green. Run `rtk tsc --project src-bridge` — green.

### Phase 2 — Go side schema + ReviewService.Complete back-compat

- [ ] **Task 3: failing test — ReviewService.Complete accepts findings/v2 payload**
  - File: `src-go/internal/service/review_service_test.go`
  - Add `TestComplete_AcceptsFindingsV2WithSuggestedPatch`:
    - Given: a review created via fixture; call `Complete(ctx, id, &model.CompleteReviewRequest{Findings: []ReviewFinding{{... SuggestedPatch: "--- a/foo\n+++ b/foo\n@@ -1 +1 @@\n-x\n+y\n"}}})`.
    - Expect: stored review.Findings[0].SuggestedPatch == that diff; `dismissed=false`, `decision="pending"` defaults.
  - Add `TestComplete_AcceptsLegacyV1WithoutPatch`: same but no patch ⇒ stored row has `SuggestedPatch == ""` and serializes to `null` over the wire.
  - Run: `rtk go test ./internal/service -run TestComplete_Accepts` — fails (no field on model).

- [ ] **Task 4: add `SuggestedPatch` to `model.ReviewFinding`; teach repo to round-trip**
  - File: `src-go/internal/model/review.go`
    - Add `SuggestedPatch string \`json:"suggested_patch,omitempty\"\`` after `Suggestion`.
    - Add `Decision string \`json:"decision,omitempty"\`` (defaults `"pending"`; values: `pending|approved|dismissed|deferred|needs_manual_fix`).
  - File: `src-go/internal/repository/review_repo.go` (or whichever file owns finding JSONB serde — Grep `MarshalFindings` / `findings json` to confirm)
    - Ensure JSONB roundtrip preserves new fields. Plan 2B already adds the columns; double-check the repo writes them through the same JSONB blob (most likely yes — findings are stored as a single JSON column).
  - File: `src-go/internal/service/review_service.go`
    - In `Complete`, after merging incoming findings: leave `SuggestedPatch` untouched (value-pass); when a finding is missing, default `Decision = "pending"`.
  - Run: `rtk go test ./internal/service -run TestComplete_Accepts` — green. Run `rtk go test ./internal/model ./internal/repository` — green.

### Phase 3 — Findings Decision API

- [ ] **Task 5: failing test — POST /api/v1/findings/:id/decision (all three branches)**
  - File: `src-go/internal/handler/findings_decision_handler_test.go`
  - Cases (each via Echo test recorder):
    1. `TestDecide_ApproveSpawnsCodeFixer`: stub `codeFixerSpawner` captures `(reviewID, findingID, integrationID, headSha, employeeID)`; assert HTTP 202 + body `{executionId: "<uuid>"}`. Verify finding.Decision becomes `"approved"`.
    2. `TestDecide_DismissSetsFlagAndEmits`: assert finding.Decision=`"dismissed"`, `dismissed=true`; eventbus sees `EventFindingDismissed`; HTTP 200.
    3. `TestDecide_DeferSetsDecisionOnly`: assert finding.Decision=`"deferred"`, no spawn, no event; HTTP 200.
    4. `TestDecide_RejectsUnknownAction`: body `{action:"unknown"}` ⇒ 400.
    5. `TestDecide_AuditLogPerCall`: stubs `auditWriter` records action + actor + finding_id.
    6. `TestDecide_RBAC_ApproveDismissRequireEditor`: as a viewer, approve/dismiss ⇒ 403; defer ⇒ 200.
  - Run: `rtk go test ./internal/handler -run TestDecide_` — fails.

- [ ] **Task 6: implement `findings_decision_handler.go` + service helper**
  - File: `src-go/internal/handler/findings_decision_handler.go`
    - Handler struct holds `findings findingsRepo`, `reviews ReviewRepository`, `spawner codeFixerSpawner`, `bus eventbus.Publisher`, `audit AuditWriter`, `rbac RBACChecker`.
    - Body: `type DecisionRequest struct { Action string \`json:"action" validate:"required,oneof=approve dismiss defer"\`; Comment string \`json:"comment,omitempty"\` }`.
    - Flow:
      1. Parse `:id` (finding UUID); load finding (and parent review for `head_sha`/`integration_id`/`project_id`).
      2. RBAC: `approve|dismiss` ⇒ require `editor`, `defer` ⇒ require `viewer`.
      3. Switch on action:
         - `approve`: call `spawner.Spawn(ctx, codeFixerInput{ReviewID, FindingID, IntegrationID, HeadSha, EmployeeID: actingEmployeeID})`; persist `finding.Decision="approved"`; return 202 `{executionId}`.
         - `dismiss`: persist `Decision="dismissed", Dismissed=true`; `bus.Publish(EventFindingDismissed, payload)`; return 200.
         - `defer`: persist `Decision="deferred"`; return 200.
      4. Always: `audit.Append(ctx, "finding.decision", actor, payload{findingID, action, comment})`.
  - File: `src-go/internal/eventbus/types.go`: add `EventFindingDismissed = "finding.dismissed"`.
  - Wire route in `src-go/internal/server/`: `POST /api/v1/findings/:id/decision`.
  - File: `src-go/internal/service/code_fixer_spawner.go` — thin wrapper over `DAGWorkflowService.Start` referencing the seeded `code_fixer` definition by name (resolved via `WorkflowTemplateService.GetByName(ctx, "code_fixer")`); returns the new execution UUID.
  - Run: `rtk go test ./internal/handler -run TestDecide_` — green. `rtk go test ./internal/server` — green.

### Phase 4 — Automation rule for review.completed

- [ ] **Task 7: failing test — automation_rules.review_completed emits cards conditionally**
  - File: `src-go/internal/automation/review_completed_rule_test.go` (or extend `internal/service/automation_engine_service_test.go` if the framework lives there — Grep `EvaluateRules` to confirm rule wiring)
  - Cases:
    1. `TestRule_SkipsFixBranches`: review with `head_branch="fix/abc/def"` ⇒ helper called 0 times.
    2. `TestRule_EmitsCardForActionableFinding`: severity ≥ project.threshold AND `(suggested_patch != ""  OR  suggestion != "")` ⇒ exactly one `imcards.SendActionCard` invocation per qualifying finding; payload contains title `Apply fix? — <message[:60]>`, fields `[file:line, severity, source]`, actions `[apply, dismiss, view]`.
    3. `TestRule_SkipsBelowThreshold`: severity below threshold ⇒ skipped.
    4. `TestRule_SkipsNonActionable`: no patch + no suggestion ⇒ skipped.
    5. `TestRule_AutomationDecisionManualOnly`: project.automation_decision=`manual_only` (column from 2B) ⇒ rule does nothing (FE-only flow).
    6. `TestRule_MintsCorrelationWithNullExecutionID`: for each callback action, assert one row written into `card_action_correlations` with `execution_id IS NULL`, `node_id = "(automation)"`, `correlation_payload = {finding_id, action}`.
  - Stubs: fake `imcards.Sender`, fake `corrRepo`, fake `reviews` repo loader.
  - Run: `rtk go test ./internal/automation -run TestRule_` — fails.

- [ ] **Task 8: implement `review_completed_rule.go`**
  - File: `src-go/internal/automation/review_completed_rule.go`
  - Subscribe to `EventReviewCompleted` (register at server boot in the existing eventbus subscriber wiring — Grep `bus.Subscribe(eventbus.EventReview` to find the right boot path).
  - Pseudocode:
    ```go
    func (r *ReviewCompletedRule) Handle(ctx context.Context, evt eventbus.Envelope) error {
        review, err := r.reviews.GetByID(ctx, evt.ReviewID)
        if err != nil { return err }
        if strings.HasPrefix(review.HeadBranch, "fix/") { return nil } // §9 boundary policy
        proj, _ := r.projects.GetByID(ctx, review.ProjectID)
        if proj.AutomationDecision == "manual_only" { return nil }
        for _, f := range review.Findings {
            if !severityAtLeast(f.Severity, proj.AutoCardThreshold) { continue }
            if f.SuggestedPatch == "" && f.Suggestion == "" { continue }
            card := buildCard(f, review)               // ProviderNeutralCard mirror (1D schema)
            corrTokens := make(map[string]string)      // action.id -> token
            for _, a := range card.Actions {
                if a.Type != "callback" { continue }
                tok, _ := r.corr.Mint(ctx, imcards.CorrelationInput{
                    ExecutionID: nil,
                    NodeID:      "(automation)",
                    ActionID:    a.ID,
                    Payload:     map[string]any{"finding_id": f.ID, "action": a.ID},
                })
                corrTokens[a.ID] = tok
            }
            applyTokensToCard(&card, corrTokens)
            r.sender.SendActionCard(ctx, imcards.SendInput{
                ReplyTarget: review.IMReplyTarget, // resolved from project IM channel binding
                Card:        card,
            })
        }
        return nil
    }
    ```
  - `buildCard`: title `Apply fix? — {trim(message,60)}`, severity-derived status, summary = `truncate(suggestion, 240)`, fields `[file:line, severity, sources[0]]`, actions `[{id:"apply", type:"callback", label:"Apply"}, {id:"dismiss", type:"callback", label:"Dismiss"}, {id:"view", type:"url", label:"Open in AgentForge", url:"/reviews/{rid}#"+f.ID}]`.
  - The struct mirrors `ProviderNeutralCard` from 1D (`internal/imcards/card_template.go`) — DO NOT redefine.
  - Run: `rtk go test ./internal/automation -run TestRule_` — green.

- [ ] **Task 9: failing test + impl — card_action_router automation branch**
  - File: `src-go/internal/imcards/router_test.go` (extend, do not duplicate)
  - Add `TestRouter_AutomationBranchDispatchesToHandler`:
    - Given: a correlation row with `execution_id IS NULL`, `payload = {"finding_id": "<uuid>", "action": "apply"}`.
    - Expect: router calls `automationActionHandler.Decide(ctx, findingID, "apply", actorUserID)` exactly once; consumes the correlation row; returns `RouteOutput{Outcome: "automation_dispatched"}`.
  - Add `TestRouter_AutomationBranch_DismissAction`: same shape with `action:"dismiss"`.
  - Run: `rtk go test ./internal/imcards -run TestRouter_AutomationBranch` — fails.
  - Implement: in `src-go/internal/imcards/router.go`, after looking up the correlation, if `corr.ExecutionID == nil`, dispatch to a new injected `AutomationActionHandler` interface:
    ```go
    type AutomationActionHandler interface {
        Decide(ctx context.Context, findingID uuid.UUID, action string, actor string) error
    }
    ```
    Concrete impl in `src-go/internal/handler/findings_decision_handler.go` exposed as `(*FindingsDecisionHandler).DecideInternal(ctx, findingID, action, actor)` (no Echo context — pure service call). Wire it at server boot.
  - Run: `rtk go test ./internal/imcards -run TestRouter_` — green.

### Phase 5 — Canonical code_fixer DAG + role manifest

- [ ] **Task 10: failing test — code_fixer DAG seed registers and is discoverable by name**
  - File: `src-go/internal/workflow/system/code_fixer_dag_test.go`
  - Cases:
    1. `TestSeedCodeFixer_RegistersDefinition`: invoke `system.SeedCodeFixer(ctx, defRepo, templates)`; expect `templates.GetByName(ctx, "code_fixer")` returns a definition with nodes named exactly `[trigger, fetch_file, has_prebaked, generate, validate, decide, execute, update_original_pr, card]`.
    2. `TestSeedCodeFixer_NodeTypesMatchSpec`: assert `fetch_file.type == "http_call"`, `has_prebaked.type == "condition"`, `generate.type == "llm_agent"` with `roleId == "default-code-fixer"`, `validate.type == "function"`, `decide.type == "condition"`, `execute.type == "http_call"`, `update_original_pr.type == "http_call"`, `card.type == "im_send"`.
    3. `TestSeedCodeFixer_PrebakedShortCircuit`: assert `has_prebaked` branches to `validate` when input.suggested_patch present, otherwise to `generate`.
    4. `TestSeedCodeFixer_Idempotent`: calling seed twice doesn't duplicate the definition.
  - Run: `rtk go test ./internal/workflow/system -run TestSeedCodeFixer` — fails.

- [ ] **Task 11: implement `code_fixer_dag.go`**
  - File: `src-go/internal/workflow/system/code_fixer_dag.go`
  - Function `SeedCodeFixer(ctx, defRepo DAGWorkflowDefinitionRepo, templates WorkflowTemplateRegistry) error`.
  - Build a `*model.WorkflowDefinition` with name `"code_fixer"`, `system: true`, `kind: "dag"`. Nodes (config sketches):
    ```
    trigger:           type=trigger,  config={inputs:["review_id","finding_id","integration_id","head_sha","employee_id"]}
    fetch_file:        type=http_call, config={method:"GET", url:"/api/v1/internal/vcs/raw?integration_id={{integration_id}}&sha={{head_sha}}&path={{finding.file}}", auth:"internal"}
    has_prebaked:      type=condition, config={expr:"input.suggested_patch != null && input.suggested_patch != ''"}
    generate:          type=llm_agent, config={roleId:"default-code-fixer", model:"claude-sonnet-4-6", budgetUsd:1.0}
    validate:          type=function,  config={name:"patch_validate", url:"/api/v1/internal/fix-runs/dry-run"}    # wired by 2E
    decide:            type=condition, config={expr:"validate.output.dry_run_ok == true"}
    execute:           type=http_call, config={method:"POST", url:"/api/v1/internal/fix-runs/execute", body_template:"{...}"}  # 2E wires endpoint
    update_original_pr: type=http_call, config={method:"POST", url:"/api/v1/internal/vcs/post-comment", ...}
    card:              type=im_send,   config={template:"fix_result"}
    ```
  - Edges:
    - `trigger → fetch_file → has_prebaked`
    - `has_prebaked --true→ validate` (skip generate when input has prebaked patch)
    - `has_prebaked --false→ generate → validate`
    - `validate → decide`
    - `decide --true→ execute → update_original_pr → card`
    - `decide --false→ card` (failure path, summary=`patch validation failed`)
  - Use placeholder URLs (note in code comment: "Replaced when Plan 2E lands fix_runner endpoints").
  - Wire `SeedCodeFixer` into server boot (search for `SeedBuiltins` or similar in `cmd/server/main.go` or `internal/server/init*.go`; add the call alongside existing seeders).
  - Run: `rtk go test ./internal/workflow/system -run TestSeedCodeFixer` — green.

- [ ] **Task 12: failing test — default-code-fixer role manifest parses + registers**
  - File: `src-go/internal/role/registry_test.go` (extend) OR new `roles/default-code-fixer/role_test.go`
  - Add `TestRegistry_LoadsDefaultCodeFixer`:
    - Load `roles/` via `registry.LoadDir`; expect `registry.Get("default-code-fixer")` returns a manifest with `metadata.id="default-code-fixer"`, `system_prompt` non-empty, `capabilities.tools.built_in` empty (or just `["Read"]`), `capabilities.max_budget_usd <= 2.0`.
  - Run: `rtk go test ./internal/role -run TestRegistry_LoadsDefaultCodeFixer` — fails.

- [ ] **Task 13: create `roles/default-code-fixer/role.yaml`**
  - File: `D:/Project/AgentForge/roles/default-code-fixer/role.yaml`
  - Content (model after `roles/code-reviewer/role.yaml`):
    ```yaml
    apiVersion: agentforge/v1
    kind: Role
    metadata:
      id: default-code-fixer
      name: Default Code Fixer
      version: "1.0.0"
      author: AgentForge
      tags: [fix, patch, automation]
      description: Generates unified diffs that resolve a single review finding.
    identity:
      role: Patch Generator
      goal: Produce a minimal unified diff that resolves the supplied finding.
      backstory: You are a focused refactoring agent — read the file, read the finding, emit a patch.
      persona: Surgical, precise, no-prose
      goals:
        - Emit a syntactically valid unified diff
        - Touch only the lines necessary to resolve the finding
      constraints:
        - Never include speculative refactors
        - Never invent files that do not exist
      personality: surgical
      language: zh-CN
      response_style:
        tone: terse
        verbosity: minimal
        format_preference: diff
    system_prompt: |
      You receive: (1) the contents of a single source file, (2) one review finding
      with file/line/message/suggestion. You output ONLY a unified diff (--- / +++ / @@)
      that resolves the finding. No prose, no commentary. If you cannot resolve the
      finding without ambiguity, return an empty patch.
    capabilities:
      packages: []
      tools:
        built_in: []
        external: []
      max_turns: 4
      max_budget_usd: 1.0
    knowledge:
      repositories: []
      documents: []
      patterns: []
    security:
      profile: standard
      permission_mode: readonly
      allowed_paths: ["*"]
      output_filters:
        - no_credentials
    collaboration:
      accepts_delegation_from:
        - default-code-reviewer
      communication:
        preferred_channel: structured
        report_format: diff
    ```
  - Run: `rtk go test ./internal/role -run TestRegistry_LoadsDefaultCodeFixer` — green.

### Phase 6 — Frontend: diff viewer + per-finding actions

- [ ] **Task 14: add `react-diff-viewer-continued` dependency + smoke test**
  - Run: `rtk pnpm add react-diff-viewer-continued@latest`.
  - Pin the version in `package.json`. Confirm `pnpm-lock.yaml` updates and `rtk pnpm install` is clean.
  - File: `components/review/finding-patch-modal.test.tsx` (new):
    - `renders patch text inside diff viewer`: mount `<FindingPatchModal patch={"--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b"} open onClose={...} />`; assert it renders the diff (any line containing `-a` and `+b`).
    - `shows empty state when patch is null`.
  - Run: `rtk vitest run components/review/finding-patch-modal` — fails.

- [ ] **Task 15: implement `finding-patch-modal.tsx` + extend `review-findings-table.tsx` with action buttons**
  - File: `components/review/finding-patch-modal.tsx`
    - shadcn `<Dialog>` containing `<ReactDiffViewer oldValue={originalText} newValue={patchedText} splitView />`. For Phase 6 we render the raw unified diff text (parse the `@@` hunks client-side via a tiny helper — `lib/utils/parse-unified-diff.ts`) inside the diff viewer.
  - File: `components/review/review-findings-table.tsx`
    - Add columns:
      - **Decision**: badge derived from `finding.decision` (`pending|approved|dismissed|deferred|needs_manual_fix`).
      - **Actions**: three icon buttons (Approve/Dismiss/Defer). When `finding.suggested_patch` truthy, show a fourth "Show patch" link that opens `<FindingPatchModal>`.
    - Each button calls `useReviewStore().decideFinding(findingId, action)` (new store action) ⇒ `POST /api/v1/findings/:id/decision` ⇒ on success, toast (`sonner`) + refresh review.
  - File: `lib/stores/review-store.ts` — add `decideFinding(findingId: string, action: "approve"|"dismiss"|"defer", comment?: string): Promise<void>`. Use the shared backend URL resolver.
  - Test (`components/review/review-findings-table.test.tsx` — extend):
    - `TestApproveButton_FiresPOST`: click Approve ⇒ `fetch` mock receives `POST /findings/:id/decision {action:"approve"}`; on 202 toast appears.
    - `TestDismissButton_FiresPOST`: same with dismiss.
    - `TestDeferButton_FiresPOST`: same with defer.
    - `TestShowPatch_OpensModalWhenPatchPresent`.
    - `TestShowPatch_HiddenWhenPatchAbsent`.
  - Run: `rtk vitest run components/review/finding-patch-modal components/review/review-findings-table` — green.

- [ ] **Task 16: new per-finding detail page `/reviews/[id]/findings/[fid]`**
  - Files:
    - `app/(dashboard)/reviews/[id]/findings/[fid]/page.tsx`
    - `app/(dashboard)/reviews/[id]/findings/[fid]/page.test.tsx`
  - Page shows:
    1. Finding metadata header (file, line, severity, sources, decision badge).
    2. Full patch preview using `<FindingPatchModal>`'s diff viewer inline (or extract `<DiffPanel>`).
    3. Fix run history table — fetches `GET /api/v1/findings/:fid/fix-runs` (read-only; this endpoint is added by Plan 2E). For now, render `<EmptyState title="No fix runs yet" />` if 404; do NOT block this plan on 2E. Add a TODO comment referencing the dependency.
  - Test:
    - Renders metadata + diff panel when finding has `suggested_patch`.
    - Renders empty fix-runs state when API returns 404.
    - Shows `Approve/Dismiss/Defer` buttons reusing the same store action.
  - Run: `rtk vitest run "app/(dashboard)/reviews/\\[id\\]/findings"` — green.

### Phase 7 — Integration test (cross-component)

- [ ] **Task 17: integration test — review.completed → automation card → click Apply → spawn execution**
  - File: `src-go/internal/server/integration_findings_decision_test.go`
  - Setup: real eventbus, in-memory repos, fake IM Bridge HTTP recorder, fake `DAGWorkflowService` that records `Start("code_fixer", ...)` calls.
  - Flow:
    1. Insert a project with `automation_decision="auto_send"` and `auto_card_threshold="medium"`.
    2. Insert a review with `head_branch="feature/foo"` and one finding `severity="high", suggested_patch="--- ..."`.
    3. Publish `EventReviewCompleted`.
    4. Assert: IM Bridge recorder captured one card payload; one `card_action_correlations` row exists with `execution_id IS NULL`, `payload.finding_id == finding.ID`.
    5. Simulate user click: `POST /api/v1/im/card-actions` with the captured token + `action_id="apply"`.
    6. Assert: `DAGWorkflowService.Start` was called with name `"code_fixer"` and seed containing `{review_id, finding_id, integration_id, head_sha}`; finding.Decision becomes `"approved"`.
  - Run: `rtk go test ./internal/server -run TestIntegration_FindingsDecisionLoop` — green.

### Phase 8 — Verification

- [ ] **Task 18: full-stack verification + lint**
  - Run sequentially:
    1. `rtk go test ./...` — entire Go suite green.
    2. `rtk pnpm test` — Jest/Vitest green.
    3. `rtk tsc --noEmit` — no TS errors.
    4. `rtk lint` — no new violations.
    5. `rtk pnpm --filter @agentforge/bridge test` — bridge SDK green.
  - Manual smoke (only if web stack is running):
    1. Trigger a review with a `suggested_patch` finding via SDK fixture.
    2. Confirm IM Bridge log shows one outbound card.
    3. Open `/reviews/<id>` — diff viewer renders, Approve button POSTs and returns 202.
  - Document any deviations in plan footer; do NOT mark task done until each command above is green.

---

## Out of scope (handled elsewhere)

- `fix_runs` table writes / fix_runner endpoints — Plan 2E.
- `vcs.Provider.GetRawFile` internal endpoint impl — Plan 2A.
- 1E `imcards.SendActionCard` helper signature — Plan 1E.
- `card_action_correlations` schema — Plan 1E.
- Project-level `automation_decision` / `auto_card_threshold` columns — Plan 2B.
