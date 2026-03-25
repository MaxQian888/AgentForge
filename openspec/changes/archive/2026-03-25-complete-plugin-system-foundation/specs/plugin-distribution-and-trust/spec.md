## ADDED Requirements

### Requirement: Plugin installation supports multiple normalized source types
The system SHALL support plugin installation records for built-in, local path, Git, npm package or tarball, and configured catalog or registry sources using one normalized source model in the registry.

#### Scenario: Git-sourced plugin is installed
- **WHEN** an operator installs a plugin from a supported Git source
- **THEN** the registry stores the repository source, resolved reference, and installed plugin metadata in the same normalized record shape used for other sources

#### Scenario: npm-sourced plugin is installed
- **WHEN** an operator installs a plugin from a supported npm package or tarball source
- **THEN** the registry stores the package identity, version, and resolved artifact metadata needed to audit the installed plugin

### Requirement: External plugin installation and update flows verify integrity before enablement
The system SHALL verify digest and trust metadata for external plugin artifacts before they become enabled, and MUST preserve signature or approval status in the registry so operators can distinguish verified plugins from untrusted ones.

#### Scenario: Signed external plugin is marked verified
- **WHEN** an external plugin artifact includes a valid digest and a signature that chains to a configured trust source
- **THEN** the registry marks the plugin as verified and allows normal enablement flow

#### Scenario: Untrusted external plugin remains blocked from activation
- **WHEN** an external plugin artifact lacks valid trust metadata or fails signature verification
- **THEN** the registry records the plugin as untrusted and blocks activation until an operator-approved trust path is satisfied

### Requirement: Configured plugin catalogs can be searched before installation
The system SHALL expose configured plugin catalog entries separately from installed plugin records so operators can search, inspect, and select installable plugins before installation without requiring a public marketplace UI.

#### Scenario: Catalog search returns installable plugin metadata
- **WHEN** an operator searches a configured plugin catalog
- **THEN** the platform returns matching catalog entries with source, version, capability, and trust metadata without creating installed plugin records yet

#### Scenario: Catalog entry can be promoted into an installed plugin record
- **WHEN** an operator chooses a catalog entry to install
- **THEN** the platform creates or updates the installed plugin record using the selected catalog source metadata and normal verification flow

### Requirement: Plugin release history and trust state remain operator-visible
The system SHALL preserve operator-visible release metadata for installed plugins, including current version, available update information, last installed digest, approval state, and last trust decision.

#### Scenario: Plugin update preserves identity and release history
- **WHEN** an installed plugin is updated to a newer artifact
- **THEN** the registry keeps the same plugin identity while recording the new version metadata and the prior installed artifact history needed for auditability

