## MODIFIED Requirements

### Requirement: Plugin installation supports multiple normalized source types
The system SHALL support plugin installation records for built-in, local path, Git, npm package or tarball, configured catalog sources, and configured remote registry sources using one normalized source model in the registry. Remote registry installs MUST preserve the registry identity, selected entry, requested version, and any resolved artifact metadata needed to audit how the plugin entered the system.

#### Scenario: Git-sourced plugin is installed
- **WHEN** an operator installs a plugin from a supported Git source
- **THEN** the registry stores the repository source, resolved reference, and installed plugin metadata in the same normalized record shape used for other sources

#### Scenario: npm-sourced plugin is installed
- **WHEN** an operator installs a plugin from a supported npm package or tarball source
- **THEN** the registry stores the package identity, version, and resolved artifact metadata needed to audit the installed plugin

#### Scenario: Remote registry plugin is installed
- **WHEN** an operator installs a plugin from a configured remote registry entry
- **THEN** the registry stores the remote registry identity, selected entry, requested version, and resolved artifact metadata in the normalized source model used by other external sources

### Requirement: External plugin installation and update flows verify integrity before enablement
The system SHALL verify digest and trust metadata for external plugin artifacts before they become enabled, and MUST preserve signature or approval status in the registry so operators can distinguish verified plugins from untrusted ones. Remote registry artifacts MUST pass the same verification and approval gates as other external sources, and verification failures MUST be surfaced as stable operator-facing results instead of generic transport errors.

#### Scenario: Signed external plugin is marked verified
- **WHEN** an external plugin artifact includes a valid digest and a signature that chains to a configured trust source
- **THEN** the registry marks the plugin as verified and allows normal enablement flow

#### Scenario: Untrusted external plugin remains blocked from activation
- **WHEN** an external plugin artifact lacks valid trust metadata or fails signature verification
- **THEN** the registry records the plugin as untrusted and blocks activation until an operator-approved trust path is satisfied

#### Scenario: Remote registry artifact fails verification after download
- **WHEN** a remote registry artifact downloads successfully but fails digest, signature, approval, or policy checks
- **THEN** the install flow records the related verification or approval failure without enabling the plugin
- **THEN** the operator-visible result distinguishes verification blocking from remote source connectivity failure

### Requirement: Plugin release history and trust state remain operator-visible
The system SHALL preserve operator-visible release metadata for installed plugins, including current version, available update information, last installed digest, approval state, last trust decision, and source identity. For plugins installed from a remote registry, the operator-visible metadata MUST also retain the originating registry URL or identifier and the selected registry entry or version used for installation.

#### Scenario: Plugin update preserves identity and release history
- **WHEN** an installed plugin is updated to a newer artifact
- **THEN** the registry keeps the same plugin identity while recording the new version metadata and the prior installed artifact history needed for auditability

#### Scenario: Remote registry install preserves source provenance
- **WHEN** a plugin is installed or updated from a configured remote registry entry
- **THEN** the operator-visible record preserves the originating registry identity and selected remote version alongside trust and release metadata
- **THEN** installed plugin diagnostics can show where that artifact was sourced without requiring backend log inspection