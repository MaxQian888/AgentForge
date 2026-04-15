## ADDED Requirements

### Requirement: Project bootstrap summary reflects lifecycle readiness across existing management surfaces
The system SHALL expose a project-scoped bootstrap summary for newly created or partially configured projects instead of treating project creation as complete after the initial record is saved. The summary MUST evaluate the current lifecycle using canonical project data and MUST cover at least governance readiness, team readiness, reusable playbook readiness, planning readiness, and delivery handoff readiness. Each phase MUST surface a visible status, an explanation or blocking reason, and the next recommended action when operator work is required.

#### Scenario: Newly created project shows bootstrap summary
- **WHEN** an operator creates a new project that has only its initial metadata and seeded baseline assets
- **THEN** the system shows a bootstrap summary for that project instead of silently treating the project as fully ready
- **AND** the summary identifies which lifecycle phases still need operator action

#### Scenario: Existing project remains partially configured
- **WHEN** an operator opens an existing project that still lacks required governance, team, planning, or delivery prerequisites
- **THEN** the bootstrap summary reflects the unresolved phases using current project state
- **AND** the system does not hide those gaps behind generic success messaging

### Requirement: Project bootstrap status stays truthful and non-destructive
The bootstrap lifecycle SHALL be derived from existing project state rather than a manually checked setup wizard, and it SHALL NOT auto-create fake tasks, teams, dashboards, or workflows just to mark phases complete. Built-in or seeded assets MAY count as baseline availability only when those assets actually exist for the project, and unresolved phases MUST remain visible until real operator actions or project state changes resolve them.

#### Scenario: Missing planning work remains visible
- **WHEN** a project has settings and templates available but still has no sprint, milestone, or task planning data
- **THEN** the planning phase remains unresolved in the bootstrap summary
- **AND** the system does not fabricate a placeholder backlog item to mark planning as complete

#### Scenario: Baseline assets are unavailable
- **WHEN** the expected seeded or built-in project assets for docs or workflows are unavailable for the current project context
- **THEN** the bootstrap summary marks the affected playbook phase as blocked or attention-required
- **AND** the summary explains which baseline asset is missing

### Requirement: Project creation and project entry preserve explicit bootstrap scope
The system SHALL preserve explicit project scope when an operator creates a project or re-enters one from project-entry surfaces. When explicit project context is present, downstream bootstrap or management destinations MUST use that context instead of falling back to ambient first-project selection or a no-project placeholder.

#### Scenario: Project creation hands off to the created project
- **WHEN** an operator creates a new project from the project list or equivalent entry surface
- **THEN** the system selects that created project as the active bootstrap scope
- **AND** the next rendered lifecycle guidance belongs to the new project rather than the previously selected project

#### Scenario: Explicit project handoff overrides stale ambient selection
- **WHEN** an operator follows a bootstrap or project-entry action that includes explicit context for project B while project A is still the ambient selection
- **THEN** the destination opens in project B scope
- **AND** the system SHALL NOT continue showing project A content as if the handoff were ignored

### Requirement: Bootstrap handoff actions preserve destination intent
The system SHALL provide project-scoped handoff actions from the bootstrap summary into the existing management surfaces, including settings, team management, docs and templates, workflow management, planning surfaces, and dashboard or execution visibility. Each handoff MUST preserve project scope and MAY include a destination-specific focus intent such as a section, tab, filter, or create action. Destinations that receive an explicit handoff intent MUST consume it directly instead of rendering only a generic default landing state.

#### Scenario: Operator follows bootstrap action into a setup surface
- **WHEN** an operator selects a bootstrap action such as configuring governance, adding the first member, opening workflow templates, or creating the first sprint
- **THEN** the destination workspace opens with the same project scope preserved
- **AND** the destination uses the provided focus intent to highlight or open the relevant section instead of leaving the operator at an unrelated default tab

#### Scenario: Destination would otherwise show no-project placeholder
- **WHEN** a bootstrap handoff targets a project-scoped destination that normally falls back to a no-project state without explicit selection
- **THEN** the destination consumes the provided project context directly
- **AND** the operator does not see the generic no-project placeholder for that handoff
