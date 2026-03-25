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
The system SHALL translate Layer 2 recommendations into consistent task and review state updates so downstream users and agents can act on the result.

#### Scenario: Approve recommendation completes the review
- **WHEN** a Layer 2 review recommendation is `approve`
- **THEN** the review is marked completed and the associated task or review workflow state reflects that deep review passed

#### Scenario: Request-changes recommendation records actionable feedback
- **WHEN** a Layer 2 review recommendation is `request_changes`
- **THEN** the system persists the actionable findings summary and updates the associated task or review workflow state to reflect requested changes

#### Scenario: Reject recommendation marks the review as failed
- **WHEN** a Layer 2 review recommendation is `reject`
- **THEN** the system marks the Layer 2 review outcome as failed or rejected and exposes that status through review APIs and notifications

