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
The system SHALL allow users to create custom templates within a project from an existing page or from scratch, edit their content and metadata later, duplicate them for variation, and delete them without affecting pages already created from those templates. Custom templates SHALL use the same block-editor content contract as normal wiki pages while remaining distinct from built-in templates.

#### Scenario: Create template from page
- **WHEN** the user clicks "Save as Template" on a wiki page
- **THEN** the system creates a custom template with the page's current content, a user-provided name, and a selected category

#### Scenario: Create template from scratch
- **WHEN** the user creates a new custom template from the template center
- **THEN** the system creates a blank template record and opens it in the document editor so the operator can author its content

#### Scenario: Edit custom template
- **WHEN** the user opens an existing custom template from the template center
- **THEN** the system allows the user to update the template content and metadata through the normal document editing surface

#### Scenario: Delete user template
- **WHEN** the user deletes a custom template
- **THEN** the system removes the template from the project template center
- **THEN** pages that were previously created from that template remain unchanged

### Requirement: Create page from template
The system SHALL allow users to create a new wiki page by selecting a template, previewing it, and providing the target page metadata before the copy is materialized. The created page SHALL be independent of the template thereafter.

#### Scenario: New page from template
- **WHEN** the user creates a new page from a selected template
- **THEN** the system prompts for the new page title and target location before creating the page
- **THEN** the system creates the page with a copy of the template's content blocks

#### Scenario: Created page remains independent of later template changes
- **WHEN** a page was previously created from a template and the source template is later edited or deleted
- **THEN** the created page keeps its own content and does not inherit those later template changes

### Requirement: Template listing API
The system SHALL expose a REST endpoint to list available document templates with enough metadata for discovery and guarded management. The listing contract SHALL support keyword, category, and source filters and SHALL return id, title, category, source or system flags, preview snippet, updated metadata, and actionability signals required by the template center.

#### Scenario: List templates via API
- **WHEN** the client sends a template-list request for the current project
- **THEN** the system returns built-in and custom templates for that project with discovery metadata suitable for the template center and picker

#### Scenario: Filter templates via API
- **WHEN** the client sends a template-list request with keyword, category, or source filters
- **THEN** the system returns only the templates that satisfy those filters

### Requirement: Template center supports discovery and preview
The template center SHALL provide search, category filtering, source filtering, and preview affordances so operators can inspect document templates before using or managing them.

#### Scenario: Search and filter templates in the template center
- **WHEN** the operator enters a keyword or changes category or source filters in the template center
- **THEN** the template list updates to the matching templates only
- **THEN** the UI keeps built-in and custom template sources distinguishable in the filtered results

#### Scenario: Preview a template before taking action
- **WHEN** the operator opens a template preview from the template center or template picker
- **THEN** the system shows the template title, category, source, preview content, and action availability before the operator instantiates or edits the template

### Requirement: System templates remain protected but reusable
Built-in document templates SHALL remain immutable baselines. Operators MUST be able to preview and instantiate them, and MAY duplicate them into custom templates, but MUST NOT edit or delete the built-in template record in place.

#### Scenario: Built-in template offers reuse without destructive management
- **WHEN** the operator opens a built-in document template in the management surface
- **THEN** the original template does not expose in-place edit or delete actions
- **THEN** the surface exposes use-template and duplicate-to-custom actions instead

