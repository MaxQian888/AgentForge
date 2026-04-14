## MODIFIED Requirements

### Requirement: The role library summarizes execution-relevant properties for selection
The system SHALL present the role library as a distinguishable catalog rather than a name-only list. Each role entry MUST expose the metadata and governance cues needed to compare roles quickly, including version, tags, inheritance markers when present, and visible execution or safety signals when configured.

The catalog panel header SHALL display the section title and description on the first line and the action buttons (Marketplace and New Role) on a second line below, so that both lines are fully legible within the 260px panel width without truncation or overflow.

#### Scenario: Review role differences from the list view
- **WHEN** the operator scans the role library
- **THEN** each role entry shows enough summary information to distinguish role purpose, version, and inheritance state without opening the editor first

#### Scenario: Role uses review or path restrictions
- **WHEN** a role requires review or defines allowed or denied paths
- **THEN** the role library surfaces those constraints as visible summary cues instead of hiding them only inside the edit workspace

#### Scenario: Catalog header is fully legible at 260px width
- **WHEN** the catalog panel renders at its default width of 260px
- **THEN** the title "Role Library" and its description text are fully visible on their own line
- **AND** the Marketplace and New Role buttons appear on a separate line below the title
- **AND** no button text is truncated or overlaps the title text
