## MODIFIED Requirements

### Requirement: Custom field columns in views
The task multi-view board SHALL render custom field values as columns in list and table views, and support filtering, sorting, and grouping by custom fields.

#### Scenario: Custom field column in table view
- **WHEN** a project has a custom field "Risk Level" and user enables it as a column in table view
- **THEN** each task row displays the Risk Level value, editable inline

#### Scenario: Filter by custom field in board view
- **WHEN** user applies a filter "Risk Level = High" in any view
- **THEN** only tasks with Risk Level set to "High" are displayed

#### Scenario: Group by custom field
- **WHEN** user groups the board view by "Module" custom field
- **THEN** tasks are grouped into columns by their Module value, including an "Unset" column for tasks without a value

### Requirement: Saved view integration
The task multi-view board SHALL load and apply saved view configurations when a view is selected from the view switcher.

#### Scenario: Apply saved view
- **WHEN** user selects a saved view from the view switcher
- **THEN** the workspace applies the view's layout type, filters, sorts, groups, and column configuration
