# saved-views Specification

## Purpose
Define reusable task workspace view configurations, including sharing, defaults, and view switching for project members.

## Requirements
### Requirement: Create and save views
The system SHALL allow users to save the current view configuration (layout, filters, sorts, groups, columns) as a named view.

#### Scenario: Save personal view
- **WHEN** user configures filters/sorts/columns and clicks "Save View" with a name
- **THEN** the system creates a personal view visible only to that user

#### Scenario: Save shared view
- **WHEN** user saves a view and marks it as "shared"
- **THEN** the view becomes visible to all project members

#### Scenario: Set default view
- **WHEN** project admin marks a shared view as the project default
- **THEN** all project members see this view by default when opening the task workspace

### Requirement: View switching
The system SHALL display a view switcher in the task workspace header.

#### Scenario: Switch between views
- **WHEN** user clicks on a different view in the view switcher
- **THEN** the task workspace applies the selected view's layout, filters, sorts, groups, and columns

### Requirement: View sharing to specific members
The system SHALL allow sharing a view with specific members or roles.

#### Scenario: Share view with a role
- **WHEN** user shares a view with the "reviewer" role
- **THEN** only members with the reviewer role can see and use the view

### Requirement: View API
The system SHALL expose REST endpoints for saved view operations.

#### Scenario: List views via API
- **WHEN** client sends `GET /api/v1/projects/:pid/views`
- **THEN** the system returns all views accessible to the current user (personal + shared + role-matched)

#### Scenario: Create view via API
- **WHEN** client sends `POST /api/v1/projects/:pid/views` with name, config, and visibility settings
- **THEN** the system creates the view and returns 201

### Requirement: View update and delete
The system SHALL allow view owners to update or delete their views.

#### Scenario: Update view config
- **WHEN** view owner modifies filters and clicks "Update View"
- **THEN** the saved view config is updated for all who can access it

#### Scenario: Delete personal view
- **WHEN** user deletes a personal view
- **THEN** the view is removed and no longer appears in the view switcher
