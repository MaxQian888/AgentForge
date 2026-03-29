# plugin-catalog-feed Specification

## Purpose
Define the operator-facing plugin catalog feed so AgentForge serves real manifest-backed plugin offerings and installed-state visibility from the Go control plane.
## Requirements
### Requirement: Catalog entries are sourced from real plugin manifests
The system SHALL serve plugin catalog data from real built-in and installable plugin manifests the Go server can resolve, instead of returning hardcoded placeholder marketplace entries. Entries shown in the built-in availability section MUST correspond to manifest-backed assets that are declared in the repository's official built-in plugin bundle. Each catalog entry MUST include plugin identifier, name, version, kind, description, source metadata, an install action target that the Go control plane can actually use, and truthful availability metadata when the entry belongs to the official built-in bundle.

#### Scenario: Official built-in plugin appears in the catalog
- **WHEN** the Go server resolves a built-in bundle entry whose manifest path exists and parses successfully
- **THEN** the catalog feed includes that plugin with built-in source metadata, official built-in provenance, and an install target derived from the real manifest path

#### Scenario: Broken or unlisted built-in asset is not exposed as an official plugin
- **WHEN** a manifest is missing, invalid, or not declared in the repository's built-in plugin bundle
- **THEN** the control plane does not expose that asset as an official built-in catalog entry
- **THEN** it does not synthesize a placeholder install target for the missing or unlisted asset

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

### Requirement: Official built-in catalog entries expose readiness separately from installability
The system SHALL return evaluated readiness state and setup guidance for official built-in catalog and discovery entries separately from the source installability contract. A built-in entry MAY remain explicitly installable while still reporting blocked activation readiness, but the response MUST NOT imply that installability alone means the plugin is immediately runnable.

#### Scenario: Built-in entry is installable but still requires configuration
- **WHEN** an official built-in plugin can be installed through the supported built-in flow but lacks required configuration for activation
- **THEN** the catalog response keeps the explicit install path available
- **THEN** the same response marks the built-in as not ready for activation and includes setup guidance describing the missing configuration

#### Scenario: Built-in entry is blocked on the current host
- **WHEN** an official built-in plugin is unsupported on the current host or missing a required local prerequisite
- **THEN** the catalog or discovery response includes the built-in with evaluated readiness and blocking guidance
- **THEN** the response does not misrepresent that built-in as fully runnable on the current host

