# role-management Specification

## Purpose
Define the current Go role management contract for advanced role manifests, canonical storage layout, effective-role resolution, preview and sandbox flows, and runtime execution projection.
## Requirements
### Requirement: Role manifests support the current advanced authoring schema
The system SHALL parse, normalize, and round-trip the current role manifest shape, including `apiVersion`, `kind`, `metadata`, `identity`, top-level `system_prompt`, `capabilities`, `knowledge`, `security`, `extends`, `overrides`, `collaboration`, and `triggers`.

#### Scenario: Parse advanced role authoring sections
- **WHEN** a role manifest includes response style, packages, built-in and external tools, MCP servers, custom settings, shared knowledge sources, memory settings, collaboration preferences, and triggers
- **THEN** the parser preserves those sections in the normalized role manifest

#### Scenario: Reject invalid role manifest shape
- **WHEN** a role manifest omits the required role identity or uses invalid structured values
- **THEN** parsing fails with a validation error instead of silently accepting the manifest

### Requirement: Canonical role layout is authoritative while legacy flat files remain readable
The system SHALL treat `roles/<role-id>/role.yaml` as the canonical on-disk layout for roles. During migration, the loader MUST still read legacy flat `.yaml` and `.yml` files in the roles root when no canonical manifest exists for the same role ID.

#### Scenario: Load canonical directory role
- **WHEN** the roles directory contains `roles/frontend-developer/role.yaml`
- **THEN** the role registry discovers and loads that canonical role manifest

#### Scenario: Read legacy flat role only when canonical manifest is absent
- **WHEN** the roles directory contains `roles/code-reviewer.yaml` and no canonical duplicate
- **THEN** the registry still loads the legacy role manifest for compatibility

#### Scenario: Canonical manifest wins over legacy duplicate
- **WHEN** both `roles/test-engineer/role.yaml` and `roles/test-engineer.yaml` resolve to the same role ID
- **THEN** the canonical directory manifest is the authoritative source
- **THEN** the legacy duplicate does not override the canonical role

### Requirement: Role inheritance resolves deterministically with stricter governance merging
The system SHALL resolve `extends` at load and preview time. Child manifests MUST inherit parent fields unless explicitly overridden, while security and resource-governance settings MUST merge using the stricter effective constraint.

#### Scenario: Child role inherits parent sections
- **WHEN** a child role extends a valid parent and overrides only part of its identity or capabilities
- **THEN** the effective role keeps inherited parent fields alongside the child overrides

#### Scenario: Security merge prefers stricter child-effective result
- **WHEN** parent and child disagree on allowed paths, budget, permission mode, or output filters
- **THEN** the effective role uses the stricter merged governance values

#### Scenario: Dependency cycle or missing parent blocks resolution
- **WHEN** a role extends a missing parent or introduces a cyclic inheritance chain
- **THEN** role resolution fails with an explicit error

### Requirement: Role execution profiles project runtime-facing fields from the effective manifest
The system SHALL build an execution profile from the effective role manifest for runtime consumers. The profile MUST project synthesized system prompt content, allowed tools, external and MCP tool IDs, knowledge context, output filters, budget and turn limits, permission mode, and runtime skill readiness.

#### Scenario: Project runtime-facing tool and knowledge fields
- **WHEN** an effective role includes external tools, MCP servers, documents, repositories, and shared knowledge sources
- **THEN** the execution profile exposes normalized runtime tool IDs and a knowledge context string derived from those sources

#### Scenario: Auto-load skills become loaded runtime instructions
- **WHEN** an effective role references skills with `auto_load: true` and those skill files exist under the configured skill root
- **THEN** the execution profile includes those skills as loaded runtime instructions

#### Scenario: On-demand skills remain inventory entries
- **WHEN** an effective role references skills with `auto_load: false`
- **THEN** the execution profile keeps them as available runtime inventory instead of injecting their instructions

#### Scenario: Missing auto-load skill produces blocking readiness diagnostic
- **WHEN** an auto-load skill cannot be resolved from the configured skill root
- **THEN** the execution profile includes a blocking skill diagnostic for that missing skill

### Requirement: Role authoring preview returns normalized and effective role views
The system SHALL provide preview and sandbox flows that expose both the normalized draft manifest and the effective manifest after inheritance resolution, together with execution profile and readiness diagnostics.

#### Scenario: Preview unsaved draft against parent role
- **WHEN** the caller previews a draft role that extends an existing parent
- **THEN** the response includes both `normalizedManifest` and inherited `effectiveManifest`
- **THEN** the response includes the derived execution profile for that effective role

#### Scenario: Preview reports inheritance summary
- **WHEN** the normalized role manifest extends another role
- **THEN** the preview response includes the parent role ID in its inheritance summary

#### Scenario: Sandbox returns blocking diagnostics without probing the runtime
- **WHEN** runtime readiness or skill diagnostics are blocking
- **THEN** the sandbox response returns readiness diagnostics
- **THEN** the bridge probe is not executed

#### Scenario: Sandbox runs bounded probe when runtime is ready
- **WHEN** runtime readiness is non-blocking and the caller supplies sandbox input
- **THEN** the sandbox flow runs a bounded bridge generation probe using the selected runtime/provider/model
- **THEN** the response includes the probe result together with normalized and effective role views

### Requirement: Partial role updates preserve advanced sections unless explicitly replaced
The system SHALL preserve advanced role sections during partial updates and previews when the incoming payload omits those sections, while still allowing explicit clearing when the caller sends the field intentionally.

#### Scenario: Partial update keeps advanced authoring sections
- **WHEN** an existing role already stores custom settings, knowledge sources, or overrides and an update payload omits those fields
- **THEN** the saved manifest preserves the existing advanced sections

#### Scenario: Explicit field removal clears preserved advanced section
- **WHEN** the caller explicitly sends an empty advanced section in the payload
- **THEN** the saved or previewed manifest clears that section instead of restoring the previous value
