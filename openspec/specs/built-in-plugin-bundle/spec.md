# built-in-plugin-bundle Specification

## Purpose
Define the official repository-owned built-in plugin bundle so AgentForge exposes only maintained manifest-backed built-ins with aligned discovery, docs, and verification metadata.
## Requirements
### Requirement: Official built-in plugin bundle is explicitly declared
The repository SHALL maintain an explicit built-in plugin bundle that lists the official manifest-backed plugins shipped with the current checkout. Each bundle entry MUST declare the plugin identifier, plugin kind, manifest path, and bundle membership state required for operator-facing discovery. Only bundle entries that resolve to a valid manifest MAY be treated as official built-ins by the platform.

#### Scenario: Official bundle entry resolves to a valid manifest
- **WHEN** the repository declares a built-in bundle entry whose manifest path exists and parses successfully
- **THEN** the platform treats that plugin as an official built-in candidate for discovery, catalog, and installation flows

#### Scenario: Unlisted manifest is not treated as an official built-in
- **WHEN** a manifest exists somewhere under plugin development paths but is not declared in the built-in bundle
- **THEN** the platform does not surface that manifest as an official built-in plugin in operator-facing built-in discovery

### Requirement: Bundle entries carry docs, verification, and availability metadata
Each official built-in bundle entry SHALL include the docs reference, verification profile, and operator-facing availability metadata needed to keep repository assets, documentation, and control-plane views aligned. Availability metadata MUST distinguish immediately installable entries from entries that require local prerequisites or secret configuration before activation can succeed.

#### Scenario: Built-in entry requires a local prerequisite
- **WHEN** an official built-in plugin depends on a local binary, package manager tool, or fixture service before it can run
- **THEN** the bundle metadata records that prerequisite and the control plane can surface the plugin as available but not immediately runnable

#### Scenario: Verification can trace a bundle entry to its maintained contract
- **WHEN** repository verification checks the official built-in bundle
- **THEN** each bundle entry exposes the docs reference and verification profile needed to report bundle drift against maintained assets

### Requirement: Bundle entries declare structured readiness contracts
Each official built-in bundle entry SHALL declare structured readiness metadata that the control plane can evaluate without inferring behavior from free-form text alone. The readiness contract MUST declare the verification profile, maintained docs reference, prerequisite or configuration categories when applicable, and operator-facing next-step guidance needed to explain why a built-in is or is not ready.

#### Scenario: Bundle entry declares missing-configuration guidance
- **WHEN** an official built-in plugin requires secrets or persisted config before activation can succeed
- **THEN** its bundle entry declares that configuration requirement in structured readiness metadata
- **THEN** the readiness contract can be surfaced without hardcoding plugin-specific UI rules

#### Scenario: Bundle verification rejects incomplete readiness metadata
- **WHEN** an official built-in bundle entry omits required readiness contract fields for a non-ready built-in
- **THEN** repository verification fails before that entry is treated as a valid official built-in
- **THEN** the failure identifies which bundle entry and readiness field drifted

