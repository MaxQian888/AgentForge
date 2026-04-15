# workflow-template-library Specification

## Purpose
Define the project-aware workflow template library so operators can discover, publish, manage, clone, and execute built-in, marketplace, and current-project workflow templates without cross-project leakage or mutating immutable shipped templates in place.
## Requirements
### Requirement: Workflow template library resolves templates in current project context
The system SHALL provide a workflow template library that resolves the active project context and returns built-in, marketplace, and current-project custom workflow templates without exposing custom templates owned by other projects.

#### Scenario: Project-aware template library merges global and project-owned templates
- **WHEN** the operator opens the workflow template library for a project
- **THEN** the library includes built-in templates, marketplace templates, and custom templates owned by that project
- **THEN** the library excludes custom templates owned by any other project

#### Scenario: Workflow template library supports discovery filters
- **WHEN** the operator searches or filters the workflow template library by keyword, category, or source
- **THEN** the library returns only templates that match those filters while preserving source visibility

### Requirement: Workflow template preview exposes reuse and compatibility context
The workflow template library SHALL let operators inspect a template before using or managing it. Preview MUST include name, description, source, category, topology summary, required template variables, and any immutable or compatibility cues relevant to the current source.

#### Scenario: Preview built-in or marketplace template before use
- **WHEN** the operator opens preview for a built-in or marketplace workflow template
- **THEN** the system shows the template metadata, required variables, and immutable-source cues before the operator clones or executes it

#### Scenario: Preview custom project template before management
- **WHEN** the operator opens preview for a custom workflow template owned by the current project
- **THEN** the system shows the same template metadata together with management actions allowed for that custom template

### Requirement: Operators can publish saved workflows as custom templates
The system SHALL allow operators to publish a saved workflow definition as a project-owned custom template without mutating the original workflow definition in place. The published template SHALL retain lineage to the source workflow definition so future management surfaces can explain where the template came from.

#### Scenario: Publish workflow definition as reusable template
- **WHEN** the operator publishes a saved workflow definition as a template
- **THEN** the system creates a separate template record owned by the current project
- **THEN** the original workflow definition remains available as its original non-template workflow record

### Requirement: Operators can manage project-owned workflow templates
The system SHALL allow operators to edit and delete workflow templates owned by the current project. Built-in and marketplace templates MUST remain immutable in place and SHALL require clone or publish-a-copy flows for customization.

#### Scenario: Edit or delete custom workflow template
- **WHEN** the operator edits or deletes a workflow template owned by the current project
- **THEN** the system applies that change to the custom template record only
- **THEN** previously cloned workflows or prior executions remain unchanged

#### Scenario: Immutable shipped template cannot be edited in place
- **WHEN** the operator attempts to manage a built-in or marketplace workflow template as if it were custom
- **THEN** the system blocks in-place edit and delete actions
- **THEN** the system offers clone or copy-based customization actions instead

### Requirement: Workflow templates can be cloned or executed with scoped overrides
The system SHALL allow operators to clone or execute a workflow template within the current project using template-variable overrides. The resulting workflow definition or execution SHALL belong to the current project and SHALL remain independent from the source template afterward.

#### Scenario: Clone template with overrides
- **WHEN** the operator clones a workflow template with variable overrides for the current project
- **THEN** the system creates a new workflow definition in that project using the merged template variables
- **THEN** later edits to the source template do not retroactively change the cloned workflow definition

#### Scenario: Execute template with overrides
- **WHEN** the operator executes a workflow template with variable overrides for the current project
- **THEN** the system starts execution from a project-scoped cloned definition using those merged overrides
- **THEN** the resulting execution record belongs to the current project context
