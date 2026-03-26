## MODIFIED Requirements

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

## ADDED Requirements

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
