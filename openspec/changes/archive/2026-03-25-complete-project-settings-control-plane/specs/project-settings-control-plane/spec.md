## ADDED Requirements

### Requirement: Project settings workspace renders one complete operator control plane
The system SHALL present one project-scoped settings workspace that covers general project identity, repository metadata, coding-agent defaults, budget and alert governance, review policy, and operator-facing summary or diagnostics. The workspace MUST load from the project settings API response and MUST NOT rely on hard-coded local defaults beyond explicit fallback values returned by the backend.

#### Scenario: Operator opens settings for a configured project
- **WHEN** an authenticated operator opens the settings page for a project with saved governance settings
- **THEN** the UI shows labeled sections for general, repository, coding-agent, budget and alerts, review policy, and operator summary
- **THEN** the rendered values come from the current project settings response instead of page-local constants

#### Scenario: Legacy project only has coding-agent settings
- **WHEN** an authenticated operator opens settings for a project whose persisted settings only contain legacy coding-agent fields
- **THEN** the backend returns defaulted governance sections together with that project's existing coding-agent settings
- **THEN** the UI renders editable fallback values instead of leaving governance sections blank or unavailable

### Requirement: Project settings save as one structured document
The system SHALL persist project governance settings as one structured settings document returned through the existing project read and update surfaces. A valid save MUST preserve unchanged sections, while invalid threshold or policy combinations MUST be rejected with actionable validation feedback.

#### Scenario: Operator saves multiple settings sections together
- **WHEN** an authenticated operator submits one valid project settings update that changes repository fields, coding-agent defaults, budget thresholds, and review policy together
- **THEN** the backend persists those changes as one project settings document for that project
- **THEN** a subsequent project read returns the same resolved values without dropping unchanged sections

#### Scenario: Invalid governance thresholds are rejected
- **WHEN** an operator submits a project settings update whose threshold or policy values are internally inconsistent or outside supported ranges
- **THEN** the backend rejects the update with field-level validation errors
- **THEN** the previously persisted project settings remain unchanged

### Requirement: Project settings expose truthful governance diagnostics and summaries
The system SHALL expose a project-scoped settings summary or diagnostics surface that reflects the active runtime readiness, budget governance posture, review routing posture, and any fallback or blocking conditions relevant to operators.

#### Scenario: Selected coding-agent runtime is currently unavailable
- **WHEN** the project's selected coding-agent runtime has blocking diagnostics in the runtime catalog
- **THEN** the settings summary identifies that runtime as unavailable and surfaces the blocking reason
- **THEN** the operator can still inspect the persisted selection before choosing a new one

#### Scenario: Project policy requires manual approval
- **WHEN** the project's review policy is configured to require manual approval or equivalent escalation before final approval
- **THEN** the settings summary explicitly states that project reviews will pause for manual approval after deep review
- **THEN** the saved settings response preserves that review posture for later operator inspection
