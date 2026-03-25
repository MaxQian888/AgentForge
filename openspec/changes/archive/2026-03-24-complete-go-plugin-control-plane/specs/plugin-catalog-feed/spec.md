## ADDED Requirements

### Requirement: Catalog entries are sourced from real plugin manifests
The system SHALL serve plugin catalog data from real built-in and installable plugin manifests the Go server can resolve, instead of returning hardcoded placeholder marketplace entries. Each catalog entry MUST include plugin identifier, name, version, kind, description, source metadata, and an install action target that the Go control plane can actually use.

#### Scenario: Built-in plugin appears in the catalog
- **WHEN** the Go server scans a valid built-in plugin manifest under the configured plugins directory
- **THEN** the catalog feed includes that plugin with built-in source metadata and an install target derived from the real manifest path

#### Scenario: Placeholder entries are not returned
- **WHEN** the catalog endpoint is requested
- **THEN** it does not return synthetic plugin entries that are unrelated to resolvable manifests on disk or configured install sources

### Requirement: Catalog status reflects registry ownership
The system SHALL merge catalog entries with the persistent plugin registry so operators can distinguish installable plugins from already-installed plugins using one server-backed response.

#### Scenario: Installed built-in plugin is marked from registry state
- **WHEN** a built-in plugin is already present in the plugin registry
- **THEN** the catalog response identifies that entry as already installed using the persisted registry record instead of duplicating it as an unrelated offering

#### Scenario: Missing or invalid manifest is excluded from the catalog
- **WHEN** a configured catalog source points to a missing or invalid manifest
- **THEN** the Go server excludes that entry from the operator-facing catalog response instead of returning incomplete placeholder data
