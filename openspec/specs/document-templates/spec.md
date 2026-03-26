# document-templates Specification

## Purpose
Define built-in and user-authored wiki templates so projects can start common document types from reusable structured content.

## Requirements
### Requirement: Built-in templates
The system SHALL provide a set of built-in document templates seeded on project creation: PRD, RFC, ADR, Postmortem, Onboarding, Runbook, and Agent Task Brief.

#### Scenario: Built-in templates available on new project
- **WHEN** a new project is created
- **THEN** the wiki space contains built-in templates marked as `system = true` in categories: prd, rfc, adr, postmortem, onboarding, runbook, agent-task-brief

#### Scenario: Built-in template content
- **WHEN** user views a built-in template
- **THEN** the template contains structured sections appropriate to its type (e.g., ADR has Status, Context, Decision, Consequences sections)

### Requirement: User-created templates
The system SHALL allow users to create, edit, and delete custom templates within a project.

#### Scenario: Create template from page
- **WHEN** user clicks "Save as Template" on a wiki page
- **THEN** the system creates a template with the page's current content, a user-provided name, and a selected category

#### Scenario: Create template from scratch
- **WHEN** user creates a new template via the template center
- **THEN** the system opens the block editor with an empty template that the user can populate

#### Scenario: Delete user template
- **WHEN** user deletes a custom template
- **THEN** the system removes the template; existing pages created from it are not affected

### Requirement: Create page from template
The system SHALL allow users to create a new wiki page by selecting a template.

#### Scenario: New page from template
- **WHEN** user creates a new page and selects a template
- **THEN** the system creates the page with a copy of the template's content blocks, and the page is independent of the template thereafter

### Requirement: Template listing API
The system SHALL expose a REST endpoint to list available templates.

#### Scenario: List templates via API
- **WHEN** client sends `GET /api/v1/projects/:pid/wiki/templates`
- **THEN** the system returns all templates (built-in and user-created) with id, name, category, is_system, and preview snippet
