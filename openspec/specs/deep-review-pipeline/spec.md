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
The system SHALL execute logic, security, performance, and compliance review dimensions through the TypeScript bridge and aggregate them into a single Layer 2 review result.

#### Scenario: Parallel review completes successfully
- **WHEN** the bridge receives a valid deep review execution request
- **THEN** it runs the four configured review dimensions in parallel and returns one aggregated result containing findings, summary, recommendation, and cost metadata

#### Scenario: One review dimension fails
- **WHEN** one review dimension errors or times out while the others complete
- **THEN** the aggregated result marks the dimension failure explicitly and the overall review run is completed or failed according to the configured backend handling rules

#### Scenario: Duplicate findings are consolidated
- **WHEN** multiple review dimensions report materially identical findings for the same code region
- **THEN** the aggregation step emits a deduplicated finding set while preserving the original dimension metadata needed for auditability

### Requirement: Layer 2 review results are persisted and observable
The system SHALL persist each Layer 2 review run with structured findings and make the result available through backend APIs and real-time events.

#### Scenario: Review result is stored
- **WHEN** the backend receives an aggregated Layer 2 result
- **THEN** it stores the review with `layer = 2`, pull request context, risk level, findings, summary, recommendation, and cost information

#### Scenario: Review completion is broadcast
- **WHEN** a Layer 2 review transitions to a terminal state
- **THEN** the backend emits the corresponding review WebSocket event and creates a notification record for the relevant task or member context

#### Scenario: Review status can be queried
- **WHEN** a client requests the status or details of a stored Layer 2 review
- **THEN** the backend returns the persisted review metadata and aggregated findings for that review identifier

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
