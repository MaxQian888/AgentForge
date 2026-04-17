## MODIFIED Requirements

### Requirement: Workflow template library resolves templates in current project context
The system SHALL provide a workflow template library that resolves the active project context from an explicit, validated project-scoped request contract and returns built-in, marketplace, and current-project custom workflow templates without exposing custom templates owned by other projects. Requests that do not provide a valid project context for project-owned template discovery MUST fail explicitly instead of falling back to ambient selection, implicit header defaults, or zero-value project scope.

#### Scenario: Project-aware template library merges global and project-owned templates
- **WHEN** the operator opens the workflow template library for a project
- **THEN** the library includes built-in templates, marketplace templates, and custom templates owned by that project
- **THEN** the library excludes custom templates owned by any other project

#### Scenario: Workflow template library supports discovery filters
- **WHEN** the operator searches or filters the workflow template library by keyword, category, or source
- **THEN** the library returns only templates that match those filters while preserving source visibility

#### Scenario: Workflow template request omits project context
- **WHEN** a client requests project-aware workflow templates without a valid explicit project context
- **THEN** the system rejects the request with a client-visible error
- **THEN** the library does not silently return a zero-project or globally over-broad result set

### Requirement: Operators can publish saved workflows as custom templates
The system SHALL allow operators to publish a saved workflow definition as a project-owned custom template without mutating the original workflow definition in place. The published template SHALL retain lineage to the source workflow definition so future management surfaces can explain where the template came from, and the publish operation MUST verify that the source workflow definition belongs to the explicit current project context before creating the template.

#### Scenario: Publish workflow definition as reusable template
- **WHEN** the operator publishes a saved workflow definition as a template
- **THEN** the system creates a separate template record owned by the current project
- **THEN** the original workflow definition remains available as its original non-template workflow record

#### Scenario: Publish request targets another project's workflow
- **WHEN** a client attempts to publish a workflow definition that belongs to a different project than the explicit request scope
- **THEN** the system rejects the publish request as an ownership mismatch
- **THEN** no template record is created for that mismatched workflow definition

### Requirement: Workflow templates can be cloned or executed with scoped overrides
The system SHALL allow operators to clone or execute a workflow template within the current project using template-variable overrides. The resulting workflow definition or execution SHALL belong to the explicit current project and SHALL remain independent from the source template afterward. The system MUST reject clone or execute requests when the explicit project context is missing or when the requested template source is not visible to that project.

#### Scenario: Clone template with overrides
- **WHEN** the operator clones a workflow template with variable overrides for the current project
- **THEN** the system creates a new workflow definition in that project using the merged template variables
- **THEN** later edits to the source template do not retroactively change the cloned workflow definition

#### Scenario: Execute template with overrides
- **WHEN** the operator executes a workflow template with variable overrides for the current project
- **THEN** the system starts execution from a project-scoped cloned definition using those merged overrides
- **THEN** the resulting execution record belongs to the current project context

#### Scenario: Template source is not visible to current project
- **WHEN** a client attempts to clone or execute a template that is neither global nor owned by the explicitly requested project
- **THEN** the system rejects the request instead of creating a workflow definition or execution in the current project
- **THEN** the request does not leak custom template behavior across project boundaries