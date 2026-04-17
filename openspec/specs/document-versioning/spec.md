# document-versioning Specification

## Purpose
Define named wiki page snapshots, version history browsing, restore flows, and read-only version sharing.

## Requirements
### Requirement: Create named version

The system SHALL allow users to save a named version snapshot of a knowledge asset's current content. Named versions SHALL be supported for `kind=wiki_page` and `kind=template`. For `kind=ingested_file`, versions SHALL be created automatically by reuploads rather than by user-named snapshots.

#### Scenario: Save named version on wiki page

- **WHEN** user clicks "Save Version" on a `wiki_page` asset and provides a version name
- **THEN** the system creates an `AssetVersion` record with the current `content_json`, `kind_snapshot=wiki_page`, version name, creator, and timestamp

#### Scenario: Auto-increment version number

- **WHEN** a named version is created
- **THEN** the system assigns a monotonically increasing `version_number` (v1, v2, v3...) scoped to the asset, in addition to the user-provided name

#### Scenario: Reject manual version on ingested file

- **WHEN** user attempts to call the "Save Version" API against a `kind=ingested_file` asset
- **THEN** the system rejects the request with a validation error directing the user to re-upload instead

### Requirement: Version history browsing

The system SHALL display a version history list for each asset that supports versioning.

#### Scenario: View version history

- **WHEN** user opens the version history panel for an asset
- **THEN** the system shows a chronological list of versions with `version_number`, name, `kind_snapshot`, creator, and timestamp

#### Scenario: View version content

- **WHEN** user clicks on a specific version in the history
- **THEN** the system renders the version's content in a read-only viewer appropriate to the `kind_snapshot` (BlockNote viewer for wiki/template, file download for ingested file)

### Requirement: Version restore

The system SHALL allow users to restore a `wiki_page` or `template` asset to a previous version's content. Restore is NOT supported for `ingested_file` kinds.

#### Scenario: Restore a wiki-page version

- **WHEN** user clicks "Restore" on a version of a `wiki_page` asset
- **THEN** the system replaces the asset's current `content_json` with the version's content and auto-saves, creating a new version entry named "Restored from v{N}"

#### Scenario: Reject restore on ingested file

- **WHEN** user attempts to restore a version on a `kind=ingested_file` asset
- **THEN** the system rejects the request with a validation error

### Requirement: Read-only version link

The system SHALL support generating shareable read-only links to specific versions.

#### Scenario: Share version link

- **WHEN** user clicks "Share" on a version
- **THEN** the system generates a URL that renders the version content in a read-only view accessible to project members

### Requirement: Version API

The system SHALL expose REST endpoints for version operations scoped by asset id.

#### Scenario: List versions via API

- **WHEN** client sends `GET /api/v1/projects/:pid/knowledge/assets/:id/versions`
- **THEN** the system returns all versions for the asset, ordered by `version_number` descending

#### Scenario: Create named version via API

- **WHEN** client sends `POST /api/v1/projects/:pid/knowledge/assets/:id/versions` with a `name` against a `wiki_page` or `template` asset
- **THEN** the system snapshots the current `content_json` and returns the created version with 201 status

#### Scenario: Restore version via API

- **WHEN** client sends `POST /api/v1/projects/:pid/knowledge/assets/:id/versions/:vid/restore` against a `wiki_page` or `template` asset
- **THEN** the system restores the asset content from the specified version

### Requirement: Reupload creates a version for ingested files

The system SHALL automatically snapshot an `ingested_file` asset into an `AssetVersion` when the asset is reuploaded. The snapshot SHALL capture the prior `file_ref`, `file_size`, `mime_type`, and `content_text`.

#### Scenario: Reupload snapshots prior state

- **WHEN** a reupload is processed for an `ingested_file` asset
- **THEN** the system creates an `AssetVersion` with `kind_snapshot=ingested_file` capturing the prior binary metadata and parsed text, before replacing the asset's binary
