# role-authoring-sandbox Specification

## Purpose
Define the non-persistent preview and sandbox workflow for role authoring so operators can inspect effective manifests, execution profiles, readiness diagnostics, and bounded prompt probes before saving or launching a role.
## Requirements
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

### Requirement: Preview and sandbox expose advanced field provenance and persistence context
The system SHALL let preview and sandbox results explain the effective state of advanced role fields in a way that is useful for authoring. For advanced sections such as custom settings, inherited tool-host data, structured knowledge or memory metadata, and overrides, the role authoring flow MUST expose enough context to tell the operator whether the effective value comes from the current draft, an inherited parent, or preserved existing manifest state, and whether that section persists only in canonical YAML or also affects the current execution profile.

#### Scenario: Preview shows effective advanced values for a child role
- **WHEN** the operator previews an unsaved child draft that inherits advanced fields and overrides part of them
- **THEN** the preview result shows the effective advanced values after inheritance and override resolution
- **THEN** the authoring flow can indicate which advanced values are inherited versus explicitly supplied by the draft

#### Scenario: Review context distinguishes persisted role data from runtime projection
- **WHEN** the operator inspects preview or sandbox output for a role that includes memory, collaboration, trigger, or override metadata beyond the current runtime contract
- **THEN** the authoring flow identifies that those sections remain part of the canonical role definition
- **THEN** the operator can see that unsupported runtime-only omissions are projection behavior rather than data loss

### Requirement: Preview and sandbox report advanced authoring validation failures without silent fallback
The system SHALL reject malformed advanced authoring input through preview or sandbox with section-specific validation feedback instead of silently dropping the invalid subsection and continuing with a degraded manifest.

#### Scenario: Sandbox rejects malformed advanced override input
- **WHEN** the operator runs sandbox on a draft whose advanced override block is malformed or semantically incompatible with the current role schema
- **THEN** the system returns a validation issue that points to the override section
- **THEN** the sandbox flow does not proceed as if the invalid override had simply been omitted

#### Scenario: Preview rejects malformed advanced metadata rows
- **WHEN** the operator previews a draft with invalid advanced subsection content such as malformed custom settings entries or inconsistent shared knowledge source metadata
- **THEN** the system returns authoritative validation feedback for the failing subsection
- **THEN** the operator can correct the issue before any save attempt or sandbox probe

### Requirement: Preview and sandbox explain role-skill resolution without conflating it with runtime readiness
The system SHALL include role-skill resolution and compatibility context in preview and sandbox authoring feedback so operators can see which configured skill references are resolved by the authoritative repository catalog, which are inherited or template-derived, which transitive skills will load through dependency closure, and what declared tool requirements apply to the effective role-skill tree. This feedback MUST distinguish authoring-level resolution warnings from runtime readiness blockers such as missing auto-load skills or incompatible auto-load tool requirements.

#### Scenario: Preview shows effective skill-tree resolution and compatibility for an unsaved child draft
- **WHEN** the operator previews an unsaved draft whose effective skill tree includes inherited skills, template-copied skills, newly added manual references, and auto-load dependencies
- **THEN** the preview result shows the effective role skills after inheritance or template application together with any transitive loaded skills
- **THEN** the authoring flow can indicate for each effective skill whether it is catalog-resolved, inherited, template-derived, unresolved, and whether its declared tool requirements are currently compatible with the effective role

#### Scenario: Sandbox separates warning-only inventory gaps from blocking compatibility failures
- **WHEN** the operator runs sandbox for a valid draft that includes both a non-auto-load unresolved manual skill reference and an auto-load skill whose declared tool requirements are not covered by the effective role tool inventory
- **THEN** the sandbox result reports the unresolved non-auto-load skill as warning-only authoring or inventory context
- **THEN** the auto-load compatibility failure is returned as a blocking readiness issue before any probe is executed

