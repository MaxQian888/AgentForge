## Why

The review system has a functioning middle layer (Layer 2 bridge, ReviewPlugin aggregation, persistence) but both ends of the pipeline are broken: project review policy is never persisted or enforced, human approval flows overwrite evidence instead of transitioning state, and the Web/IM surfaces expose only a fraction of the available backend capabilities. The result is a system that appears complete in demos but cannot actually close a review loop in production.

## What Changes

- **Backend**: Extend `ProjectSettings` to persist `reviewPolicy` (requiredLayers, requireManualApproval, minRiskLevelForBlock); wire policy into `ReviewService.Complete` so it routes to `pending_human` when conditions are met instead of always resolving to done
- **Backend**: Replace the Approve/Reject/RequestChanges pattern of re-calling `Complete` with dedicated state-transition methods that append a decision record without overwriting findings, executionMetadata, summary, or costUSD
- **Backend**: Allow `CreateReview` to accept a standalone `prUrl` without a pre-existing `taskId`, creating a detached review record and initiating deep review independently
- **CI / GitHub Actions**: Wire the existing `/reviews/ci-result` ingest API into the Layer 1 workflow; fix `agent-review.yml` to emit structured JSON and call the backend endpoint instead of hardcoding `needs_deep_review: true`
- **Frontend**: Sync `ReviewRecord` TS type with Go DTO (add `executionMetadata`); add `requestChanges` and `markFalsePositive` actions to the review store
- **Frontend**: Surface `pending_human` as an actionable state — show Approve/Request Changes buttons when `status === "pending_human"`, not only when `status === "completed"`
- **Frontend**: Add plugin provenance / sources column to findings table; show `executionMetadata` (trigger, changed files, per-plugin results) in detail panel
- **Frontend**: Register `review.*` WebSocket event handlers in `ws-store` to keep review backlog and task review section live
- **IM**: Extend `/review` command to support `deep <pr-url>`, `approve <id>`, and `request-changes <id> <comment>`; wire IM action buttons to the existing action execution infrastructure

## Capabilities

### New Capabilities

- `review-policy-enforcement`: Project-level review policy persisted in backend and enforced during `ReviewService.Complete` — determines whether a completed automated review auto-resolves or enters `pending_human`
- `review-state-transitions`: Dedicated Approve / RequestChanges / FalsePositive transition operations that append a decision record without mutating the original review evidence
- `review-standalone-deep`: Ability to trigger a deep review from a bare PR URL with no pre-existing task binding

### Modified Capabilities

- `deep-review-pipeline`: Layer 1 workflow gains a real structured JSON output and calls the `/reviews/ci-result` ingest API; the `pending_human` routing path is wired into the `Complete` flow
- `review-plugin-support`: ExecutionMetadata (plugin provenance, per-plugin results, changedFiles) surfaced in the Web UI; findings table gains a sources/provenance column
- `project-settings-control-plane`: ProjectSettings schema extended with `reviewPolicy` block; handler and repository merge/persist logic updated
- `im-bridge-control-plane`: `/review` IM command extended with `deep`, `approve`, `request-changes` subcommands and action buttons

## Impact

**Go backend**: `internal/model/project.go`, `internal/service/review_service.go`, `internal/handler/review_handler.go`, `internal/handler/project_handler.go`, `internal/repository/` (project settings persist path)

**TypeScript bridge**: `src-im-bridge/cmd/bridge/review.go`

**Frontend**: `lib/stores/review-store.ts`, `lib/stores/ws-store.ts`, `components/reviews/review-list.tsx`, `components/reviews/review-detail-panel.tsx`, `components/reviews/review-findings-table.tsx`, `app/(dashboard)/reviews/page.tsx`

**CI**: `.github/workflows/agent-review.yml`, `.github/workflows/review-layer2.yml`

**No new external dependencies required.**
