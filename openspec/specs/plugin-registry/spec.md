# plugin-registry Specification

## Purpose
Define AgentForge's authoritative plugin registry model for installed plugins, including source tracking, operator-visible lifecycle state, filtering, enablement controls, and runtime-state reconciliation from plugin hosts.
## Requirements
### Requirement: Registry stores the authoritative plugin record
The system SHALL maintain a registry record for each installed plugin as the authoritative source of truth for plugin metadata and operator-visible state. Each record MUST include plugin identifier, plugin kind, version, install source, declared runtime, declared permissions, current lifecycle state, and manifest payload or its normalized equivalent. For plugins that declare `runtime: wasm`, the record MUST also preserve the resolved module source and SDK ABI compatibility metadata needed to audit which runtime artifact is expected to execute. For externally sourced plugins, the record MUST also preserve digest, signature or trust metadata, approval state, and the normalized release metadata needed to audit how the plugin was installed.

#### Scenario: New external plugin registration creates a trust-aware registry record
- **WHEN** a plugin is installed from a supported external source
- **THEN** the platform creates a registry record containing its metadata, source, runtime declaration, trust metadata, and initial lifecycle state

#### Scenario: Registry record survives runtime and source ownership boundaries
- **WHEN** a plugin is managed by the TS bridge runtime or the Go orchestrator runtime and originates from any supported source type
- **THEN** operators still see one registry record for that plugin instead of separate per-runtime or per-source records

### Requirement: Registry supports local and built-in plugin sources
The system SHALL support registering plugins from built-in, local path, Git, npm package or tarball, and configured catalog or registry sources. The registry MUST preserve normalized source details so future marketplace integrations can be added without changing plugin identity semantics.

#### Scenario: Built-in plugin is discovered
- **WHEN** the platform scans built-in plugin definitions
- **THEN** it registers each built-in plugin with a source type that identifies it as platform-provided

#### Scenario: Git plugin is registered from a repository installation
- **WHEN** an operator installs a plugin from a supported Git source
- **THEN** the registry stores the repository source details alongside the plugin metadata and lifecycle state

#### Scenario: Catalog-backed plugin is registered from a selected entry
- **WHEN** an operator installs a plugin from a configured catalog or registry entry
- **THEN** the registry stores the selected catalog source details alongside the installed plugin metadata and lifecycle state

### Requirement: Registry supports operational plugin management
The system SHALL allow operators and internal services to query plugins by kind, lifecycle state, trust state, and source type, and to change whether a plugin is enabled, disabled, approved, updated, or uninstalled. A plugin MUST NOT be activated while its registry state is `disabled`, and an external plugin that remains untrusted or unapproved MUST NOT be activated.

#### Scenario: Operator filters plugins by state and trust
- **WHEN** an operator requests the plugin list with kind, lifecycle, or trust filters
- **THEN** the platform returns only the plugins matching those registry filters

#### Scenario: Unapproved external plugin cannot be activated
- **WHEN** an externally sourced plugin remains untrusted or lacks the required approval decision
- **THEN** the platform refuses activation requests until the plugin is approved through the supported trust flow

#### Scenario: Update preserves plugin identity
- **WHEN** an operator updates an installed plugin to a newer source artifact
- **THEN** the registry preserves the existing plugin identity while recording the new version, source, and trust metadata for that plugin

### Requirement: Registry synchronizes runtime state from plugin hosts
The registry SHALL accept runtime state updates from the plugin host that owns execution and reconcile them into the authoritative plugin record. Runtime updates for Go-hosted WASM plugins MUST reflect actual instantiation, initialization, health, and restart outcomes. Runtime updates for TS-hosted Tool or Review plugins MUST reflect actual bridge-side activation and execution outcomes. Runtime success MUST NOT be inferred before the owning host completes its handshake. Runtime updates MUST NOT bypass the registry or create a second source of truth.

#### Scenario: TS bridge reports a review plugin runtime state change
- **WHEN** the TS bridge reports that a review plugin has become `active` or `degraded`
- **THEN** the registry updates the corresponding plugin record with the new lifecycle state and operational details

#### Scenario: Go runtime reports successful workflow activation
- **WHEN** the Go runtime reports that a workflow plugin has finished activation successfully
- **THEN** the registry updates the corresponding plugin record with the new lifecycle state and operational details

#### Scenario: Go runtime reports a WASM instantiation failure
- **WHEN** the Go runtime cannot instantiate or initialize a WASM plugin
- **THEN** the registry updates the corresponding plugin record with a degraded state and the reported error without requiring a separate manual sync step

