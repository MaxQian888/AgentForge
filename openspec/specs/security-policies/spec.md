# security-policies Specification

## Purpose
Define the current declarative security-policy contract carried by role manifests, including parsing, inherited merge semantics, and runtime-facing policy projection.
## Requirements
### Requirement: Role manifests preserve the current security policy schema
The system SHALL parse and persist the current role security policy shape, including `profile`, `permission_mode`, `allowed_paths`, `denied_paths`, nested `permissions`, `output_filters`, and `resource_limits`.

#### Scenario: Parse advanced security policy fields
- **WHEN** a role manifest includes file access, network, and code-execution permissions together with output filters and resource limits
- **THEN** the parser preserves those fields in the normalized role manifest

#### Scenario: Round-trip security policy fields through role API updates
- **WHEN** a role is created or updated through the role authoring API with advanced security sections present
- **THEN** those security sections are preserved in the saved canonical role manifest

### Requirement: Role security inheritance resolves to stricter effective constraints
The system SHALL merge inherited security policies deterministically so that the effective role keeps the stricter constraint when parent and child security settings disagree.

#### Scenario: Permission mode resolves to stricter value
- **WHEN** parent and child roles specify different permission modes
- **THEN** the effective role uses the stricter permission mode

#### Scenario: Allowed paths resolve to stricter subset
- **WHEN** parent and child roles specify different allowed path sets
- **THEN** the effective role keeps the stricter allowed path result

#### Scenario: Output filters and denied paths merge without losing inherited protections
- **WHEN** parent and child roles both define output filters or denied paths
- **THEN** the effective role includes the merged protection set without dropping inherited entries

#### Scenario: Resource limits resolve to smaller effective ceilings
- **WHEN** parent and child roles define competing budget, rate, or execution limit values
- **THEN** the effective role uses the smaller positive effective limit for that resource

### Requirement: Runtime execution profiles project security-facing fields to the bridge contract
The system SHALL project role security configuration into the runtime execution profile consumed by the bridge-facing role config.

#### Scenario: Project permission mode and output filters
- **WHEN** an effective role includes `permission_mode` and `output_filters`
- **THEN** the execution profile exposes those values in the runtime role config

#### Scenario: Project budget and turn constraints together with security fields
- **WHEN** an effective role includes security budgets and execution ceilings
- **THEN** the execution profile includes the effective budget and governance fields used at runtime

### Requirement: Current security policy support is declarative rather than middleware-enforced
The system SHALL treat the current security policy implementation as declarative role contract support. Parsing, inheritance, preview, and execution-profile projection are supported today; repository-wide file, network, sandbox, and output-filter enforcement middleware is not yet guaranteed by this capability.

#### Scenario: Preview shows declarative security contract
- **WHEN** a caller previews or sandboxes a role with security policy fields
- **THEN** the response reflects the normalized and effective security contract
- **THEN** the system does not claim separate enforcement middleware beyond the current runtime contract projection
