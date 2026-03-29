## Context

AgentForge already has the core review-pipeline execution path described across `docs/PRD.md`, `docs/part/REVIEW_PIPELINE_DESIGN.md`, and the archived `close-review-pipeline-loop` work: Layer 1 CI ingestion, Layer 2 deep review triggering, review persistence, `pending_human` routing, and `review.completed` / `review.pending_human` / `review.updated` WebSocket events. The remaining gap is the dashboard surface that operators use to inspect and act on that data.

Today the dashboard exposes two separate review experiences:

- `app/(dashboard)/reviews/page.tsx` renders a backlog table with inline findings expansion.
- `components/review/task-review-section.tsx` renders task-scoped cards plus a separate detail panel.

These paths duplicate badge styles, recommendation labels, findings rendering, and pending-human actions. They also diverge in capability: the task surface supports trigger and human actions, while the backlog page does not expose the same decision flow or shared detail contract. This makes the review pipeline harder to operate and drifts from the documented dashboard/manual trigger story.

## Goals / Non-Goals

**Goals:**

- Define one operator workspace contract for review backlog, detail, manual deep-review trigger, and pending-human decision handling.
- Reuse the same review presentation primitives across `/reviews` and task detail surfaces so status, recommendations, findings, provenance, and decision history stay consistent.
- Ensure task-bound and standalone deep reviews resolve into the same detail model and navigation target.
- Preserve existing backend APIs and event names while making frontend consumption consistent and scalable for future review features.

**Non-Goals:**

- Re-architect Layer 1 / Layer 2 execution, CI workflows, or review persistence.
- Introduce new review decision types beyond approve, request-changes, reject, and false-positive.
- Redesign the overall dashboard information architecture outside review-related surfaces.
- Change plugin execution, review aggregation logic, or IM command parsing beyond what is required to point users to the shared workspace.

## Decisions

### Decision: Introduce shared review workspace primitives instead of expanding page-local JSX

The frontend will define a reusable review workspace component set that can power both the backlog page and task-scoped review entry points. At minimum this workspace will separate:

- backlog/list shell
- summary/status metadata presentation
- detail panel
- decision action area
- manual trigger form

Why this over extending existing pages independently:

- It removes duplicated rendering logic already split between `/reviews` and `components/review/*`.
- It keeps future review surfaces, such as project-level backlog or IM-linked detail pages, on the same DTO and interaction model.
- It makes i18n and badge semantics consistent instead of page-specific.

Alternative considered: keep `/reviews` as a table-only page and only extract small helper functions for colors/labels. Rejected because it would still leave two incompatible interaction models and would not satisfy the documented operator workflow.

### Decision: Use one detail view contract for task-bound and standalone reviews

The shared detail surface will treat `taskId` as optional and render both review types through the same data contract. Task-bound reviews can still show task context, but detached reviews must not fork to a different component tree.

Why this over separate detached-review components:

- The store already normalizes detached reviews by permitting empty task IDs.
- The docs describe standalone deep review as a first-class review path, not a different product area.
- Reuse reduces the chance that execution metadata, decision history, or findings actions drift between review types.

Alternative considered: keep detached reviews only in `/reviews` and task-bound reviews only inside task detail. Rejected because human operators need a single navigation and action model.

### Decision: Keep WebSocket event semantics unchanged and standardize consumer behavior

The change will not add new review event types. Instead, the shared workspace contract will require backlog and task views to consume `ReviewDTO` updates through the existing store and event pipeline so both contexts react identically to `review.completed`, `review.pending_human`, and `review.updated`.

Why this over new UI-specific events:

- Existing event coverage already matches the state transition specs.
- New event types would add avoidable backend scope for a frontend consistency gap.
- A shared consumer path is enough to close the operator experience gap.

Alternative considered: add page-specific refresh events or polling hooks. Rejected because it would duplicate state synchronization paths and regress the real-time contract.

### Decision: Make manual deep-review trigger a workspace-level action

Manual trigger UX will be defined once and embedded where needed, rather than allowing `/reviews` and task surfaces to invent different request shapes or input constraints. The trigger flow must support task-bound requests and standalone PR URL requests using the same validation and submission pattern.

Why this over keeping manual trigger task-only:

- The docs explicitly position manual deep review as a dashboard-triggerable operator flow.
- Standalone deep review already exists as a backend capability; the missing piece is the operator surface.

## Risks / Trade-offs

- [Risk] Shared components could flatten context-specific affordances and make task pages feel less focused. → Mitigation: keep shell composition flexible so task pages can embed the shared detail/list primitives without copying their internals.
- [Risk] Converging backlog and task flows may expose edge cases for detached reviews with no task metadata. → Mitigation: define optional task context explicitly in the workspace spec and test detached review rendering paths.
- [Risk] `/reviews` currently uses a table while task surfaces use cards; forcing one layout everywhere may hurt readability. → Mitigation: reuse view-model and detail/action primitives while allowing list shells to differ where layout needs diverge.
- [Risk] Existing tests cover task review components more than the backlog page. → Mitigation: require task, backlog, and store/event tests to be updated together as part of implementation.

## Migration Plan

1. Extract and align shared review presentation/action primitives under `components/review/`.
2. Refit `app/(dashboard)/reviews/page.tsx` to consume the shared workspace contract.
3. Refit task detail review sections to use the same primitives without changing backend API contracts.
4. Update translations and route linking so standalone deep reviews open the shared detail surface.
5. Verify store and WebSocket tests still prove that one incoming review event updates both backlog and task contexts.

Rollback is straightforward because this change is frontend- and spec-focused: revert the shared workspace composition and restore page-local rendering while leaving backend review APIs untouched.

## Open Questions

- Whether `/reviews` should retain a table shell or switch entirely to card/list layout can remain an implementation decision as long as the shared detail and action contract is preserved.
- If operators need project-level filters or saved views in the review backlog, those should be proposed separately once the shared workspace baseline exists.
