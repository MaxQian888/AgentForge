# plugin-catalog-feed Specification

## Purpose
Define the operator-facing plugin catalog feed so AgentForge serves real manifest-backed plugin offerings and installed-state visibility from the Go control plane.
## Requirements
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

### Requirement: Catalog discovery is read-only until installation is explicit
The system SHALL treat built-in discovery and catalog search as read-only browse operations. Serving discovery or catalog results MUST NOT create, update, enable, or otherwise implicitly promote installed plugin registry records. Only explicit install actions, such as catalog install or local install flows, may create or update installed plugin ownership in the registry.

#### Scenario: Browsing built-in discovery leaves the registry unchanged
- **WHEN** the operator loads built-in or catalog availability entries without choosing an install action
- **THEN** the control plane returns browse data for those entries without creating installed plugin records
- **THEN** a plugin only appears in the installed registry section if it was already installed before the browse request

#### Scenario: Catalog search returns installable metadata without side effects
- **WHEN** the operator searches the plugin catalog for matching entries
- **THEN** the control plane returns installable catalog metadata and installed-state visibility for those entries
- **THEN** the search request itself does not mutate registry ownership, lifecycle state, or plugin instance data

#### Scenario: Explicit catalog install promotes an entry into the registry
- **WHEN** the operator chooses a catalog or built-in entry to install through an explicit install action
- **THEN** the control plane creates or updates the installed plugin record using the selected source metadata
- **THEN** installed-state visibility changes only after that install action completes successfully
