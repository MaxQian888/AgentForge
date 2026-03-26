# review-doc-writeback Specification

## Purpose
Define how completed reviews append findings into linked documents while preserving prior versions and handling concurrent edits safely.

## Requirements
### Requirement: Auto-append review findings to linked document
The system SHALL automatically append review findings to a task's linked document page when a review completes.

#### Scenario: Review findings appended to requirement doc
- **WHEN** a review completes for a task that has a linked document with link_type=requirement or link_type=design
- **THEN** the system appends a "Review Findings" section to the document containing the review summary, findings list, and a link back to the review

#### Scenario: No linked doc - write-back skipped
- **WHEN** a review completes for a task with no linked documents
- **THEN** the system skips the write-back step without error

#### Scenario: Write-back creates a new version
- **WHEN** the review write-back appends content to a document
- **THEN** the system creates a new named version "Review v{N} findings" before appending, preserving the pre-write-back state

### Requirement: Write-back conflict handling
The system SHALL handle concurrent edit conflicts during review write-back using optimistic locking.

#### Scenario: Conflict during write-back
- **WHEN** the document was edited by another user between the review start and write-back
- **THEN** the system retries with the latest content, appending findings at the end
