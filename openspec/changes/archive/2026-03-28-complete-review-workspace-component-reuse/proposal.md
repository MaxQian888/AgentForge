## Why

The review pipeline's backend, WebSocket events, and CI triggers now cover Layer 1 ingest, Layer 2 deep review, and human follow-up states, but the operator-facing dashboard still exposes that workflow through fragmented UI surfaces. `/reviews` and the task-level review section duplicate status rendering, findings presentation, and human decision entry, so the current experience does not fully match the documented review backlog and manual deep-review flow in `docs/PRD.md` and `docs/part/REVIEW_PIPELINE_DESIGN.md`.

This change is needed now because the remaining gap is no longer infrastructure but operator usability and consistency. Without a shared review workspace contract, the documented review pipeline remains only partially usable in the dashboard and future review features will keep re-implementing the same view logic.

## What Changes

- Add a dedicated review operator workspace capability for the dashboard that unifies backlog, detail, manual deep-review trigger, and pending-human decision flows.
- Define shared review presentation and action patterns so `/reviews` and task-embedded review surfaces reuse the same summary, findings, provenance, and decision components instead of maintaining separate render paths.
- Require standalone deep reviews and task-bound reviews to resolve into the same detail surface, including execution metadata, review decisions, and actionable next steps.
- Align dashboard behavior with documented review pipeline flows by making backlog filtering, manual trigger entry, and review decision actions first-class operator workflows.

## Capabilities

### New Capabilities
- `review-operator-workspace`: Dashboard review backlog, detail, manual deep-review trigger, and task-level review entry points operate through one shared workspace contract and reusable UI building blocks.

### Modified Capabilities
- `review-standalone-deep`: Standalone deep reviews must surface in the shared review workspace with the same detail view and status tracking semantics as task-bound reviews.
- `review-state-transitions`: Human review transitions must be exposed through shared operator actions so pending-human reviews can be resolved consistently from backlog and task contexts.

## Impact

- Affected frontend areas: `app/(dashboard)/reviews/page.tsx`, `components/review/*`, review-related translations, and any task detail surface embedding review UI.
- Affected client state: `lib/stores/review-store.ts` and WebSocket-driven review updates consumed by the dashboard.
- Affected existing specs: `review-standalone-deep`, `review-state-transitions`.
- Documentation alignment target: `docs/PRD.md` and `docs/part/REVIEW_PIPELINE_DESIGN.md`.
