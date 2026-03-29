## MODIFIED Requirements

### Requirement: Operators can preview the effective role manifest and execution profile before saving
The system SHALL provide a non-persistent role preview flow that accepts either a stored role reference or an unsaved role draft and returns the normalized effective role manifest plus the runtime-facing execution profile derived from that same authoritative normalization path. When the effective role includes skills, preview MUST also expose the execution-facing skill projection so operators can see which skills will load into runtime context and which remain on-demand inventory before saving.

#### Scenario: Preview an existing stored role
- **WHEN** the operator requests preview for an existing persisted role
- **THEN** the system returns the normalized role manifest, the effective inherited values, and the execution profile derived from that role without mutating the canonical YAML asset
- **THEN** any execution-facing skill projection in that role is included in the preview response

#### Scenario: Preview an unsaved child draft
- **WHEN** the operator requests preview for an unsaved role draft that extends a parent role
- **THEN** the system resolves inheritance and override semantics against the draft payload and existing role store
- **THEN** the preview response shows the effective child role state and the corresponding loaded-versus-available skill projection without persisting the draft

### Requirement: Sandbox validation reports authoritative issues and readiness state
The system SHALL validate role drafts through the preview or sandbox pipeline and report authoritative errors, warnings, and readiness diagnostics before an operator attempts a prompt probe or save. For role skills, unresolved auto-load references or unresolved auto-load dependencies MUST be reported as blocking readiness issues, while unresolved on-demand references MUST remain warning-only unless another runtime contract requires them.

#### Scenario: Sandbox reports invalid advanced configuration
- **WHEN** the operator submits a role draft with invalid advanced schema, incompatible merge state, or unsupported runtime governance settings
- **THEN** the system returns validation issues that point to the failing role section instead of silently ignoring the invalid configuration

#### Scenario: Sandbox reports runtime readiness blockers
- **WHEN** the operator requests a sandbox probe for a role whose selected runtime, provider, model, bridge prerequisites, or auto-load skill projection are not ready
- **THEN** the system returns readiness diagnostics describing the missing credentials, executables, incompatible selections, or blocking skill failures before any probe is executed

#### Scenario: Sandbox distinguishes blocking and warning-only skill gaps
- **WHEN** the effective role skill tree contains both an unresolved auto-load skill and an unresolved non-auto-load skill
- **THEN** the sandbox result marks the auto-load failure as blocking for execution-facing projection
- **THEN** the unresolved non-auto-load skill is surfaced as warning-only context instead of being promoted to the same blocking severity
