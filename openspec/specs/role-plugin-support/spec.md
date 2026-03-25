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
The system SHALL validate loaded role manifests against the full PRD-aligned Role YAML contract before they become visible to APIs, preview flows, sandbox flows, or execution paths. A valid role manifest MUST support structured metadata, identity, capabilities, knowledge, security, collaboration, overrides, and trigger sections, including advanced identity fields such as `persona`, `goals`, `constraints`, `personality`, `language`, and `response_style`; capability packages and tool host config; structured knowledge and memory blocks; security profile and governance fields; and collaboration or trigger definitions. The system MUST normalize compatible manifests into one internal role model so API handlers, preview flows, persistence, and execution code do not each maintain separate parsing logic.

#### Scenario: Full advanced PRD-aligned manifest is accepted
- **WHEN** a role manifest declares `apiVersion`, `kind: Role`, required metadata, and valid advanced sections for identity, capabilities, knowledge, security, collaboration, or triggers
- **THEN** the system accepts the manifest and stores it in normalized form for downstream list, get, preview, and sandbox use

#### Scenario: Invalid manifest is rejected before exposure
- **WHEN** a role manifest omits a required top-level field or provides an invalid nested advanced section such as malformed `response_style`, malformed MCP server config, or invalid memory governance structure
- **THEN** the system rejects the manifest and returns a validation error instead of exposing a partially loaded role

#### Scenario: Optional execution prompt can be synthesized from identity
- **WHEN** a valid role manifest omits an explicit `system_prompt` but provides identity fields such as role, goal, backstory, persona, or constraints
- **THEN** the system produces a deterministic normalized role record that remains executable without requiring the YAML author to duplicate prompt text everywhere

### Requirement: Role inheritance and security merge are resolved deterministically
The system SHALL support role inheritance through `extends` and related override semantics during role loading and preview. The resolved child role MUST inherit parent fields unless explicitly overridden, advanced collections and structured sections MUST merge using stable PRD-aligned semantics, and security or resource-governance settings MUST merge using the stricter effective constraint when parent and child disagree.

#### Scenario: Child role inherits parent identity and capabilities
- **WHEN** a role manifest extends a valid parent role and overrides only a subset of identity, capability, or knowledge fields
- **THEN** the resolved child role includes the inherited parent fields plus the child-specific overrides

#### Scenario: Child role merges advanced sections predictably
- **WHEN** a child role extends a parent that already defines capability packages, MCP tool hosts, shared knowledge, collaboration policies, or triggers and the child overrides only part of those sections
- **THEN** the system resolves the child role using documented merge rules for those sections instead of dropping inherited values or duplicating the same entry unpredictably

#### Scenario: Security settings merge to the stricter effective policy
- **WHEN** a parent role allows broader file access, looser governance, or higher budget ceilings than its child override
- **THEN** the resolved role uses the stricter allowed paths, resource limits, permission profile, or approval constraint instead of expanding the effective security envelope

#### Scenario: Cyclic inheritance is rejected
- **WHEN** role manifests create a circular `extends` chain
- **THEN** the system rejects the affected role load with an explicit inheritance error

### Requirement: Role APIs operate on normalized YAML-backed roles
The system SHALL expose list, get, create, and update behavior through one unified role registry or store backed by YAML assets instead of ad hoc file scanning inside handlers. API write operations MUST persist full advanced role manifests to the canonical directory layout, and subsequent reads MUST reflect the normalized role representation without dropping advanced sections such as packages, tool host config, knowledge or memory blocks, collaboration metadata, triggers, or overrides.

#### Scenario: Listing roles returns normalized roles with advanced sections
- **WHEN** a caller requests the role list
- **THEN** the system returns the roles loaded from the unified registry
- **THEN** each role includes the advanced structured fields that were normalized from its YAML-backed source of truth instead of omitting them from API responses

#### Scenario: Creating a role writes the canonical directory layout
- **WHEN** a caller creates a new valid role through the role API
- **THEN** the system persists it at `roles/<role-id>/role.yaml`
- **THEN** a subsequent get or list operation returns that role from the same normalized registry

#### Scenario: Updating a role preserves advanced YAML-backed source of truth
- **WHEN** a caller updates an existing role through the role API and the payload contains advanced identity, knowledge, collaboration, or trigger sections
- **THEN** the system rewrites the canonical YAML asset for that role instead of only mutating an in-memory entry
- **THEN** a subsequent get or list operation returns the same advanced sections in normalized form instead of silently dropping them

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

### Requirement: Role manifests preserve structured skill tree entries
The system SHALL allow role manifests to declare `capabilities.skills` as an ordered list of structured skill references. Each skill reference MUST include a `path` and MUST preserve its `auto_load` intent when roles are loaded from YAML, listed through the role API, created, updated, and written back to the canonical YAML-backed source of truth.

#### Scenario: Create and reload a role with mixed skill loading modes
- **WHEN** an operator creates or updates a role that declares both auto-loaded and on-demand skill references under `capabilities.skills`
- **THEN** a subsequent get or list operation returns the same ordered skill references with the same `path` and `auto_load` values
- **THEN** the canonical YAML-backed role asset preserves those skill references instead of dropping or flattening them

#### Scenario: Role without skills remains valid
- **WHEN** the system loads or saves a valid role manifest that omits `capabilities.skills`
- **THEN** the role remains valid without requiring placeholder skill entries
- **THEN** the normalized role record exposes an empty or omitted skills list instead of a malformed default entry

### Requirement: Role skill inheritance resolves deterministically
The system SHALL resolve inherited role skill references deterministically using skill `path` as the stable identity. Parent skill references SHALL be inherited in order, child role entries with the same `path` SHALL override the inherited entry, and empty or duplicate skill paths within one saved role SHALL be rejected before exposure through APIs or normalized role storage.

#### Scenario: Child role overrides inherited skill loading behavior
- **WHEN** a child role extends a parent role that already declares `skills/react` and the child redeclares `skills/react` with a different `auto_load` value while also appending `skills/testing`
- **THEN** the resolved child role contains one effective `skills/react` entry using the child's `auto_load` value
- **THEN** the resolved skill list preserves inherited order and appends the child's new skill references without duplicate paths

#### Scenario: Duplicate or blank skill path is rejected
- **WHEN** a role manifest contains an empty skill path or repeats the same skill path more than once within the saved role definition
- **THEN** the system rejects the role before it becomes visible through list/get APIs
- **THEN** the error explicitly points to invalid skill path configuration instead of silently deduplicating or dropping entries

### Requirement: Skill metadata does not break current execution profile projection
The system SHALL preserve role skill references in normalized role records without requiring the current execution profile projection to auto-load or expand those skills into unrelated runtime fields.

#### Scenario: Execution profile is derived from a role that declares skills
- **WHEN** the system derives an execution profile from a valid role that includes `capabilities.skills`
- **THEN** execution profile derivation still succeeds using the current runtime-facing fields
- **THEN** the role's structured skill references remain available through normalized role reads without being silently rewritten into tool or prompt fields

