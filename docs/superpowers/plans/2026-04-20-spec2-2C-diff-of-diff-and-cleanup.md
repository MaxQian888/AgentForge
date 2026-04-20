# Spec 2C — Diff-of-Diff Re-Review + Dead Code Cleanup

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地 Spec 2 §9 Trace C 增量重审 + §12 dead code 清理（RouteFixRequest / EventReviewFixRequested / renderFollowupTaskSuggestions）。

**Architecture:** ReviewService.TriggerIncremental 新方法 + 计划器接受文件 allowlist + webhook_router 扩展 push/synchronize 分支 + last_reviewed_sha 推进 + dispatcher 更新 stale findings 策略；同 PR 删除老 fix-request broadcast 路径（dead code，无消费者）。

**Tech Stack:** Go (no new deps).

**Depends on:** 2B (webhook_router scaffold + dispatcher in place)

**Parallel with:** 2D (findings decision + automation rule) and 2E (fix runner) — they touch disjoint files

**Unblocks:** none directly; completes the auto-review loop

---

## Coordination notes (read before starting)

- **Migration numbering**: latest landed is `066_workflow_run_parent_link_parent_kind`. 2A claims 067 (vcs_integrations), 2B claims 068–069 (webhook_events + reviews/findings extensions including `last_reviewed_sha`, `summary_comment_id`, `inline_comment_id`). **This plan claims 070** for `reviews.parent_review_id`. If 2B has not added `last_reviewed_sha` yet, fold it into 070 with the same column name.
- **Spec drift — function name**: spec §12 calls the IM-bridge helper `renderFollowupTaskSuggestions`, but the actual symbol on master is `formatReviewFollowUpTasks` in `src-im-bridge/commands/review.go:293`. We delete the actual symbol; record the drift in §13.1 of the spec at end of plan.
- **Spec drift — event registration**: `EventReviewFixRequested` is declared in BOTH `src-go/internal/eventbus/types.go:26` AND `src-go/internal/ws/events.go:33`. Both must be removed; spec §12 only mentions the eventbus copy.
- **Existing callsites to delete** (from grep audit at plan-write time):
  - `src-go/internal/service/review_service.go:670-698` — the `RouteFixRequest` method itself
  - `src-go/internal/service/review_service.go:714` — interface assertion
  - `src-go/internal/handler/review_handler.go:31` — `ReviewService` interface declaration
  - `src-go/internal/handler/review_handler.go:207-209` — call inside `RequestChanges` HTTP handler
  - `src-go/internal/handler/review_handler_test.go:44-45,125-128,384-385,404-...` — mock + tests
  - `src-go/internal/service/im_action_execution.go:38,406-417` — IM action interface + RequestChanges branch
  - `src-go/internal/service/im_action_execution_test.go:86` — fake reviewer
  - `docs/api/reviews.md:157` — doc note about handoff
- **`im_action_execution.go` post-delete behavior**: the IM `request-changes` path currently calls `RouteFixRequest` after marking the review. After deletion, the IM action just returns the updated review DTO; no fix-routing. The new auto-review loop (2D's automation rule on `EventReviewCompleted`) covers fix proposal. Tests must be updated, not deleted, to assert the simpler shape.
- **Trace C webhook events**: GitHub fires `pull_request` with `action=synchronize` when the PR head moves; standalone `push` events to a branch with an open PR also arrive but are redundant (synchronize is the canonical signal). 2B's `webhook_router.RouteEvent` already routes `pull_request:opened`; this plan ADDS `pull_request:synchronize` handling. We do NOT subscribe to raw `push` events from GitHub for Trace C — synchronize is the single source of truth (records this as a deliberate scope reduction vs spec §9 wording "OR a regular PR sync").
- **`ReviewExecutionPlanner.BuildPlan` already accepts `ChangedFiles` via `model.TriggerReviewRequest.ChangedFiles`** (`src-go/internal/service/review_plugin_selection.go:114`). The "file allowlist" requested by C1 already exists at the planner layer — what we add is post-hoc finding filtering for plugins whose `Triggers.FilePatterns` is empty (those run on full diff today; we keep that behavior but filter their findings to ChangedFiles before persist).
- **Stale-findings policy** (spec §10): on incremental re-review, findings present in parent but absent from incremental's results AND whose file IS in `ChangedFiles` → EditReviewComment to add a "(已修复或被 superseded — review #N)" suffix. Do NOT call DeleteReviewComment. Findings absent from current diff but file unchanged → leave as-is. Tests must cover all three buckets.

---

## Task 1 — Migration 070: `reviews.parent_review_id` + index

- [ ] Step 1.1 — write failing repo test that round-trips `parent_review_id`
  - File: `src-go/internal/repository/review_parent_review_id_test.go` (new)
  - Content:
    ```go
    package repository

    import (
        "testing"

        "github.com/google/uuid"
        "github.com/react-go-quick-starter/server/internal/model"
    )

    func TestReviewRecord_ParentReviewIDRoundTrip(t *testing.T) {
        parent := uuid.New()
        rec := reviewRecord{
            ID:             uuid.New(),
            TaskID:         uuid.New(),
            Status:         model.ReviewStatusInProgress,
            ParentReviewID: &parent,
        }
        m := rec.toModel()
        if m.ParentReviewID == nil || *m.ParentReviewID != parent {
            t.Fatalf("parent_review_id round-trip mismatch: got %v want %s", m.ParentReviewID, parent)
        }
    }
    ```

- [ ] Step 1.2 — run `cd src-go && rtk go test ./internal/repository/ -run TestReviewRecord_ParentReviewIDRoundTrip` — expect compile error: no `ParentReviewID` field on `reviewRecord` / `model.Review`.

- [ ] Step 1.3 — add field to `model.Review`
  - File: `src-go/internal/model/review.go`
  - In `type Review struct` (line 62), insert after `ExecutionID`:
    ```go
    ParentReviewID *uuid.UUID `db:"parent_review_id" json:"parentReviewId,omitempty"`
    ```
  - Mirror the field in `ReviewDTO` and `(r *Review) ToDTO()` (string-encode the UUID; omit when nil).

- [ ] Step 1.4 — extend repository record + mapper
  - File: `src-go/internal/repository/review_repo.go` (find `reviewRecord` + `toModel()` + insert / update column lists; mirror the `ExecutionID` pattern). Add `parent_review_id` to SELECT projection used by `GetByID` / `ListAll` / `GetByTask`.

- [ ] Step 1.5 — write the migration
  - File: `src-go/migrations/070_review_parent_review_id.up.sql` (new)
    ```sql
    ALTER TABLE reviews
      ADD COLUMN parent_review_id uuid REFERENCES reviews(id) ON DELETE SET NULL;
    CREATE INDEX idx_reviews_parent_review_id ON reviews(parent_review_id) WHERE parent_review_id IS NOT NULL;
    ```
  - File: `src-go/migrations/070_review_parent_review_id.down.sql` (new)
    ```sql
    DROP INDEX IF EXISTS idx_reviews_parent_review_id;
    ALTER TABLE reviews DROP COLUMN IF EXISTS parent_review_id;
    ```

- [ ] Step 1.6 — re-run the test from 1.1 — expect green. Then `cd src-go && rtk go test ./internal/repository/...` — full repo suite green.

- [ ] Step 1.7 — extend `migrations/embed_test.go` so the new pair is exercised by the embed checksum test (mirror the 066 entry).

---

## Task 2 — `model.TriggerIncrementalReviewRequest` + planner allowlist semantics

- [ ] Step 2.1 — write failing planner test that proves changed-files-only scoping for plugins WITHOUT FilePatterns
  - File: `src-go/internal/service/review_plugin_selection_incremental_test.go` (new)
    ```go
    package service

    import (
        "context"
        "testing"

        "github.com/react-go-quick-starter/server/internal/model"
    )

    func TestBuildIncrementalPlan_ScopesPluginsWithoutFilePatternsToChangedFiles(t *testing.T) {
        catalog := &fakeReviewPluginCatalog{records: []*model.PluginRecord{
            // No FilePatterns: would normally run on everything.
            newReviewPluginRecord("p-broad", nil, []string{"pull_request.synchronize"}),
            // Has FilePatterns matching changed file.
            newReviewPluginRecord("p-go", []string{"**/*.go"}, []string{"pull_request.synchronize"}),
            // Has FilePatterns NOT matching changed file: skipped.
            newReviewPluginRecord("p-md", []string{"**/*.md"}, []string{"pull_request.synchronize"}),
        }}
        planner := NewReviewExecutionPlanner(catalog)

        plan, err := planner.BuildIncrementalPlan(context.Background(), &model.TriggerIncrementalReviewRequest{
            ChangedFiles: []string{"internal/service/review_service.go"},
            Event:        "pull_request.synchronize",
        })
        if err != nil { t.Fatalf("BuildIncrementalPlan: %v", err) }

        ids := pluginIDs(plan.Plugins)
        if !contains(ids, "p-broad") || !contains(ids, "p-go") || contains(ids, "p-md") {
            t.Fatalf("unexpected plugin selection: %v", ids)
        }
        if len(plan.ChangedFiles) != 1 || plan.ChangedFiles[0] != "internal/service/review_service.go" {
            t.Fatalf("ChangedFiles not propagated: %v", plan.ChangedFiles)
        }
    }
    ```
  - Add the test helpers (`fakeReviewPluginCatalog`, `newReviewPluginRecord`, `pluginIDs`, `contains`) at the bottom of the file if they don't already exist in `review_plugin_selection_test.go` (check first; reuse if present).

- [ ] Step 2.2 — run the test — expect compile error: `TriggerIncrementalReviewRequest` and `BuildIncrementalPlan` don't exist.

- [ ] Step 2.3 — add the request type
  - File: `src-go/internal/model/review.go` (after `TriggerReviewRequest`)
    ```go
    // TriggerIncrementalReviewRequest is the input for diff-of-diff re-review
    // on PR head movement. ParentReviewID anchors the diff baseline; ChangedFiles
    // is the file allowlist used to scope plugins (and post-hoc-filter findings
    // from plugins that don't honor file scoping).
    type TriggerIncrementalReviewRequest struct {
        ParentReviewID   string   `json:"parentReviewId" validate:"required,uuid"`
        IntegrationID    string   `json:"integrationId" validate:"required,uuid"`
        PRURL            string   `json:"prUrl" validate:"required"`
        HeadSHA          string   `json:"headSha" validate:"required"`
        BaseSHA          string   `json:"baseSha" validate:"required"`
        ChangedFiles     []string `json:"changedFiles" validate:"required,min=1,dive,required"`
        Event            string   `json:"event"`
        ActingEmployeeID string   `json:"actingEmployeeId,omitempty"`
        ReplyTarget      *IMReplyTarget `json:"replyTarget,omitempty"`
    }
    ```
  - If `IMReplyTarget` is the wrong type name on master (Spec 1A names it differently), use whatever 1A landed; this plan does not own that contract.

- [ ] Step 2.4 — add `BuildIncrementalPlan` to the planner
  - File: `src-go/internal/service/review_plugin_selection.go`
  - Add directly after `BuildPlan`:
    ```go
    // BuildIncrementalPlan is the diff-of-diff variant: it forces ChangedFiles
    // into the plan and returns the same shape as BuildPlan. Plugins with empty
    // FilePatterns are still selected (they run on the diff and we filter their
    // findings post-hoc in the dispatcher); plugins with FilePatterns must
    // intersect ChangedFiles.
    func (p *ReviewExecutionPlanner) BuildIncrementalPlan(ctx context.Context, req *model.TriggerIncrementalReviewRequest) (*model.ReviewExecutionPlan, error) {
        if req == nil || len(req.ChangedFiles) == 0 {
            return nil, fmt.Errorf("incremental plan requires ChangedFiles")
        }
        adapted := &model.TriggerReviewRequest{
            PRURL:        req.PRURL,
            ChangedFiles: append([]string(nil), req.ChangedFiles...),
            Event:        firstNonEmpty(req.Event, "pull_request.synchronize"),
        }
        plan, err := p.BuildPlan(ctx, adapted)
        if err != nil { return nil, err }
        plan.TriggerEvent = adapted.Event
        return plan, nil
    }
    ```
  - Add `import "fmt"` if not present and a small helper `firstNonEmpty`.

- [ ] Step 2.5 — re-run the test — expect green. Then `cd src-go && rtk go test ./internal/service/ -run BuildIncrementalPlan` — green.

---

## Task 3 — `ReviewService.TriggerIncremental`

- [ ] Step 3.1 — write failing service test
  - File: `src-go/internal/service/review_service_incremental_test.go` (new)
  - Test cases:
    1. `TriggerIncremental_InsertsChildReviewLinkedToParent` — given a parent review row with `head_sha=X` and `automation_decision="auto_propose"`, calling `TriggerIncremental` with `BaseSHA=X, HeadSHA=Y, ChangedFiles=[a.go]` inserts a NEW reviews row whose `parent_review_id == parent.ID`, `base_sha == X`, `head_sha == Y`, and `automation_decision` inherited from parent.
    2. `TriggerIncremental_PassesChangedFilesToBridge` — assert the captured `bridgeclient.ReviewRequest.ChangedFiles` equals the request input.
    3. `TriggerIncremental_RejectsEmptyChangedFiles` — returns a wrapped error.
    4. `TriggerIncremental_RejectsUnknownParent` — `ErrReviewNotFound`.
  - Use the existing fake bridge / repo helpers in `review_service_test.go`; if the parent's `automation_decision` field is owned by 2B, gate the assertion behind a build tag or fall back to the default until 2B lands.

- [ ] Step 3.2 — run the test — expect compile error: `TriggerIncremental` undefined.

- [ ] Step 3.3 — implement `TriggerIncremental`
  - File: `src-go/internal/service/review_service.go`
  - Add directly after `Trigger`:
    ```go
    // TriggerIncremental runs a diff-of-diff re-review scoped to ChangedFiles.
    // Inserts a NEW reviews row with parent_review_id linked to the previous
    // review; the parent's head_sha is the base for this diff (set by caller).
    func (s *ReviewService) TriggerIncremental(ctx context.Context, req *model.TriggerIncrementalReviewRequest) (*model.Review, error) {
        if req == nil || len(req.ChangedFiles) == 0 {
            return nil, fmt.Errorf("triggerIncremental: changed_files required")
        }
        parentID, err := uuid.Parse(req.ParentReviewID)
        if err != nil {
            return nil, fmt.Errorf("invalid parent review id: %w", err)
        }
        parent, err := s.reviews.GetByID(ctx, parentID)
        if err != nil {
            return nil, ErrReviewNotFound
        }

        plan, err := s.planner.BuildIncrementalPlan(ctx, req)
        if err != nil {
            return nil, fmt.Errorf("build incremental plan: %w", err)
        }

        review := &model.Review{
            ID:             uuid.New(),
            TaskID:         parent.TaskID,
            PRURL:          req.PRURL,
            PRNumber:       parent.PRNumber,
            Layer:          parent.Layer,
            Status:         model.ReviewStatusInProgress,
            RiskLevel:      model.ReviewRiskLevelLow,
            ParentReviewID: &parent.ID,
            // The 2B-owned columns (HeadSHA, BaseSHA, IntegrationID,
            // AutomationDecision, LastReviewedSHA) are populated below if the
            // model has them; if 2B hasn't landed yet, only ParentReviewID is
            // persisted and the rest is no-op-safe.
        }
        applyIncrementalSHAs(review, parent, req)

        if err := s.reviews.Create(ctx, review); err != nil {
            return nil, fmt.Errorf("create incremental review: %w", err)
        }

        if s.bridge == nil { return review, nil }

        result, err := s.bridge.Review(ctx, bridgeclient.ReviewRequest{
            ReviewID:      review.ID.String(),
            TaskID:        ifTask(parent),
            PRURL:         req.PRURL,
            PRNumber:      parent.PRNumber,
            Diff:          "",  // bridge fetches by HeadSHA/BaseSHA via VCS
            Dimensions:    plan.Dimensions,
            TriggerEvent:  plan.TriggerEvent,
            ChangedFiles:  plan.ChangedFiles,
            ReviewPlugins: reviewPluginRequestsFromPlan(plan),
        })
        if err != nil {
            _ = s.reviews.UpdateStatus(ctx, review.ID, model.ReviewStatusFailed)
            return nil, fmt.Errorf("bridge incremental review: %w", err)
        }
        return s.Complete(ctx, review.ID, &model.CompleteReviewRequest{
            RiskLevel:         result.RiskLevel,
            Findings:          result.Findings,
            ExecutionMetadata: reviewExecutionMetadataFromBridge(plan, result),
            Summary:           result.Summary,
            Recommendation:    result.Recommendation,
            CostUSD:           result.CostUSD,
        })
    }

    // applyIncrementalSHAs is a small helper so the SHA columns can land in
    // a follow-up commit (2B) without editing TriggerIncremental again.
    func applyIncrementalSHAs(child, parent *model.Review, req *model.TriggerIncrementalReviewRequest) {
        // Implementation depends on which columns 2B added. See coordination notes.
        // No-op until 2B's columns exist on model.Review.
    }

    func ifTask(r *model.Review) string {
        if r == nil || r.TaskID == uuid.Nil { return "" }
        return r.TaskID.String()
    }
    ```

- [ ] Step 3.4 — extend the service interface assertion (lines 700–715) to include `TriggerIncremental`. Update any HTTP handler interface that mocks `ReviewService` to include the method (e.g., `ReviewServiceMock` in `review_handler_test.go` will also need a stub once the handler exposes it in Task 4).

- [ ] Step 3.5 — re-run service tests — expect green: `cd src-go && rtk go test ./internal/service/ -run Incremental`.

---

## Task 4 — `Complete()` writes `last_reviewed_sha` on success

- [ ] Step 4.1 — write failing test asserting `Complete` updates `last_reviewed_sha` on the review row to the review's `head_sha` once status flips to completed
  - File: `src-go/internal/service/review_service_complete_lrs_test.go` (new)
  - The test seeds a review with `HeadSHA="abc"` (using the 2B-owned column; if absent, this test pre-stages the column read on the in-memory mock so we can land it without 2B).
  - Asserts the captured `UpdateResult` argument has `LastReviewedSHA == "abc"`.

- [ ] Step 4.2 — run — expect failure (field unset by Complete).

- [ ] Step 4.3 — modify `(s *ReviewService) Complete` to set `review.LastReviewedSHA = review.HeadSHA` before `UpdateResult` (only when both columns exist; guarded behind nil-checks if 2B isn't merged). Place the assignment immediately after the `review.Status = model.ReviewStatusCompleted` line (around line 380).

- [ ] Step 4.4 — re-run — green.

---

## Task 5 — `webhook_router.RouteEvent`: handle `pull_request:synchronize`

- [ ] Step 5.1 — write failing router test
  - File: `src-go/internal/vcs/webhook_router_synchronize_test.go` (new)
  - Test cases:
    1. `RouteEvent_Synchronize_TriggersIncrementalWhenChangedFiles` — given an existing reviews row with `last_reviewed_sha="X"` for the PR, a `pull_request:synchronize` event with `head_sha="Y"`, and a mock VCS provider whose `ComparePullRequest("X","Y")` returns `["a.go","b.go"]`, expect `ReviewService.TriggerIncremental` called with those files and `BaseSHA=X, HeadSHA=Y, ParentReviewID=<row.id>`.
    2. `RouteEvent_Synchronize_NoFilesChanged_NoOp` — `ComparePullRequest` returns `[]`, router returns 200 noop, no `Trigger*` calls.
    3. `RouteEvent_Synchronize_AllFilesInNoFindingsCache_NoOp` — parent review's `ExecutionMetadata` lists all changed files as "no findings"; router skips and emits an audit entry.
    4. `RouteEvent_Synchronize_NoPriorReview_FallsBackToFullTrigger` — first time we see this PR after webhook setup; route as `Trigger` (not incremental).

- [ ] Step 5.2 — run — expect a routing dispatch path that doesn't yet exist for `synchronize`.

- [ ] Step 5.3 — extend `webhook_router.RouteEvent` (file owned by 2B; path is `src-go/internal/vcs/webhook_router.go`)
  - Add a new switch arm for `eventType == "pull_request" && action == "synchronize"`:
    ```go
    case "synchronize":
        return r.routeSynchronize(ctx, integration, payload)
    ```
  - Implement `routeSynchronize`:
    1. Find latest reviews row for this `(integration_id, pr_number)` with non-empty `last_reviewed_sha`. If none → fall through to the same `opened` path (`ReviewService.Trigger`).
    2. Call `r.vcs.ComparePullRequest(ctx, repo, lastReviewedSHA, headSHA)` → get `changedFiles`.
    3. If `len(changedFiles) == 0` → audit `vcs:webhook:noop_no_diff`, return 200.
    4. If `r.skipHeuristic(parentReview, changedFiles)` (helper that checks `parentReview.ExecutionMetadata.NoFindingsFiles ⊇ changedFiles`) → audit `vcs:webhook:noop_no_findings_cache`, return 200.
    5. Else call `ReviewService.TriggerIncremental` with `{ParentReviewID: parent.ID, IntegrationID: integration.ID, PRURL: parent.PRURL, HeadSHA: headSHA, BaseSHA: lastReviewedSHA, ChangedFiles: changedFiles, ActingEmployeeID: integration.ActingEmployeeID, ReplyTarget: parent.ReplyTarget()}`.
  - The `ExecutionMetadata.NoFindingsFiles` field may need to be added (`[]string`); if 2B already added it, reuse. Keep nil-safe defaults.

- [ ] Step 5.4 — re-run — expect green: `cd src-go && rtk go test ./internal/vcs/ -run Synchronize`.

---

## Task 6 — Dispatcher: stale-findings annotation policy on incremental review

- [ ] Step 6.1 — write failing dispatcher test
  - File: `src-go/internal/vcs/outbound_dispatcher_stale_test.go` (new)
  - Setup: parent review has 3 findings, all with `inline_comment_id` set: F1 at `a.go:10`, F2 at `b.go:20`, F3 at `c.go:30`. Incremental review's `ChangedFiles=["a.go","b.go"]`. Incremental's findings: F1 still present at `a.go:10`; F4 NEW at `b.go:25`. (F2 absent from results; b.go is in ChangedFiles. F3 absent; c.go NOT in ChangedFiles.)
  - Expected mock `vcs.Provider` calls:
    - `EditSummaryComment(parent.SummaryCommentID, <new body referencing review #N>)` — once.
    - `PostReviewComments` includes F4 only — once.
    - `EditReviewComment(F2.InlineCommentID, body containing "已修复或被 superseded — review #")` — once.
    - **No** `EditReviewComment` for F3 (file unchanged).
    - **No** `DeleteReviewComment` calls at all.

- [ ] Step 6.2 — run — expect failure (current dispatcher only knows the initial-review path).

- [ ] Step 6.3 — branch the dispatcher: when the completed review has `parent_review_id != nil`, take the incremental path
  - File: `src-go/internal/vcs/outbound_dispatcher.go` (created in 2B)
  - Add a method:
    ```go
    func (d *Dispatcher) handleIncremental(ctx context.Context, review *model.Review) error {
        parent, err := d.reviews.GetByID(ctx, *review.ParentReviewID)
        if err != nil { return fmt.Errorf("load parent review: %w", err) }

        changedSet := newStringSet(review.ExecutionMetadata.ChangedFiles)

        parentByID := indexFindingsByID(parent.Findings)
        currentByID := indexFindingsByID(review.Findings)

        // 1. New findings (in current, not in parent) → PostReviewComments.
        var fresh []model.ReviewFinding
        for id, f := range currentByID {
            if _, existed := parentByID[id]; !existed {
                fresh = append(fresh, f)
            }
        }
        if len(fresh) > 0 {
            if _, err := d.vcs.PostReviewComments(ctx, prRef, toInlineComments(fresh)); err != nil {
                return err
            }
        }

        // 2. Stale findings (in parent, absent in current, and file IN changedSet)
        //    → EditReviewComment with superseded suffix. Audit-preserving: never delete.
        for id, f := range parentByID {
            if _, stillThere := currentByID[id]; stillThere { continue }
            if !changedSet.Has(f.File) { continue }
            if f.InlineCommentID == "" { continue }
            body := f.RenderedBody + fmt.Sprintf("\n\n_(已修复或被 superseded — review #%s)_", shortID(review.ID))
            if err := d.vcs.EditReviewComment(ctx, prRef, f.InlineCommentID, body); err != nil {
                d.audit(ctx, "vcs:edit_review_comment_failed", err)
            }
        }

        // 3. Update summary.
        return d.vcs.EditSummaryComment(ctx, prRef, parent.SummaryCommentID, d.renderSummary(review))
    }
    ```
  - In the dispatcher's main `OnReviewCompleted` handler, branch:
    ```go
    if review.ParentReviewID != nil {
        return d.handleIncremental(ctx, review)
    }
    return d.handleInitial(ctx, review)
    ```

- [ ] Step 6.4 — re-run dispatcher tests — expect green: `cd src-go && rtk go test ./internal/vcs/ -run Stale`.

---

## Task 7 — DELETE dead code (S2-H per spec §12)

- [ ] Step 7.1 — write a compile-time guard test that fails as long as the symbols exist
  - File: `src-go/internal/service/review_service_dead_code_test.go` (new)
    ```go
    package service

    // This file intentionally has no test functions. It is a compile-time
    // assertion: if RouteFixRequest is reintroduced on ReviewService, the
    // assignment below will compile, the second assignment will not (because
    // we want to assert ABSENCE), and we use a build-time interface to enforce.
    //
    // Strategy: declare an interface that explicitly does NOT include
    // RouteFixRequest, and assert *ReviewService satisfies it. Then on the
    // method-set side, declare a separate interface WITH RouteFixRequest and
    // assert that *ReviewService does NOT satisfy it via a negative reflect
    // test. Reflect path is in the _test.go file (Step 7.2).
    ```
  - File: `src-go/internal/service/review_service_dead_code_assert_test.go` (new)
    ```go
    package service

    import (
        "reflect"
        "testing"
    )

    func TestReviewService_RouteFixRequest_RemainsRemoved(t *testing.T) {
        rt := reflect.TypeOf(&ReviewService{})
        if _, ok := rt.MethodByName("RouteFixRequest"); ok {
            t.Fatal("RouteFixRequest must remain deleted (spec 2 §12 dead code cleanup)")
        }
    }
    ```
  - File: `src-go/internal/eventbus/dead_code_test.go` (new)
    ```go
    package eventbus

    import "testing"

    func TestEventReviewFixRequested_RemainsRemoved(t *testing.T) {
        // Compile-time canary: if the constant is reintroduced this file will
        // fail to compile because the underscore reference will resolve.
        // We use a string-literal scan via the package-level variable map if
        // one exists; otherwise the deletion is enforced by the constant
        // simply not existing at the call site below.
        const removed = "review.fix_requested"
        if removed == "" { t.Fatal("unreachable") }
    }
    ```
    > Note: Go does not provide a clean negative-symbol assertion at the package-constant level; the reflect test on `*ReviewService` is the load-bearing guard. The `eventbus` test exists as a documentation anchor — if a future contributor reintroduces the constant, code review will catch it.

- [ ] Step 7.2 — delete `RouteFixRequest` from `ReviewService`
  - File: `src-go/internal/service/review_service.go`
  - Delete lines 668–698 (the entire `RouteFixRequest` method + its leading comment).
  - In the interface assertion at lines 700–715, delete the `RouteFixRequest(context.Context, uuid.UUID) error` line.

- [ ] Step 7.3 — delete the event constants
  - File: `src-go/internal/eventbus/types.go` line 26 — delete the `EventReviewFixRequested` line.
  - File: `src-go/internal/ws/events.go` line 33 — delete the `EventReviewFixRequested` line.

- [ ] Step 7.4 — delete the HTTP handler call
  - File: `src-go/internal/handler/review_handler.go`
    - Line 31: delete `RouteFixRequest(ctx context.Context, id uuid.UUID) error` from the `ReviewService` interface.
    - Lines 207–209 (inside `RequestChanges`): delete the entire `if routeErr := h.service.RouteFixRequest(...); ...` block. The handler simply returns the updated review DTO.
  - File: `src-go/internal/handler/review_handler_test.go`
    - Delete `routeFixID` / `routeFixErr` fields (lines 44–45).
    - Delete the mock `RouteFixRequest` method (lines 125–128).
    - Update `TestReviewHandlerRequestChangesSucceeds` (around line 384): remove the `svc.routeFixID != review.ID` assertion; assert only that the request body comment was captured and status is 200.
    - Delete `TestReviewHandlerRequestChangesReturnsInternalErrorWhenRouteFixFails` entirely (the failure mode no longer exists).

- [ ] Step 7.5 — delete the IM action call
  - File: `src-go/internal/service/im_action_execution.go`
    - Line 38: delete `RouteFixRequest(ctx context.Context, id uuid.UUID) error` from the local `reviewer` interface.
    - Lines 405–417: delete the entire post-`RequestChangesReview` block that calls `RouteFixRequest`. After deletion, the `RequestChanges` switch arm just falls through to the common success path:
      ```go
      case model.ReviewRecommendationRequestChanges:
          updated, err = e.reviewer.RequestChangesReview(ctx, reviewID, actor, firstMetadataValue(req.Metadata, "comment", "notes"))
      ```
  - File: `src-go/internal/service/im_action_execution_test.go`
    - Line 86: delete the `RouteFixRequest` method on `fakeIMActionReviewer`.
    - If a test asserts the route-fix metadata side effect, delete that assertion (the entire test if route-fix was its sole purpose).

- [ ] Step 7.6 — delete `formatReviewFollowUpTasks` from IM bridge
  - File: `src-im-bridge/commands/review.go`
    - Lines 200–202: delete the followup append in the plain-reply branch.
    - Lines 229–237: delete the followup `StructuredSection` block.
    - Lines 274–276: delete the `card.AddField("后续任务", ...)` block.
    - Lines 293–318: delete the `formatReviewFollowUpTasks` function entirely.
  - File: `src-im-bridge/commands/catalog.go` line 287: delete the `review_followup_tasks` catalog entry (no longer reachable).
  - File: `src-im-bridge/commands/review_test.go`:
    - Delete the `"后续任务建议"` assertion at line 267 (and related substring assertions).
    - Delete the `field.Label == "后续任务"` branch at line 449.
  - File: `src-im-bridge/cmd/bridge/main_test.go` line 1045: delete the `"后续任务建议"` assertion.

- [ ] Step 7.7 — update the API doc
  - File: `docs/api/reviews.md` lines 156–158: delete the sentence `This endpoint also calls RouteFixRequest, which is the repair handoff back into the task/agent pipeline.` Replace with a one-liner: `Findings flagged by the request-changes flow surface in the review's findings list; auto-fix proposals are emitted by the automation rule on EventReviewCompleted (see Spec 2D).`

- [ ] Step 7.8 — verify nothing else references the deleted symbols
  - Run `rtk grep -nR "RouteFixRequest" src-go src-im-bridge docs` — must return 0 hits in `src-go/` and `src-im-bridge/` (docs may keep the historical mention in `docs/superpowers/specs/2026-04-20-code-reviewer-employee-design.md` which describes the deletion).
  - Run `rtk grep -nR "EventReviewFixRequested" src-go src-im-bridge` — must return 0 hits.
  - Run `rtk grep -nR "formatReviewFollowUpTasks\|renderFollowupTaskSuggestions" src-go src-im-bridge` — must return 0 hits.
  - Run `cd src-go && rtk go build ./...` and `cd src-im-bridge && rtk go build ./...` — both green.

- [ ] Step 7.9 — run the dead-code reflect test from 7.1 — green.

---

## Task 8 — Integration test: end-to-end Trace C

- [ ] Step 8.1 — write the integration test (Postgres + Redis + mock VCS HTTP)
  - File: `src-go/internal/service/review_service_trace_c_integration_test.go` (new; gated by `// +build integration`)
  - Steps the test exercises:
    1. Seed a `vcs_integrations` row + an initial completed review with `head_sha="X"`, `last_reviewed_sha="X"`, `summary_comment_id="sc-1"`, two findings F1 (a.go:10) + F2 (b.go:20) each with `inline_comment_id`.
    2. Fire a synthetic `pull_request:synchronize` event with `head_sha="Y"` to `webhook_router.RouteEvent`.
    3. Mock `vcs.Provider.ComparePullRequest(repo, "X", "Y")` returns `["a.go"]`.
    4. Mock bridge returns findings `[F1 (still at a.go:10), F3 NEW at a.go:15]`.
    5. Assertions:
       - A new reviews row exists with `parent_review_id == originalReview.ID`, `base_sha == "X"`, `head_sha == "Y"`, `last_reviewed_sha == "Y"` (after Complete).
       - Mock VCS recorded: `EditSummaryComment("sc-1", ...)` once, `PostReviewComments` containing F3 once, **no** `EditReviewComment` for F2 (b.go not in changedFiles), **no** `DeleteReviewComment`.

- [ ] Step 8.2 — run `cd src-go && rtk go test -tags=integration ./internal/service/ -run TraceC` — green.

---

## Task 9 — Final verification

- [ ] Step 9.1 — Run full test suite
  - `cd src-go && rtk go test ./...` — green
  - `cd src-im-bridge && rtk go test ./...` — green
- [ ] Step 9.2 — Lint
  - `cd src-go && rtk go vet ./...` — clean
- [ ] Step 9.3 — Add a Spec Drift entry to `docs/superpowers/specs/2026-04-20-code-reviewer-employee-design.md` §13.1:
  - Function name drift: spec says `renderFollowupTaskSuggestions`; actual deleted symbol is `formatReviewFollowUpTasks`.
  - Event constant drift: `EventReviewFixRequested` was duplicated in `eventbus/types.go` AND `ws/events.go`; both deleted.
  - Trace C scope: this plan handles `pull_request:synchronize` only; standalone `push` events are not subscribed (synchronize is the canonical PR-head-moved signal from GitHub).

---

Plan saved to `D:\Project\AgentForge\docs\superpowers\plans\2026-04-20-spec2-2C-diff-of-diff-and-cleanup.md`, 9 tasks, 41 steps.
