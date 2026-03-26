## MODIFIED Requirements

### Requirement: Linked Docs column in board views
The task multi-view board SHALL support a "Linked Docs" optional column in list and table views displaying linked document titles.

#### Scenario: Show Linked Docs column in table view
- **WHEN** user enables the "Linked Docs" column in table view settings
- **THEN** each task row displays the titles of linked documents as clickable chips

### Requirement: Doc preview popover on task cards
Task cards in all board views SHALL display a document preview popover when hovered over a linked-doc indicator.

#### Scenario: Hover doc indicator on task card
- **WHEN** user hovers over the document icon on a task card that has linked docs
- **THEN** a popover shows the first linked document's title and first 3 lines of content, with a "View" link
