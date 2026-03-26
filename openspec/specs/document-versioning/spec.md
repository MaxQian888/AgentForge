# document-versioning Specification

## Purpose
Define named wiki page snapshots, version history browsing, restore flows, and read-only version sharing.

## Requirements
### Requirement: Create named version
The system SHALL allow users to save a named version snapshot of a wiki page's current content.

#### Scenario: Save named version
- **WHEN** user clicks "Save Version" and provides a version name
- **THEN** the system creates a version record with the current content snapshot, version name, creator, and timestamp

#### Scenario: Auto-increment version number
- **WHEN** a named version is created
- **THEN** the system assigns a monotonically increasing version number (v1, v2, v3...) in addition to the user-provided name

### Requirement: Version history browsing
The system SHALL display a version history list for each wiki page.

#### Scenario: View version history
- **WHEN** user opens the version history panel for a page
- **THEN** the system shows a chronological list of named versions with version number, name, creator, and timestamp

#### Scenario: View version content
- **WHEN** user clicks on a specific version in the history
- **THEN** the system renders the version's content in a read-only viewer

### Requirement: Version restore
The system SHALL allow users to restore a page to a previous version's content.

#### Scenario: Restore a version
- **WHEN** user clicks "Restore" on a specific version
- **THEN** the system replaces the page's current content with the version's content and auto-saves, creating a new version entry named "Restored from v{N}"

### Requirement: Read-only version link
The system SHALL support generating shareable read-only links to specific versions.

#### Scenario: Share version link
- **WHEN** user clicks "Share" on a version
- **THEN** the system generates a URL that renders the version content in a read-only view accessible to project members

### Requirement: Version API
The system SHALL expose REST endpoints for version operations.

#### Scenario: List versions via API
- **WHEN** client sends `GET /api/v1/projects/:pid/wiki/pages/:id/versions`
- **THEN** the system returns all versions for the page, ordered by version number descending

#### Scenario: Create version via API
- **WHEN** client sends `POST /api/v1/projects/:pid/wiki/pages/:id/versions` with a name
- **THEN** the system snapshots the current content and returns the created version with 201 status

#### Scenario: Restore version via API
- **WHEN** client sends `POST /api/v1/projects/:pid/wiki/pages/:id/versions/:vid/restore`
- **THEN** the system restores the page content from the specified version
