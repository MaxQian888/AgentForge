## MODIFIED Requirements

### Requirement: The panel distinguishes built-in, local, and marketplace plugin sources
The system SHALL present available plugins by source channel instead of flattening every source into one list. The plugin management panel SHALL separately surface built-in plugin discoveries, installed plugins, local catalog entries, and remote registry marketplace entries, and SHALL indicate whether each available entry is currently installable with the platform's real capabilities. Remote registry entries MUST expose registry availability or source-health state so operators can distinguish a reachable remote marketplace from a disabled or failing one.

#### Scenario: Built-in plugin already installed
- **WHEN** a built-in plugin is already present in the installed plugin registry
- **THEN** the built-in availability section SHALL not present it as a duplicate install candidate
- **THEN** the installed section SHALL remain the authoritative place to manage that plugin

#### Scenario: Marketplace entry is browse-only
- **WHEN** a marketplace plugin entry lacks a real install source supported by the current platform contract
- **THEN** the marketplace section SHALL show the entry as browse-only or coming soon
- **THEN** the panel SHALL not present an enabled install action that implies unsupported remote installation

#### Scenario: Remote registry source is unavailable
- **WHEN** the configured remote registry is disabled, unreachable, or returns an invalid response
- **THEN** the remote marketplace section SHALL show the registry source as unavailable with a stable reason
- **THEN** the rest of the plugin management panel continues to function for installed and local plugin sources

### Requirement: Operators can install plugins through truthful source-aware flows
The system SHALL let operators initiate plugin installation only through source flows the current platform contract supports. The plugin management panel SHALL provide explicit local-path install, catalog-entry install, and supported remote-registry install actions, and MUST keep browse-only or unavailable entries non-installable when no supported install source is available.

#### Scenario: Operator installs a plugin from a local path
- **WHEN** the operator provides a filesystem path to a plugin directory or manifest file
- **THEN** the panel submits an explicit local install request instead of relying on discovery side effects
- **THEN** the installed plugin appears in the installed registry section only after the install request succeeds

#### Scenario: Operator installs a built-in or catalog entry explicitly
- **WHEN** the operator chooses an installable built-in or catalog entry from the availability sections
- **THEN** the panel triggers an explicit install action for that entry
- **THEN** the entry does not become an installed plugin record until that install action completes successfully

#### Scenario: Browse-only marketplace entry remains non-installable
- **WHEN** an availability entry lacks a real install source supported by the current platform contract
- **THEN** the panel shows the entry as browse-only or coming soon
- **THEN** the panel MUST NOT render an install action that implies unsupported remote installation

#### Scenario: Operator installs a plugin from the remote registry marketplace
- **WHEN** the operator chooses an installable remote marketplace entry from the remote registry section
- **THEN** the panel triggers the supported remote install action through the Go control plane instead of direct browser download
- **THEN** the entry only moves into the installed registry section after the remote install and verification flow succeeds

## ADDED Requirements

### Requirement: Remote registry install and availability states stay operator-visible in the panel
The plugin management panel SHALL surface remote registry availability, in-progress install state, install failure categories, and resulting source provenance for remote marketplace entries. Remote-source failures MUST remain scoped to the remote marketplace section and MUST NOT collapse installed-plugin diagnostics or local install actions.

#### Scenario: Remote install failure stays visible on the selected marketplace entry
- **WHEN** a remote marketplace install fails because of registry reachability, download, or trust verification
- **THEN** the panel shows the corresponding failure reason on the affected remote entry or detail surface
- **THEN** the operator can retry or inspect the failure without losing the rest of the plugin console state

#### Scenario: Successful remote install updates source provenance in details
- **WHEN** a remote marketplace entry installs successfully and becomes an installed plugin record
- **THEN** the plugin detail view shows the remote registry source provenance and selected version alongside trust and runtime metadata
- **THEN** the remote marketplace section reflects that the entry is already installed