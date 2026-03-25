## ADDED Requirements

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
