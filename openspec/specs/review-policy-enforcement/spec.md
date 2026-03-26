# review-policy-enforcement Specification

## Purpose
TBD - created by archiving change close-review-pipeline-loop. Update Purpose after archive.
## Requirements
### Requirement: Project review policy is persisted in backend settings
The system SHALL persist `reviewPolicy` as a structured sub-document within `ProjectSettings.GovernanceSettings`. The policy MUST include at minimum: `requiredLayers` (list of required review layer identifiers), `requireManualApproval` (boolean), and `minRiskLevelForBlock` (severity threshold string). A project with no saved policy MUST return safe default values (empty requiredLayers, requireManualApproval false, minRiskLevelForBlock empty) that preserve existing auto-resolve behavior.

#### Scenario: Operator saves review policy fields
- **WHEN** an authenticated operator submits a project settings update that includes a `reviewPolicy` block with valid fields
- **THEN** the backend persists those values within the project's governance settings document
- **THEN** a subsequent project settings read returns the same `reviewPolicy` values without dropping or defaulting them

#### Scenario: Project with no saved policy returns safe defaults
- **WHEN** an authenticated caller reads settings for a project that has never had a `reviewPolicy` saved
- **THEN** the backend returns a `reviewPolicy` block with `requireManualApproval: false`, empty `requiredLayers`, and no `minRiskLevelForBlock` threshold
- **THEN** the auto-resolve behavior of `ReviewService.Complete` is unaffected for that project

#### Scenario: Invalid review policy is rejected on save
- **WHEN** an operator submits a project settings update with a `reviewPolicy` block containing an unrecognized `minRiskLevelForBlock` value or contradictory field combination
- **THEN** the backend rejects the update with a field-level validation error
- **THEN** the previously persisted project settings remain unchanged

### Requirement: Review policy is evaluated during review completion to determine routing
The system SHALL evaluate the project's persisted `reviewPolicy` at the point when `ReviewService.Complete` is called with an automated bridge result. If `requireManualApproval` is true OR if the result's maximum finding severity meets or exceeds `minRiskLevelForBlock`, the review SHALL transition to `pending_human` instead of auto-resolving. Only when neither condition is satisfied SHALL the review resolve to a terminal state automatically.

#### Scenario: Policy requires manual approval — review enters pending_human
- **WHEN** `ReviewService.Complete` is called for a project whose `reviewPolicy.requireManualApproval` is true
- **THEN** the review transitions to `pending_human` instead of a completed terminal state
- **THEN** a `review.pending_human` event is emitted so that frontend and IM surfaces can expose the pending approval action

#### Scenario: Finding severity meets block threshold — review enters pending_human
- **WHEN** `ReviewService.Complete` is called and the aggregated result contains at least one finding whose severity meets or exceeds the project's `minRiskLevelForBlock` value
- **THEN** the review transitions to `pending_human` regardless of the automated recommendation
- **THEN** a `review.pending_human` event is emitted

#### Scenario: No policy conditions met — review resolves automatically
- **WHEN** `ReviewService.Complete` is called, `requireManualApproval` is false, and no finding meets the `minRiskLevelForBlock` threshold
- **THEN** the review transitions to its terminal state based on the automated recommendation (approved or changes-requested)
- **THEN** a `review.completed` event is emitted with the final state

