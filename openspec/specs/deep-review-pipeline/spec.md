# deep-review-pipeline Specification

## Purpose
Define the baseline contract for running Layer 2 deep review through the AgentForge Go backend, the TypeScript bridge, and GitHub-triggered escalation so that escalated pull requests can be reviewed in parallel dimensions, persisted as one aggregated review result, and surfaced through task state, notifications, and real-time events.
## Requirements
### Requirement: Layer 2 deep review can be triggered from escalation sources
The system SHALL support creating a Layer 2 deep review when an agent-authored pull request, a Layer 1 escalation result, or an authenticated manual request indicates that deeper review is required.

#### Scenario: Agent-authored pull request escalates automatically
- **WHEN** a pull request created from an `agent/` branch enters the review workflow
- **THEN** the system creates a Layer 2 review trigger with the pull request identifier and review dimensions needed for deep review

#### Scenario: Layer 1 escalation requests deep review
- **WHEN** Layer 1 produces escalation metadata indicating `needs_deep_review`
- **THEN** the system triggers a Layer 2 review for the same pull request without requiring a second manual action

#### Scenario: Manual deep review request is accepted
- **WHEN** an authenticated caller submits a manual deep review request with a valid pull request reference
- **THEN** the system creates a pending Layer 2 review run and returns a review identifier that can be tracked

### Requirement: Layer 2 deep review runs parallel review dimensions through the bridge
The system SHALL execute Layer 2 deep review through the TypeScript bridge using one execution plan that includes the built-in review dimensions and any enabled `ReviewPlugin` instances that match the current review trigger. The aggregated result MUST preserve per-dimension or per-plugin provenance while still returning one unified review outcome.

#### Scenario: Built-in and custom review plugins complete successfully
- **WHEN** the bridge receives a valid deep-review execution request and one or more enabled review plugins match that request
- **THEN** it runs the built-in review dimensions together with the matching review plugins and returns one aggregated result containing findings, summary, recommendation, and provenance metadata

#### Scenario: One review plugin fails while others complete
- **WHEN** one enabled review plugin errors or times out while other built-in or custom review dimensions complete
- **THEN** the aggregated result marks that plugin failure explicitly and preserves the successful findings from the remaining dimensions

#### Scenario: Duplicate findings are consolidated across built-in and plugin sources
- **WHEN** multiple built-in dimensions or review plugins report materially identical findings for the same code region
- **THEN** the aggregation step emits a deduplicated finding set while preserving the original plugin or dimension metadata needed for auditability

### Requirement: Layer 2 review results are persisted and observable
The system SHALL persist each Layer 2 review run with structured findings and make the result available through backend APIs and real-time events. Persisted findings and execution metadata MUST preserve which built-in dimension or `ReviewPlugin` produced each contribution.

#### Scenario: Review result stores plugin provenance
- **WHEN** the backend receives an aggregated Layer 2 result that includes built-in and custom review plugin contributions
- **THEN** it stores the review with structured findings, plugin or dimension provenance, summary, recommendation, and cost information

#### Scenario: Review completion is broadcast with plugin-aware metadata
- **WHEN** a Layer 2 review transitions to a terminal state
- **THEN** the backend emits the corresponding review event and makes the persisted plugin-aware review metadata queryable through review APIs

### Requirement: Layer 2 recommendations drive follow-up workflow state
The system SHALL translate Layer 2 recommendations together with project-scoped review policy into consistent task, review, and approval state updates so downstream users and agents can act on the result. Project settings that require manual approval or equivalent escalation MUST be honored before a review can fully complete, even when the deep-review recommendation is `approve`. When a project policy condition is met, the system SHALL transition the review to `pending_human` rather than auto-resolving, and SHALL emit a `review.pending_human` event with enough context for human reviewers to take action.

#### Scenario: Approve recommendation completes automatically when no manual gate applies
- **WHEN** a Layer 2 review recommendation is `approve` and the associated project's review policy does not require manual approval for that pull request
- **THEN** the review is marked completed and the associated task or review workflow state reflects that deep review passed

#### Scenario: Approve recommendation enters manual approval when project policy requires it
- **WHEN** a Layer 2 review recommendation is `approve` and the associated project's review policy requires manual approval for that pull request
- **THEN** the system records that deep review passed but moves the review or task workflow into a `pending_human` state instead of auto-completing it
- **THEN** a `review.pending_human` event is emitted with the review identifier, project, and pull request reference
- **THEN** notifications and APIs expose that manual approval is still required before final approval can be granted

#### Scenario: Finding severity meets project block threshold — review enters pending_human
- **WHEN** a Layer 2 review result contains findings whose maximum severity meets or exceeds the project's `minRiskLevelForBlock` threshold
- **THEN** the system transitions the review to `pending_human` regardless of the automated recommendation
- **THEN** a `review.pending_human` event is emitted

#### Scenario: Request-changes recommendation records actionable feedback
- **WHEN** a Layer 2 review recommendation is `request_changes`
- **THEN** the system persists the actionable findings summary and updates the associated task or review workflow state to reflect requested changes

#### Scenario: Reject recommendation marks the review as failed
- **WHEN** a Layer 2 review recommendation is `reject`
- **THEN** the system marks the Layer 2 review outcome as failed or rejected and exposes that status through review APIs and notifications

### Requirement: Layer 1 CI workflow emits structured JSON and ingests results into the backend
The system SHALL provide a Layer 1 GitHub Actions workflow that evaluates each non-Draft pull request, produces a structured JSON decision object containing `needs_deep_review` (boolean), `reason` (string), and `confidence` (string), and POSTs that result to the backend `/reviews/ci-result` endpoint using a scoped CI service token. The workflow MUST NOT hard-code `needs_deep_review: true` for all pull requests.

#### Scenario: Layer 1 workflow evaluates a pull request and calls ci-result endpoint
- **WHEN** a non-Draft pull request is opened or synchronized
- **THEN** the Layer 1 workflow runs an analysis step (diff size, label presence, file scope)
- **THEN** it posts a structured JSON result to `POST /api/v1/reviews/ci-result` with `needs_deep_review`, `reason`, confidence, and the pull request reference
- **THEN** the backend creates or updates the Layer 1 ingest record for that PR

#### Scenario: Layer 1 decision with needs_deep_review true triggers Layer 2
- **WHEN** the backend receives a Layer 1 ingest result with `needs_deep_review: true`
- **THEN** it triggers a Layer 2 deep review for the referenced pull request following the same path as a manual deep review request

#### Scenario: Layer 1 decision with needs_deep_review false is recorded but does not trigger Layer 2
- **WHEN** the backend receives a Layer 1 ingest result with `needs_deep_review: false`
- **THEN** it records the result without initiating a Layer 2 deep review run

