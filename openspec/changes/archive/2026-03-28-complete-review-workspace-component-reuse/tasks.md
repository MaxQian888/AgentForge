## 1. Shared Review Workspace Primitives

- [x] 1.1 Extract shared review presentation helpers and reusable UI primitives for status, recommendation, summary metadata, findings, and decision history under `components/review/`.
- [x] 1.2 Define a reusable review detail/action composition that supports task-bound and standalone reviews through the same `ReviewDTO` contract.
- [x] 1.3 Define one reusable manual deep-review trigger flow that can submit task-bound and standalone requests with consistent validation and error handling.

## 2. Dashboard And Task Surface Integration

- [x] 2.1 Refactor `app/(dashboard)/reviews/page.tsx` to use the shared review workspace primitives instead of page-local status/findings rendering.
- [x] 2.2 Refactor task-level review entry points to reuse the same detail/action/trigger primitives while preserving task-context affordances.
- [x] 2.3 Ensure standalone deep reviews and task-bound reviews both navigate into the shared detail surface and react consistently to `review.completed`, `review.pending_human`, and `review.updated`.

## 3. Verification And Content Alignment

- [x] 3.1 Update review-related translations and UI copy so backlog, detail, and task surfaces use the same documented terminology.
- [x] 3.2 Add or update component/store tests covering shared detail rendering, pending-human actions, standalone review handling, and WebSocket-driven state updates across backlog and task contexts.
- [x] 3.3 Run the relevant frontend quality gates for the touched review workspace surface and confirm the implementation matches the documented dashboard review workflow.
