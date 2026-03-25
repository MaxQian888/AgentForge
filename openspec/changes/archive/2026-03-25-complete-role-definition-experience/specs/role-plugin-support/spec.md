## MODIFIED Requirements

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
