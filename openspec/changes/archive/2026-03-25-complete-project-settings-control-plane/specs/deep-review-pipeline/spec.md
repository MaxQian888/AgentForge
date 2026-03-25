## MODIFIED Requirements

### Requirement: Layer 2 recommendations drive follow-up workflow state
The system SHALL translate Layer 2 recommendations together with project-scoped review policy into consistent task, review, and approval state updates so downstream users and agents can act on the result. Project settings that require manual approval or equivalent escalation MUST be honored before a review can fully complete, even when the deep-review recommendation is `approve`.

#### Scenario: Approve recommendation completes automatically when no manual gate applies
- **WHEN** a Layer 2 review recommendation is `approve` and the associated project's review policy does not require manual approval for that pull request
- **THEN** the review is marked completed and the associated task or review workflow state reflects that deep review passed

#### Scenario: Approve recommendation enters manual approval when project policy requires it
- **WHEN** a Layer 2 review recommendation is `approve` and the associated project's review policy requires manual approval for that pull request
- **THEN** the system records that deep review passed but moves the review or task workflow into a pending manual approval state instead of auto-completing it
- **THEN** notifications and APIs expose that manual approval is still required before final approval can be granted

#### Scenario: Request-changes recommendation records actionable feedback
- **WHEN** a Layer 2 review recommendation is `request_changes`
- **THEN** the system persists the actionable findings summary and updates the associated task or review workflow state to reflect requested changes

#### Scenario: Reject recommendation marks the review as failed
- **WHEN** a Layer 2 review recommendation is `reject`
- **THEN** the system marks the Layer 2 review outcome as failed or rejected and exposes that status through review APIs and notifications
