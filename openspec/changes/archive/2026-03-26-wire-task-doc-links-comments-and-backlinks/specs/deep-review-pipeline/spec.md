## MODIFIED Requirements

### Requirement: Review write-back step
The deep review pipeline SHALL include a post-completion step that pushes review findings into the reviewed task's linked document pages.

#### Scenario: Write-back after review completion
- **WHEN** a deep review pipeline completes and the reviewed task has linked documents
- **THEN** the pipeline appends a "Review Findings" block to the first linked document (preferring link_type=requirement, then design) with the review summary and individual findings

#### Scenario: Write-back recorded in review log
- **WHEN** the write-back step executes
- **THEN** the review log records whether the write-back succeeded, which document was updated, and the version created
