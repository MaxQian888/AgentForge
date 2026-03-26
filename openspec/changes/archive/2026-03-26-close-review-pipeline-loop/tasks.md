## 1. Backend: Project Review Policy Persistence

- [x] 1.1 Add `ReviewPolicy` struct to `GovernanceSettings` in `src-go/internal/model/project.go` with fields `RequiredLayers []string`, `RequireManualApproval bool`, `MinRiskLevelForBlock string`
- [x] 1.2 Update project repository merge/persist logic to include `reviewPolicy` in the JSONB settings document without dropping existing governance fields
- [x] 1.3 Update `project_handler.go` UpdateProjectSettings to deserialize and persist `reviewPolicy` from the request body
- [x] 1.4 Update project settings GET response to always return a `reviewPolicy` block with safe defaults when none is saved
- [x] 1.5 Add unit tests for the settings merge logic verifying `reviewPolicy` is preserved when other sections are updated independently

## 2. Backend: Review State Machine Refactor

- [x] 2.1 Add `ReviewDecision` struct to `src-go/internal/model/review.go` with fields: `Actor string`, `Action string`, `Comment string`, `Timestamp time.Time`
- [x] 2.2 Extend `ExecutionMetadata` to include `Decisions []ReviewDecision`
- [x] 2.3 Implement `ApproveReview(ctx, reviewID, actor, comment string) error` in `review_service.go` — validates state is `pending_human`, appends decision, updates status/recommendation, does NOT overwrite findings/summary/costUSD
- [x] 2.4 Implement `RequestChangesReview(ctx, reviewID, actor, comment string) error` in `review_service.go` — same pattern as Approve with `request_changes` action
- [x] 2.5 Implement `MarkFalsePositive(ctx, reviewID, actor string, findingIDs []string, reason string) error` in `review_service.go` — appends decision, sets `dismissed: true` on referenced findings
- [x] 2.6 Update `review_handler.go` Approve endpoint to call `ApproveReview` instead of re-invoking `Complete`
- [x] 2.7 Update `review_handler.go` RequestChanges endpoint to call `RequestChangesReview` instead of re-invoking `Complete`
- [x] 2.8 Update `review_handler.go` Reject endpoint to use the new transition pattern
- [x] 2.9 Add unit tests for each new transition method verifying evidence fields are unchanged post-transition

## 3. Backend: pending_human Routing in ReviewService.Complete

- [x] 3.1 Load project settings inside `ReviewService.Complete` after persisting the bridge result
- [x] 3.2 Evaluate `policy.RequireManualApproval` and `policy.MinRiskLevelForBlock` against the result's max finding severity
- [x] 3.3 Route to `RequestHumanApproval` (emit `review.pending_human` event) when either condition is met; otherwise proceed to auto-resolve
- [x] 3.4 Wire `RequestHumanApproval` to emit the `review.pending_human` WebSocket event with review ID, project, and PR reference
- [x] 3.5 Add integration tests for the two pending_human routing scenarios and the auto-resolve pass-through

## 4. Backend: Standalone Deep Review (PR URL without taskId)

- [x] 4.1 Update `ReviewService.CreateReview` to accept `taskID == ""` when `prUrl` is present — create a detached review record with nil task reference
- [x] 4.2 Skip task-state follow-up steps in `ReviewService.Complete` when review has no `taskID`
- [x] 4.3 Update `review_handler.go` create endpoint to pass through requests missing `taskId` without returning `ErrReviewTaskNotFound`
- [x] 4.4 Add unit test: standalone review creation with PR URL only returns review ID and initiates bridge run

## 5. CI: Layer 1 Workflow Structured JSON and ci-result Ingest

- [x] 5.1 Rewrite `.github/workflows/agent-review.yml` analysis step to produce structured JSON: `{"needs_deep_review": bool, "reason": string, "confidence": string, "pr_url": string}`
- [x] 5.2 Add a workflow step that POSTs the JSON to `POST /api/v1/reviews/ci-result` using a `AGENTFORGE_CI_TOKEN` repository secret
- [x] 5.3 Verify `review-layer2.yml` listener event is compatible with the updated Layer 1 output (no hardcoded escalation bypass)
- [x] 5.4 Document the required `AGENTFORGE_CI_TOKEN` secret in the project README or workflow file comment

## 6. Frontend: Type Alignment and Store Actions

- [x] 6.1 Extend `ReviewRecord` TypeScript interface in `lib/stores/review-store.ts` to include `executionMetadata?: ExecutionMetadata` with sub-types for `triggerEvent`, `changedFiles`, `decisions`, and per-plugin results
- [x] 6.2 Add `requestChanges(reviewId: string, comment: string)` action to the review store calling `POST /api/v1/reviews/:id/request-changes`
- [x] 6.3 Add `markFalsePositive(reviewId: string, findingIds: string[], reason: string)` action to the review store calling `POST /api/v1/reviews/:id/false-positive`
- [x] 6.4 Update existing `approve` and `reject` store actions to call their correct endpoints (no change to backend contract, just verify alignment)

## 7. Frontend: pending_human Action Surface

- [x] 7.1 Update `review-list.tsx` to show Approve and Request Changes buttons when `review.status === "pending_human"` (not only when `completed`)
- [x] 7.2 Update `review-detail-panel.tsx` to show the human transition action section for `pending_human` reviews
- [x] 7.3 Add `pending_human` to the status filter options in `app/(dashboard)/reviews/page.tsx`
- [x] 7.4 Verify the existing `pending_human` badge color in the backlog page is still correct after filter addition

## 8. Frontend: ExecutionMetadata and Plugin Provenance Display

- [x] 8.1 Add an "Execution Details" collapsible section to `review-detail-panel.tsx` that renders `executionMetadata` (trigger event, changed file count, per-plugin result table) when present
- [x] 8.2 Add a "Source" column to `review-findings-table.tsx` that displays the plugin or dimension name from `finding.sources[0]` (or a badge for multi-source)
- [x] 8.3 Add a "Decisions" section to `review-detail-panel.tsx` that renders the `executionMetadata.decisions` audit trail (actor, action, comment, timestamp) when decisions exist

## 9. Frontend: WebSocket Review Event Handlers

- [x] 9.1 Register a `review.completed` handler in `ws-store.ts` that calls `reviewStore.updateReview` with the event payload
- [x] 9.2 Register a `review.pending_human` handler in `ws-store.ts` that calls `reviewStore.updateReview` and optionally triggers a toast notification
- [x] 9.3 Register a `review.updated` handler in `ws-store.ts` that calls `reviewStore.updateReview`
- [x] 9.4 Verify that review backlog and task review section re-render when store is updated via WebSocket events

## 10. IM Bridge: Extended /review Commands and Action Buttons

- [x] 10.1 Add `deep <pr-url>` subcommand to `src-im-bridge/cmd/bridge/review.go` — call standalone deep review creation API and reply with status card
- [x] 10.2 Add `approve <review-id>` subcommand calling `ApproveReview` API and replying with confirmation card
- [x] 10.3 Add `request-changes <review-id> [comment]` subcommand calling `RequestChangesReview` API and replying with confirmation card
- [x] 10.4 Update the review result card builder to include "Approve" and "Request Changes" action buttons when review status is `pending_human`
- [x] 10.5 Update the review result card builder to omit action buttons when review is in a terminal state
- [x] 10.6 Wire action button callbacks through the existing IM action execution infrastructure (`im_action_execution.go`)
- [x] 10.7 Add unit tests for each new subcommand and card button wiring
