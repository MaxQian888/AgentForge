## MODIFIED Requirements

### Requirement: The panel distinguishes built-in, local, and marketplace plugin sources
The system SHALL present available plugins by source channel instead of flattening every source into one list. The plugin management panel SHALL separately surface built-in plugin discoveries, installed plugins, local catalog entries, remote registry marketplace entries, and installed plugins that originated from the standalone marketplace workspace, and SHALL indicate whether each available entry is currently installable with the platform's real capabilities. Remote registry entries MUST expose registry availability or source-health state so operators can distinguish a reachable remote marketplace from a disabled or failing one. Installed plugins that originated from the standalone marketplace workspace MUST preserve marketplace item identity, selected version, and a navigation path back to the originating marketplace surface instead of collapsing into anonymous local provenance.

#### Scenario: Built-in plugin already installed
- **WHEN** a built-in plugin is already present in the installed plugin registry
- **THEN** the built-in availability section SHALL not present it as a duplicate install candidate
- **THEN** the installed section SHALL remain the authoritative place to manage that plugin

#### Scenario: Standalone marketplace-sourced plugin remains identifiable after install
- **WHEN** an installed plugin originated from the standalone marketplace workspace
- **THEN** the installed section and detail surface SHALL show the marketplace provenance, originating item identity, and selected version
- **THEN** the operator SHALL be able to navigate from the plugin detail surface back to the originating marketplace item or management workspace

#### Scenario: Marketplace entry is browse-only
- **WHEN** a marketplace plugin entry lacks a real install source supported by the current platform contract
- **THEN** the marketplace section SHALL show the entry as browse-only or coming soon
- **THEN** the panel SHALL not present an enabled install action that implies unsupported remote installation

#### Scenario: Remote registry source is unavailable
- **WHEN** the configured remote registry is disabled, unreachable, or returns an invalid response
- **THEN** the remote marketplace section SHALL show the registry source as unavailable with a stable reason
- **THEN** the rest of the plugin management panel continues to function for installed and local plugin sources
