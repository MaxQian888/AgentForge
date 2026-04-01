## ADDED Requirements

### Requirement: Role surfaces expose plugin dependency health and downstream plugin consumers
The role library, role workspace, and role context surfaces SHALL expose both sides of the current plugin-role contract. For the currently selected or previewed role, the UI MUST show whether each role-scoped external tool or MCP server dependency currently resolves to a usable plugin in the current checkout, and it MUST also show which installed plugins currently consume that role through declared role bindings so operators can understand impact before preview, save, or delete actions.

#### Scenario: Role workspace shows a missing plugin dependency
- **WHEN** the operator edits or previews a role whose `toolConfig.external` or `toolConfig.mcpServers` contains a plugin-scoped dependency that is not currently usable
- **THEN** the workspace and review context show that dependency as an explicit readiness cue instead of forcing the operator to infer it from raw YAML
- **THEN** the operator can see which plugin or MCP dependency is missing from the same authoring flow

#### Scenario: Role surfaces show downstream workflow consumers before delete
- **WHEN** the operator selects a role that is currently referenced by one or more installed plugins with declared role bindings
- **THEN** the role library or workspace shows a downstream consumer summary before the operator confirms deletion
- **THEN** the delete affordance explains that dependent plugins must be updated or removed first rather than failing later with a generic runtime error

### Requirement: Role workspace can select installed plugins and inspect their declared functions
The role workspace SHALL let operators specify installed tool plugins for an existing role without relying on manual string entry alone. When installed ToolPlugin records are available, the capabilities authoring surface MUST present them as selectable entries, MUST write the chosen plugin identifiers back into the role's external tool configuration, and MUST expose each plugin's runtime or lifecycle summary plus declared functions or capabilities so operators can understand what they are attaching to the role.

#### Scenario: Operator adds an installed plugin to an existing role
- **WHEN** the operator edits an existing role and the current checkout contains installed ToolPlugin records
- **THEN** the capabilities section shows those plugins as selectable entries with explicit add or use actions
- **THEN** choosing one of those entries updates the role draft's plugin configuration without requiring the operator to manually type the plugin id

#### Scenario: Role workspace shows declared plugin functions before selection
- **WHEN** the capabilities section lists available installed plugins for role authoring
- **THEN** each listed plugin shows its runtime or lifecycle summary and declared functions or capabilities
- **THEN** the operator can compare plugin choices from the same authoring surface before attaching them to the role
