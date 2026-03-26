## ADDED Requirements

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
