# review-doc-writeback Specification

## Purpose
Define how completed reviews append findings into linked documents while preserving prior versions and handling concurrent edits safely.

## Requirements
### Requirement: Auto-append review findings to linked document

The system SHALL automatically append review findings to a task's linked `kind=wiki_page` knowledge asset when a review completes. Write-back SHALL only target `wiki_page` assets; ingested-file and template kinds are excluded.

#### Scenario: Review findings appended to requirement wiki page

- **WHEN** a review completes for a task that has a linked `wiki_page` asset with `link_type=requirement` or `link_type=design`
- **THEN** the system appends a "Review Findings" section to the asset's `content_json` containing the review summary, findings list, and a link back to the review

#### Scenario: No linked wiki page — write-back skipped

- **WHEN** a review completes for a task with no linked `wiki_page` assets
- **THEN** the system skips the write-back step without error, even if the task has linked `ingested_file` or other-kind assets

#### Scenario: Write-back creates a new version

- **WHEN** the review write-back appends content to a `wiki_page` asset
- **THEN** the system creates a new named `AssetVersion` "Review v{N} findings" with `kind_snapshot=wiki_page` before appending, preserving the pre-write-back state

### Requirement: Write-back conflict handling

The system SHALL handle concurrent edit conflicts during review write-back using the asset's optimistic-lock `version`.

#### Scenario: Conflict during write-back

- **WHEN** the `wiki_page` asset was edited by another user between the review start and write-back, making the asset's current `version` newer than the version the write-back started from
- **THEN** the system retries with the latest content, appending findings at the end
