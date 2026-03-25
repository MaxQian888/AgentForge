## ADDED Requirements

### Requirement: Operators can preview the effective role manifest and execution profile before saving
The system SHALL provide a non-persistent role preview flow that accepts either a stored role reference or an unsaved role draft and returns the normalized effective role manifest plus the runtime-facing execution profile derived from that same authoritative normalization path.

#### Scenario: Preview an existing stored role
- **WHEN** the operator requests preview for an existing persisted role
- **THEN** the system returns the normalized role manifest, the effective inherited values, and the execution profile derived from that role without mutating the canonical YAML asset

#### Scenario: Preview an unsaved child draft
- **WHEN** the operator requests preview for an unsaved role draft that extends a parent role
- **THEN** the system resolves inheritance and override semantics against the draft payload and existing role store
- **THEN** the preview response shows the effective child role state without persisting the draft

### Requirement: Sandbox validation reports authoritative issues and readiness state
The system SHALL validate role drafts through the preview or sandbox pipeline and report authoritative errors, warnings, and readiness diagnostics before an operator attempts a prompt probe or save.

#### Scenario: Sandbox reports invalid advanced configuration
- **WHEN** the operator submits a role draft with invalid advanced schema, incompatible merge state, or unsupported runtime governance settings
- **THEN** the system returns validation issues that point to the failing role section instead of silently ignoring the invalid configuration

#### Scenario: Sandbox reports runtime readiness blockers
- **WHEN** the operator requests a sandbox probe for a role whose selected runtime, provider, model, or bridge prerequisites are not ready
- **THEN** the system returns readiness diagnostics describing the missing credentials, executables, or incompatible selections before any probe is executed

### Requirement: Operators can run a bounded non-persistent role probe
The system SHALL support a bounded sandbox probe that uses the resolved role definition and a caller-supplied test input to generate a sample role response without creating a task, worktree, or persisted agent run.

#### Scenario: Successful prompt probe returns sample output
- **WHEN** the operator submits a valid role draft and a supported sandbox test input
- **THEN** the system executes a bounded non-persistent probe using the resolved role prompt and selected runtime tuple
- **THEN** the response returns sample output, probe diagnostics, and the execution context used for that probe

#### Scenario: Sandbox probe does not mutate persisted role state
- **WHEN** the operator completes a successful or failed sandbox probe for an unsaved draft
- **THEN** the system does not create or update `roles/<role-id>/role.yaml`
- **THEN** the probe result remains an authoring aid rather than an implicit save or runtime side effect
