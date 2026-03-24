# role-plugin-support Specification

## Purpose
Define how AgentForge discovers, validates, normalizes, stores, and executes YAML-backed Role Plugins, including canonical role layout, inheritance behavior, API-backed role management, and Go-side projection into runtime execution profiles.
## Requirements
### Requirement: Role YAML manifests can be discovered from canonical and legacy locations
The system SHALL discover role manifests from the configured roles directory using the canonical PRD layout `roles/<role-id>/role.yaml`. During migration, the system MUST also accept legacy flat files ending in `.yaml` or `.yml` under the roles root, but canonical directory-based manifests MUST take precedence when both forms resolve to the same role identifier.

#### Scenario: Canonical directory role is loaded
- **WHEN** the roles directory contains `roles/frontend-developer/role.yaml` with a valid role manifest
- **THEN** the system loads that role and exposes it through the unified role registry

#### Scenario: Legacy flat role remains readable during migration
- **WHEN** the roles directory contains `roles/code-reviewer.yaml` with a valid legacy location
- **THEN** the system still loads the role instead of ignoring it solely because it is not yet in the canonical directory layout

#### Scenario: Canonical role overrides legacy duplicate
- **WHEN** both `roles/test-engineer/role.yaml` and `roles/test-engineer.yaml` describe the same role identifier
- **THEN** the canonical directory-based manifest is treated as the authoritative source
- **THEN** the legacy duplicate does not replace or shadow the canonical manifest

### Requirement: Role manifests are validated and normalized against the PRD schema
The system SHALL validate loaded role manifests against the PRD-aligned Role YAML contract before they become visible to APIs or execution flows. A valid role manifest MUST include `apiVersion`, `kind`, role identity metadata, and the structured sections needed for role loading. The system MUST normalize compatible manifests into one internal role model so API handlers and execution code do not each maintain separate parsing logic.

#### Scenario: Full PRD-aligned manifest is accepted
- **WHEN** a role manifest declares `apiVersion`, `kind: Role`, `metadata.id`, `metadata.name`, and valid structured role sections
- **THEN** the system accepts the manifest and stores it in normalized form for downstream use

#### Scenario: Invalid manifest is rejected before exposure
- **WHEN** a role manifest omits a required top-level identity field such as `apiVersion`, `kind`, or `metadata.id`
- **THEN** the system rejects the manifest and returns a validation error instead of exposing a partially loaded role

#### Scenario: Optional execution prompt can be synthesized from identity
- **WHEN** a valid role manifest omits an explicit `system_prompt` but provides identity fields such as role, goal, or backstory
- **THEN** the system produces a deterministic normalized role record that remains executable without requiring the YAML author to duplicate prompt text everywhere

### Requirement: Role inheritance and security merge are resolved deterministically
The system SHALL support role inheritance through `extends` and related override semantics during role loading. The resolved child role MUST inherit parent fields unless explicitly overridden, and security or resource-governance settings MUST merge using the stricter effective constraint when parent and child disagree.

#### Scenario: Child role inherits parent identity and capabilities
- **WHEN** a role manifest extends a valid parent role and overrides only a subset of identity or capability fields
- **THEN** the resolved child role includes the inherited parent fields plus the child-specific overrides

#### Scenario: Security settings merge to the stricter effective policy
- **WHEN** a parent role allows broader file access or a higher budget than its child override
- **THEN** the resolved role uses the stricter allowed paths, budget, or permission constraint instead of expanding the effective security envelope

#### Scenario: Cyclic inheritance is rejected
- **WHEN** role manifests create a circular `extends` chain
- **THEN** the system rejects the affected role load with an explicit inheritance error

### Requirement: Role APIs operate on normalized YAML-backed roles
The system SHALL expose list, get, create, and update behavior through one unified role registry/store backed by YAML assets instead of ad hoc file scanning inside handlers. API write operations MUST persist roles to the canonical directory layout and subsequent reads MUST reflect the normalized role representation.

#### Scenario: Listing roles returns normalized preset and disk roles
- **WHEN** a caller requests the role list
- **THEN** the system returns the roles loaded from the unified registry rather than a separate handler-local file scan

#### Scenario: Creating a role writes the canonical directory layout
- **WHEN** a caller creates a new valid role through the role API
- **THEN** the system persists it at `roles/<role-id>/role.yaml`
- **THEN** a subsequent get or list operation returns that role from the same normalized registry

#### Scenario: Updating a role preserves YAML-backed source of truth
- **WHEN** a caller updates an existing role through the role API
- **THEN** the system rewrites the canonical YAML asset for that role instead of only mutating an in-memory preset entry

### Requirement: Role manifests can be projected into execution profiles
The system SHALL derive a normalized execution profile from a resolved role manifest for downstream agent execution. The execution profile MUST include the runtime-facing role data needed by the current execution contract, such as the effective role name, system prompt, tool allowlist, budget or turn limits, and permission mode, while preserving richer PRD-only fields in the stored role model.

#### Scenario: Execution profile is derived from a resolved role
- **WHEN** the system resolves a valid role manifest for execution use
- **THEN** it emits a normalized execution profile containing the runtime-facing prompt, tool, budget, and permission settings derived from that role

#### Scenario: Execution profile is built from the fully resolved role
- **WHEN** a child role inherits settings from a parent role
- **THEN** the derived execution profile reflects the post-merge effective values instead of only the child YAML fragment

#### Scenario: Non-runtime role metadata remains available without leaking into execution config
- **WHEN** a role manifest contains collaboration, memory, or trigger metadata that the current runtime does not yet execute
- **THEN** the system preserves that metadata in the normalized role record
- **THEN** the execution profile omits or ignores those unsupported fields rather than failing valid role loading

### Requirement: Agent spawn requests can bind a role reference that resolves in Go
The Go orchestrator SHALL allow agent startup requests to reference an existing role by `roleId`, resolve that role through the unified YAML-backed role store, and forward the resulting normalized execution profile to the runtime bridge. The startup path MUST reject unknown role references before bridge execution begins, and persisted agent run records MUST retain the referenced `role_id` for later inspection.

#### Scenario: Spawn request with roleId injects the resolved execution profile
- **WHEN** a caller starts an agent run with a valid `roleId`
- **THEN** Go resolves that role through the unified store before execution
- **THEN** the bridge execute request receives the normalized `role_config` derived from the resolved role
- **THEN** the created agent run record stores the selected `role_id`

#### Scenario: Spawn request with unknown roleId is rejected
- **WHEN** a caller starts an agent run with a `roleId` that does not exist in the configured roles store
- **THEN** the system returns a not-found error
- **THEN** no bridge execute request is started for that agent run
