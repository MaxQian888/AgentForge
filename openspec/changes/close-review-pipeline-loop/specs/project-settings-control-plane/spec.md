## MODIFIED Requirements

### Requirement: Project settings save as one structured document
The system SHALL persist project governance settings as one structured settings document returned through the existing project read and update surfaces. A valid save MUST preserve unchanged sections, while invalid threshold or policy combinations MUST be rejected with actionable validation feedback. The persisted document SHALL include `reviewPolicy` as a first-class sub-document within the governance settings, and the merge logic MUST NOT drop `reviewPolicy` fields when other sections are updated independently.

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

## ADDED Requirements

### Requirement: Project settings API returns reviewPolicy as a structured response field
The system SHALL include `reviewPolicy` in the project settings GET response as a typed sub-document with `requiredLayers`, `requireManualApproval`, and `minRiskLevelForBlock`. When no policy has been saved, the response SHALL return default values (empty requiredLayers, requireManualApproval false, no minRiskLevelForBlock) rather than omitting the field.

#### Scenario: GET project settings returns reviewPolicy field
- **WHEN** an authenticated caller reads project settings for any project
- **THEN** the response includes a `reviewPolicy` object with at minimum `requireManualApproval` (boolean) and `requiredLayers` (array)
- **THEN** projects with no saved policy return the safe default values rather than null or absent fields
