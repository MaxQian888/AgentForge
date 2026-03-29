## MODIFIED Requirements

### Requirement: Role manifests can be projected into execution profiles
The system SHALL derive a normalized execution profile from a resolved role manifest for downstream agent execution. The execution profile MUST include the runtime-facing role data needed by the current Bridge contract, including the effective role identifier and name, system prompt, tool allowlist, bridge tool or plugin identifiers, injected knowledge context, runtime-facing skill projection, output filters, budget or turn limits, and permission mode, while preserving richer PRD-only fields in the stored role model.

#### Scenario: Execution profile is derived from a resolved role
- **WHEN** the system resolves a valid role manifest for execution use
- **THEN** it emits a normalized execution profile containing the runtime-facing prompt, tool allowlist, plugin identifiers, knowledge context, skill projection, output filters, budget, turn, and permission settings derived from that role

#### Scenario: Execution profile is built from the fully resolved role
- **WHEN** a child role inherits settings from a parent role
- **THEN** the derived execution profile reflects the post-merge effective values for prompt, tools, knowledge context, skill projection, output filters, and guardrails instead of only the child YAML fragment

#### Scenario: Non-runtime role metadata remains available without leaking into execution config
- **WHEN** a role manifest contains collaboration, memory, or trigger metadata that the current bridge runtime path does not yet execute
- **THEN** the system preserves that metadata in the normalized role record
- **THEN** the execution profile excludes those unsupported sections rather than silently dropping the stored data or sending raw YAML-shaped payloads to the Bridge

### Requirement: Agent spawn requests can bind a role reference that resolves in Go
The Go orchestrator SHALL allow agent startup requests to reference an existing role by `roleId`, resolve that role through the unified YAML-backed role store, and forward the resulting normalized execution profile to the runtime bridge. The startup path MUST reject unknown role references or blocking role-skill runtime projection failures before bridge execution begins, and persisted agent run records MUST retain the referenced `role_id` for later inspection.

#### Scenario: Spawn request with roleId injects the resolved execution profile
- **WHEN** a caller starts an agent run with a valid `roleId`
- **THEN** Go resolves that role through the unified store before execution
- **THEN** the bridge execute request receives the normalized `role_config` derived from the resolved role
- **THEN** the created agent run record stores the selected `role_id`

#### Scenario: Spawn request with unknown roleId is rejected
- **WHEN** a caller starts an agent run with a `roleId` that does not exist in the configured roles store
- **THEN** the system returns a not-found error
- **THEN** no bridge execute request is started for that agent run

#### Scenario: Spawn request with unresolved auto-load skill is rejected
- **WHEN** a caller starts an agent run with a role whose effective skill tree includes an unresolved auto-load skill or unresolved auto-load dependency
- **THEN** the system returns a role-skill-specific blocking error before bridge execution begins
- **THEN** the role's unresolved on-demand skills do not trigger the same hard failure unless another execution contract explicitly requires them

### Requirement: Skill metadata does not break current execution profile projection
The system SHALL preserve role skill references in normalized role records and SHALL also project runtime-consumable skill context from the effective skill tree when execution requires it. Auto-load skills MUST become execution-facing loaded context, non-auto-load skills MUST remain available runtime inventory, and unsupported or unresolved skill references MUST never be silently rewritten into unrelated tool or prompt fields.

#### Scenario: Execution profile is derived from a role that declares skills
- **WHEN** the system derives an execution profile from a valid role that includes `capabilities.skills`
- **THEN** execution profile derivation succeeds with explicit runtime-facing skill projection rather than dropping the role's effective skill tree
- **THEN** the role's structured skill references remain available through normalized role reads without being silently rewritten into tool or security fields

#### Scenario: Execution profile distinguishes loaded from available skills
- **WHEN** the effective role skill tree contains both auto-load and non-auto-load entries
- **THEN** the execution profile exposes which skill context was loaded for prompt assembly
- **THEN** non-auto-load skills remain available inventory metadata instead of being preloaded or silently omitted
