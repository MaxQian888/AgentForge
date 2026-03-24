## ADDED Requirements

### Requirement: Operators can browse installed plugins with operational filters and details
The system SHALL provide a plugin management panel that lists installed plugins and allows operators to filter by plugin kind, lifecycle state, runtime host, source type, and free-text search. The panel SHALL allow an operator to open a detail view for any listed plugin and inspect its declared runtime, declared permissions, resolved source information, runtime metadata, restart count, last health timestamp, and last error summary.

#### Scenario: Filter the installed plugin list
- **WHEN** the operator applies one or more filters such as `ToolPlugin`, `active`, `ts-bridge`, or a search term
- **THEN** the panel shows only installed plugins matching the selected criteria
- **THEN** clearing the filters restores the full installed list without reloading the page

#### Scenario: Inspect plugin operational details
- **WHEN** the operator opens the detail view for an installed plugin
- **THEN** the panel shows the plugin's runtime declaration, runtime host, permissions, resolved source path, runtime metadata, restart count, last health timestamp, and last error state when present

### Requirement: The panel distinguishes built-in, local, and marketplace plugin sources
The system SHALL present available plugins by source channel instead of flattening every source into one list. The plugin management panel SHALL separately surface built-in plugin discoveries, installed plugins, and marketplace directory entries, and SHALL indicate whether each available entry is currently installable with the platform's real capabilities.

#### Scenario: Built-in plugin already installed
- **WHEN** a built-in plugin is already present in the installed plugin registry
- **THEN** the built-in availability section SHALL not present it as a duplicate install candidate
- **THEN** the installed section SHALL remain the authoritative place to manage that plugin

#### Scenario: Marketplace entry is browse-only
- **WHEN** a marketplace plugin entry lacks a real install source supported by the current platform contract
- **THEN** the marketplace section SHALL show the entry as browse-only or coming soon
- **THEN** the panel SHALL not present an enabled install action that implies unsupported remote installation

### Requirement: Lifecycle actions are gated by executable capability and current state
The system SHALL render lifecycle actions in the plugin panel according to the selected plugin's kind, runtime, and current lifecycle state. The panel SHALL expose only actions that are valid for the current record and SHALL explain unavailable actions for disabled, non-executable, or unsupported runtime combinations.

#### Scenario: Disabled plugin cannot be activated directly
- **WHEN** the operator selects a plugin whose registry state is `disabled`
- **THEN** the plugin panel SHALL not present activation as an immediately available action
- **THEN** the panel SHALL require the plugin to be re-enabled before activation can be attempted

#### Scenario: Non-executable plugin shows why runtime actions are unavailable
- **WHEN** the operator views a plugin that is not executable in the current phase, such as a declarative-only role plugin
- **THEN** the panel SHALL suppress restart and health actions that require a live runtime host
- **THEN** the panel SHALL explain that the plugin does not currently run through an executable plugin runtime
