# custom-field-system Specification

## Purpose
Define project-scoped custom field definitions, task-level field values, and field-aware filtering, sorting, and grouping in task views.

## Requirements
### Requirement: Custom field definition CRUD
The system SHALL allow project admins to create, update, reorder, and delete custom field definitions for a project. Supported field types SHALL be: text, number, select, multi_select, date, user, url, checkbox.

#### Scenario: Create a select field
- **WHEN** admin creates a custom field with name "Priority", type "select", and options ["P0","P1","P2","P3"]
- **THEN** the system creates the field definition and it becomes available on all tasks in the project

#### Scenario: Update field options
- **WHEN** admin adds a new option "P4" to an existing select field
- **THEN** the field definition is updated and the new option is available for selection on tasks

#### Scenario: Delete a custom field
- **WHEN** admin deletes a custom field definition
- **THEN** the field and all its values on tasks are soft-deleted and the field disappears from all views

#### Scenario: Reorder fields
- **WHEN** admin reorders custom fields via drag-and-drop in project settings
- **THEN** the sort_order is updated and fields appear in the new order across all views

### Requirement: Custom field values on tasks
The system SHALL store and display custom field values on each task.

#### Scenario: Set field value on task
- **WHEN** user sets a custom field value (e.g., Priority = "P1") on a task
- **THEN** the system persists the value and displays it in the task detail and applicable view columns

#### Scenario: Clear field value
- **WHEN** user clears a custom field value on a task
- **THEN** the value is removed and the field shows as empty

#### Scenario: Required field validation
- **WHEN** a custom field is marked as required and a user tries to save a task without setting it
- **THEN** the system shows a validation error and prevents the save

### Requirement: Custom field API
The system SHALL expose REST endpoints for custom field operations.

#### Scenario: List field definitions via API
- **WHEN** client sends `GET /api/v1/projects/:pid/fields`
- **THEN** the system returns all custom field definitions ordered by sort_order

#### Scenario: Set field value via API
- **WHEN** client sends `PUT /api/v1/projects/:pid/tasks/:tid/fields/:fid` with a value
- **THEN** the system persists the value and returns 200

### Requirement: Custom field filtering and sorting
The system SHALL support filtering, sorting, and grouping tasks by custom field values.

#### Scenario: Filter tasks by select field
- **WHEN** user applies a filter "Priority = P0" in a view
- **THEN** only tasks with custom field Priority set to "P0" are displayed

#### Scenario: Group tasks by custom field
- **WHEN** user groups the board view by a select custom field
- **THEN** tasks are grouped into columns/sections by the field's values
