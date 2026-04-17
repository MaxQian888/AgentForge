## MODIFIED Requirements

### Requirement: Project bootstrap summary reflects lifecycle readiness across existing management surfaces
The system SHALL expose a project-scoped bootstrap summary for newly created or partially configured projects instead of treating project creation as complete after the initial record is saved. The summary MUST evaluate the current lifecycle using canonical project data from the same explicit project-scoped contracts consumed by the downstream management workspaces, and MUST cover at least governance readiness, team readiness, reusable playbook readiness, planning readiness, and delivery handoff readiness. Each phase MUST surface a visible status, an explanation or blocking reason, and the next recommended action when operator work is required.

#### Scenario: Newly created project shows bootstrap summary
- **WHEN** an operator creates a new project that has only its initial metadata and seeded baseline assets
- **THEN** the system shows a bootstrap summary for that project instead of silently treating the project as fully ready
- **AND** the summary identifies which lifecycle phases still need operator action

#### Scenario: Existing project remains partially configured
- **WHEN** an operator opens an existing project that still lacks required governance, team, planning, or delivery prerequisites
- **THEN** the bootstrap summary reflects the unresolved phases using current project state
- **AND** the system does not hide those gaps behind generic success messaging

#### Scenario: Playbook readiness uses current project template truth
- **WHEN** the bootstrap summary evaluates reusable playbook readiness for a specific project
- **THEN** the summary uses the same project-scoped docs template and workflow template contracts used by the destination workspaces
- **AND** the playbook phase does not report ready based on unscoped template data from another project or a global fallback

#### Scenario: Project entry summary and bootstrap use the same project truth
- **WHEN** an operator opens a project from the project list or another project-entry surface and then lands on bootstrap guidance
- **THEN** the bootstrap summary is derived from the same authoritative project summary contract used by the entry surface
- **AND** project card defaults or missing DTO fields do not cause bootstrap to display a false-ready state