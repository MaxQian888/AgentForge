# project-settings-control-plane Specification

## Purpose
Define the project-scoped settings control plane contract so operators can manage identity, repository metadata, coding-agent defaults, budget and alert governance, and review policy as one structured settings document with truthful diagnostics and legacy-safe defaults.
## Requirements
### Requirement: Project settings workspace renders one complete operator control plane
The system SHALL present one project-scoped settings workspace that covers general project identity, repository metadata, coding-agent defaults, budget and alert governance, review policy, and operator-facing summary or diagnostics. The workspace MUST load from the project settings API response, MUST keep a reversible client-side draft that is distinct from the last persisted settings snapshot, and MUST expose explicit save and discard/reset affordances whenever the draft diverges from persisted values.

#### Scenario: Operator opens settings for a configured project
- **WHEN** an authenticated operator opens the settings page for a project with saved governance settings
- **THEN** the UI shows labeled sections for general, repository, coding-agent, budget and alerts, review policy, and operator summary
- **THEN** the rendered values come from the current project settings response instead of page-local constants

#### Scenario: Legacy project only has coding-agent settings
- **WHEN** an authenticated operator opens settings for a project whose persisted settings only contain legacy coding-agent fields
- **THEN** the backend returns defaulted governance sections together with that project's existing coding-agent settings
- **THEN** the UI renders editable fallback values instead of leaving governance sections blank or unavailable

#### Scenario: Operator edits settings before saving
- **WHEN** an authenticated operator changes one or more editable settings fields without submitting
- **THEN** the workspace marks the settings draft as having unsaved changes
- **THEN** the workspace exposes save and discard or reset actions without mutating the last persisted snapshot

#### Scenario: Operator discards unsaved changes
- **WHEN** an authenticated operator chooses discard or reset after editing the settings draft
- **THEN** all editable settings fields revert to the last persisted snapshot for the selected project
- **THEN** the unsaved-changes indicator clears

### Requirement: Project settings save as one structured document
The system SHALL persist project governance settings as one structured settings document returned through the existing project read and update surfaces. A valid save MUST preserve unchanged sections, while invalid threshold or policy combinations MUST be rejected with actionable validation feedback. The persisted document SHALL include `reviewPolicy` as a first-class sub-document within the governance settings, and the merge logic MUST NOT drop `reviewPolicy` fields when other sections are updated independently. The frontend workspace MUST keep invalid or failed submissions in draft form and MUST surface actionable form-level or field-level feedback without falsely reporting success.

#### Scenario: Operator saves multiple settings sections together
- **WHEN** an authenticated operator submits one valid project settings update that changes repository fields, coding-agent defaults, budget thresholds, and review policy together
- **THEN** the backend persists those changes as one project settings document for that project
- **THEN** a subsequent project read returns the same resolved values without dropping unchanged sections

#### Scenario: Operator updates only review policy — other sections are unchanged
- **WHEN** an authenticated operator submits a project settings update containing only `reviewPolicy` fields
- **THEN** the backend merges the new policy into the existing settings document without modifying budget, repository, or coding-agent sections
- **THEN** a subsequent project read returns the updated `reviewPolicy` alongside the unchanged sections

#### Scenario: Invalid governance thresholds are rejected
- **WHEN** an operator submits a project settings update whose threshold or policy values are internally inconsistent or outside supported ranges
- **THEN** the backend rejects the update with field-level validation errors
- **THEN** the previously persisted project settings remain unchanged

#### Scenario: Save fails validation in the frontend workspace
- **WHEN** an operator submits a settings draft that violates supported input rules or receives a validation failure from the server
- **THEN** the workspace keeps the operator's draft values intact instead of resetting to persisted values
- **THEN** the workspace shows actionable validation feedback near the relevant fields or in form-level feedback

### Requirement: Project settings expose truthful governance diagnostics and summaries
The system SHALL expose a project-scoped settings summary or diagnostics surface that reflects the active runtime readiness, budget governance posture, review routing posture, and any fallback or blocking conditions relevant to operators. The summary MUST reflect the currently visible settings state, including draft changes that have not yet been saved, and MUST distinguish between persisted values, defaulted fallback values, and blocked or invalid selections when those states differ.

#### Scenario: Selected coding-agent runtime is currently unavailable
- **WHEN** the project's selected coding-agent runtime has blocking diagnostics in the runtime catalog
- **THEN** the settings summary identifies that runtime as unavailable and surfaces the blocking reason
- **THEN** the operator can still inspect the persisted selection before choosing a new one

#### Scenario: Project policy requires manual approval
- **WHEN** the project's review policy is configured to require manual approval or equivalent escalation before final approval
- **THEN** the settings summary explicitly states that project reviews will pause for manual approval after deep review
- **THEN** the saved settings response preserves that review posture for later operator inspection

#### Scenario: Legacy fallback values are visible to the operator
- **WHEN** the settings page renders governance values that were defaulted for a legacy project with incomplete persisted settings
- **THEN** the diagnostics or summary surface identifies those values as defaulted or fallback-backed instead of implying they were explicitly saved
- **THEN** the operator can review and save the resolved values intentionally

### Requirement: Project settings API returns reviewPolicy as a structured response field
The system SHALL include `reviewPolicy` in the project settings GET response as a typed sub-document with `requiredLayers`, `requireManualApproval`, and `minRiskLevelForBlock`. When no policy has been saved, the response SHALL return default values (empty requiredLayers, requireManualApproval false, no minRiskLevelForBlock) rather than omitting the field.

#### Scenario: GET project settings returns reviewPolicy field
- **WHEN** an authenticated caller reads project settings for any project
- **THEN** the response includes a `reviewPolicy` object with at minimum `requireManualApproval` (boolean) and `requiredLayers` (array)
- **THEN** projects with no saved policy return the safe default values rather than null or absent fields

### Requirement: Settings page includes a non-project-scoped Appearance section
The system SHALL render an Appearance card at the top of the settings page that is always visible regardless of whether a project is selected. The Appearance section SHALL contain a theme mode selector and a language selector, and SHALL NOT require a project context to function.

#### Scenario: User opens settings with no project selected
- **WHEN** a user navigates to the settings page without a project selected
- **THEN** the Appearance section is visible and fully functional
- **THEN** the project-specific settings sections are not rendered or show the no-project placeholder

#### Scenario: User opens settings with a project selected
- **WHEN** a user navigates to the settings page with a project selected
- **THEN** the Appearance section appears above all project-scoped setting cards
- **THEN** all existing project-scoped cards (General, Repository, Coding Agent, Budget, Review Policy, Webhook, Diagnostics, Custom Fields, Forms, Automations) remain present and unmodified

