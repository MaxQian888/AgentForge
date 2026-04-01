## ADDED Requirements

### Requirement: Operators can inspect plugin-role dependency health from the plugin panel
The plugin management panel SHALL surface role dependency health for any selected plugin whose current contract depends on roles or is consumed by role-scoped tool references. WorkflowPlugin records MUST show each referenced role binding, whether it currently resolves from the authoritative role registry, and whether any missing or stale role binding is blocking activation or execution. ToolPlugin or MCP-backed plugin records that are referenced by current roles through role-scoped tool or MCP identifiers MUST show the referencing roles and their current dependency state instead of hiding that relationship behind raw ids.

#### Scenario: Workflow plugin shows a stale role binding
- **WHEN** the operator opens the detail view for an installed workflow plugin whose manifest references a role id that no longer resolves from the current role registry
- **THEN** the panel shows that role binding as missing or stale instead of rendering only the raw role id list
- **THEN** the panel explains that activation or execution is currently blocked by that dependency gap and provides a path back to the roles workspace

#### Scenario: Tool plugin shows referencing roles
- **WHEN** the operator opens the detail view for a tool or MCP-backed plugin whose plugin id is referenced by one or more current roles through role-scoped tool configuration
- **THEN** the panel shows the referencing roles and the current dependency status for each reference
- **THEN** the operator can navigate from that plugin detail context to the affected roles without manually searching raw ids
