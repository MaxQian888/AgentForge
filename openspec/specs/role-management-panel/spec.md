# role-management-panel Specification

## Purpose
Define the structured role management experience for creating, editing, templating, and reviewing execution-relevant role manifest settings.
## Requirements
### Requirement: Operators can edit roles through a structured manifest-aware form

The role workspace SHALL provide editing for ALL documented role manifest fields without requiring raw file surgery. This includes security permissions (fileAccess with allowedPaths and deniedPaths, network with allowedDomains, codeExecution with sandbox mode and allowedLanguages) and resource limits (tokenBudget, apiCalls, executionTime, costLimit). These fields SHALL appear in the Governance section as collapsible sub-sections.

#### Scenario: Operator edits file access permissions
- **WHEN** operator expands the Permissions sub-section in Governance
- **THEN** the editor displays fileAccess fields with allowed paths and denied paths as editable lists
- **AND** network fields with allowed domains as an editable list
- **AND** codeExecution fields with sandbox toggle and allowed languages list

#### Scenario: Operator edits resource limits
- **WHEN** operator expands the Resource Limits sub-section in Governance
- **THEN** the editor displays tokenBudget, apiCalls, executionTime, and costLimit as numeric inputs
- **AND** each field shows its current value or empty for unset

#### Scenario: Saving preserves advanced governance fields
- **WHEN** operator saves a role after editing permissions and resource limits
- **THEN** the serialized manifest includes the updated permissions and resourceLimits
- **AND** untouched fields from the original manifest are preserved

#### Scenario: Governance sub-sections default to collapsed
- **WHEN** operator opens a role that has no permissions or resource limits set
- **THEN** the Permissions and Resource Limits sub-sections are collapsed by default
- **AND** expanding them shows empty/default fields ready for input

### Requirement: The role panel supports template-based creation and inheritance setup
The system SHALL support role creation flows that start from a blank role, copy an existing role as a template, or inherit from an existing role. The workspace SHALL present these starting decisions as a visible first-stage setup flow and SHALL keep the selected template source, inheritance parent, live execution-oriented summary, preview action, and sandbox action discoverable throughout authoring so operators can follow the recommended flow from `docs/role-authoring-guide.md` without hunting through the page.

#### Scenario: Start from an existing role template
- **WHEN** the operator chooses to create a role from an existing role template
- **THEN** the workspace prefills the new draft with the template's reusable values
- **THEN** the UI makes the template source visible in the authoring flow so the operator understands what was reused

#### Scenario: Create a child role that extends an existing role
- **WHEN** the operator chooses an existing role as a parent for inheritance
- **THEN** the draft stores that parent in the role's `extends` field
- **THEN** the workspace clearly indicates that the draft is a child role and keeps that inheritance state visible while the operator edits later sections

#### Scenario: Inspect effective summary before saving
- **WHEN** the operator edits prompt, packages, tool host config, budget, permission, collaboration, or trigger fields in the role draft
- **THEN** the workspace shows a live summary of the draft's execution-relevant settings, including prompt intent, tool limits, budget or turn caps, permission mode, governance cues, and advanced authoring signals
- **THEN** the operator can review that summary without leaving the structured editor flow

### Requirement: The role library summarizes execution-relevant properties for selection
The system SHALL present the role library as a distinguishable catalog rather than a name-only list. Each role entry MUST expose the metadata and governance cues needed to compare roles quickly, including version, tags, inheritance markers when present, and visible execution or safety signals when configured.

The catalog panel header SHALL display the section title and description on the first line and the action buttons (Marketplace and New Role) on a second line below, so that both lines are fully legible within the 260px panel width without truncation or overflow.

#### Scenario: Review role differences from the list view
- **WHEN** the operator scans the role library
- **THEN** each role entry shows enough summary information to distinguish role purpose, version, and inheritance state without opening the editor first

#### Scenario: Role uses review or path restrictions
- **WHEN** a role requires review or defines allowed or denied paths
- **THEN** the role library surfaces those constraints as visible summary cues instead of hiding them only inside the edit workspace

#### Scenario: Catalog header is fully legible at 260px width
- **WHEN** the catalog panel renders at its default width of 260px
- **THEN** the title "Role Library" and its description text are fully visible on their own line
- **AND** the Marketplace and New Role buttons appear on a separate line below the title
- **AND** no button text is truncated or overlaps the title text

### Requirement: Operators can manage role skills through a structured skills section
The system SHALL provide a structured Skills section in the role management workspace so operators can manage role skill references without editing raw YAML. Each visible skill row MUST allow editing the skill `path` and `auto_load` flag, and create/edit/template/inheritance flows MUST prefill the current role skill list instead of dropping it during draft construction or save.

#### Scenario: Edit a role with mixed skill entries
- **WHEN** an operator opens an existing role or template that already declares both auto-loaded and on-demand skill references
- **THEN** the role workspace loads those skill entries into labeled editable rows with their current `path` and `auto_load` values
- **THEN** saving the role preserves the edited skill rows instead of discarding them from the serialized payload

#### Scenario: Invalid skill rows block save
- **WHEN** an operator leaves a skill path blank or repeats the same skill path multiple times in the draft
- **THEN** the workspace shows inline validation feedback for the invalid skill rows
- **THEN** the workspace SHALL NOT submit the role draft until the invalid skill configuration is corrected

### Requirement: Role catalog and summary surfaces expose skill loading cues
The system SHALL surface role skill cues in both the role catalog and the role draft summary so operators can compare roles by their declared skill tree without inspecting raw YAML. At minimum, the UI MUST show whether a role has skills configured, how many are auto-loaded versus on-demand, and enough path-level detail to distinguish one role's skill profile from another.

#### Scenario: Compare roles from the catalog view
- **WHEN** the operator scans the role library
- **THEN** each role entry with configured skills shows visible skill summary cues such as total count, auto-load versus on-demand split, or representative skill paths
- **THEN** the operator can distinguish a skill-rich role from a role with no declared skills without opening the editor first

#### Scenario: Draft summary updates as skill rows change
- **WHEN** an operator adds, removes, or toggles skill rows in the role workspace
- **THEN** the draft summary updates to reflect the current skill counts and loading-mode split
- **THEN** the operator can review those skill cues before saving the draft

### Requirement: Role workspace provides authoring guidance and YAML visibility
The system SHALL surface field-level guidance and section-level helper content that stays aligned with the role authoring documentation and current PRD terminology. The workspace SHALL also provide a YAML-oriented view of the current draft plus preview or sandbox entry points in the same authoring context so operators do not have to infer how structured inputs map back to the canonical role asset or where to validate the draft.

#### Scenario: Operator reviews guidance for advanced fields
- **WHEN** the operator enters sections such as advanced identity, knowledge, security, collaboration, or trigger authoring
- **THEN** the workspace shows concise guidance that explains what the supported fields mean in the current AgentForge role model
- **THEN** that guidance uses the same concepts and ordering as the role authoring documentation instead of unrelated UI-only wording

#### Scenario: Operator inspects YAML preview of the current draft
- **WHEN** the operator asks to inspect the role definition as YAML before saving
- **THEN** the workspace shows the current draft or preview response in a YAML-oriented form that maps back to the canonical role asset
- **THEN** the operator can compare structured inputs against the serialized definition without manually rebuilding the YAML in another tool

### Requirement: Role workspace can launch preview and sandbox flows from the current draft
The system SHALL let operators open authoritative preview and sandbox flows directly from the role workspace using either an existing persisted role or the current unsaved draft.

#### Scenario: Unsaved draft launches preview or sandbox
- **WHEN** the operator requests preview or sandbox while editing a new or modified role draft that has not yet been saved
- **THEN** the workspace submits the current draft to the backend preview or sandbox surface without first persisting it to `roles/<role-id>/role.yaml`
- **THEN** the operator receives authoritative preview or sandbox results tied to the current draft state

#### Scenario: Operator launches preview or sandbox from the current workspace context
- **WHEN** the operator is editing a role draft and wants to validate it before saving
- **THEN** the preview and sandbox actions remain visible from the same authoring context as the guidance and YAML surfaces
- **THEN** the operator does not need to leave the current authoring flow to find validation entry points

### Requirement: Role authoring layout remains usable across responsive breakpoints
The system SHALL adapt the role management workspace layout across desktop, medium-width, and narrow-width viewports without dropping the role library, authoring controls, or validation context. The responsive behavior MUST preserve access to the role catalog, active authoring section, execution summary, YAML preview, and preview or sandbox entry points, even if those surfaces change presentation from side-by-side rails to stacked panels, tabs, drawers, or other equivalent patterns.

#### Scenario: Desktop layout keeps catalog, editor, and context visible
- **WHEN** the operator uses the role workspace at a desktop-width viewport
- **THEN** the workspace presents the role catalog, main editor, and context surfaces in a simultaneous multi-panel layout
- **THEN** the operator can compare roles, edit the draft, and inspect summary or YAML context without replacing the main editor view

#### Scenario: Medium-width layout preserves authoring context without horizontal overflow
- **WHEN** the operator uses the role workspace at a medium-width viewport where three parallel columns no longer fit comfortably
- **THEN** the workspace collapses to a layout that still keeps section navigation and validation context discoverable without requiring horizontal scrolling
- **THEN** the operator can still reach the role library and summary or preview surfaces with one clear interaction path

#### Scenario: Narrow-width layout keeps authoring flow intact
- **WHEN** the operator uses the role workspace on a narrow viewport
- **THEN** the workspace presents the role library, authoring sections, and context surfaces in a stacked or switched layout that preserves the recommended create-edit-preview flow
- **THEN** the operator can still save, preview, sandbox, and inspect guidance or YAML without losing the active draft state

### Requirement: Role workspace supports advanced role authoring without lossy saves
The system SHALL let operators inspect and edit the current documented advanced role configuration from the role workspace without requiring raw file surgery for routine authoring. The workspace MUST cover the supported advanced fields that the current Go role contract already preserves, including advanced capability settings, richer tool-host configuration, structured knowledge details, memory metadata, and controlled override authoring. Saving a draft that changes only part of the role MUST preserve the rest of the loaded advanced manifest instead of serializing a reduced UI subset.

#### Scenario: Editing a role with advanced configuration does not drop untouched fields
- **WHEN** the operator opens a role that already includes advanced custom settings, shared knowledge source details, memory metadata, or overrides
- **AND** the operator edits only one advanced subsection or a basic field in the workspace
- **THEN** the save payload still preserves the untouched advanced sections from the loaded draft
- **THEN** the role remains semantically equivalent after reload except for the fields the operator actually changed

#### Scenario: Workspace blocks malformed advanced subsection input
- **WHEN** the operator enters malformed key-value data, invalid shared-source metadata, or invalid override content in an advanced authoring subsection
- **THEN** the workspace surfaces inline validation in the relevant subsection before submission
- **THEN** the workspace SHALL NOT submit a lossy or malformed role payload to the role API

### Requirement: Role workspace reveals advanced field provenance and save impact

The role workspace SHALL display provenance indicators on all advanced fields (custom settings, MCP servers, knowledge sources, memory settings, collaboration, triggers, overrides) showing whether each value is inherited from a parent role, copied from a template, or explicitly set by the operator. The context rail Advanced Authoring panel SHALL distinguish inherited values from explicitly-set values. Provenance SHALL be computed by comparing the current draft against the parent manifest and template manifest.

#### Scenario: Custom setting inherited from parent role
- **WHEN** operator opens a role that extends a parent, and the parent has custom_settings entries
- **THEN** each inherited custom setting row displays an "inherited" provenance badge
- **AND** explicitly added settings display an "explicit" provenance badge

#### Scenario: MCP server copied from template
- **WHEN** operator creates a role from a template that has MCP server entries
- **THEN** each template-derived MCP server row displays a "template" provenance badge
- **AND** newly added servers display an "explicit" provenance badge

#### Scenario: Context rail shows provenance summary
- **WHEN** operator views the Advanced Authoring panel in the context rail
- **THEN** each advanced field category shows count of inherited, template-derived, and explicit values
- **AND** individual values are labeled with their provenance source

#### Scenario: Provenance updates on field edit
- **WHEN** operator modifies an inherited custom setting value
- **THEN** the provenance badge changes from "inherited" to "explicit" indicating an override

### Requirement: Role workspace can discover and select skills from the authoritative catalog
The system SHALL let operators discover and select skills from the authoritative role-skill catalog inside the existing role workspace while preserving the current structured skill-row editing model. Each skill row MUST continue to support direct editing of `path` and `auto_load`, but the workspace MUST also offer catalog-backed selection so operators do not have to memorize valid skill paths before authoring a role.

#### Scenario: Operator selects a skill from the catalog
- **WHEN** the operator opens the Skills section and the repository catalog contains discovered skills
- **THEN** the workspace shows a searchable or otherwise browsable list of available skills from that catalog in the same authoring flow
- **THEN** selecting a catalog skill fills the current row with the canonical role-compatible path while preserving the operator's ability to set or change the `auto_load` flag

#### Scenario: Operator falls back to a manual skill path
- **WHEN** the operator enters a skill path that does not resolve to a discovered catalog skill
- **THEN** the workspace preserves that manual path in the current row instead of discarding it
- **THEN** the row is marked as an unresolved manual reference while save behavior continues to block only blank or duplicate paths

### Requirement: Role workspace explains skill resolution and provenance cues
The system SHALL surface role-skill resolution and provenance cues in the role library, live draft summary, and review context so operators can understand whether configured skills are resolved from the repository catalog, inherited from a parent role, copied from a template, or still unresolved manual references.

#### Scenario: Operator compares resolved and unresolved skills from the role library
- **WHEN** the operator scans the role library or draft summary for a role whose skill list mixes catalog-resolved entries and unresolved manual references
- **THEN** the UI shows enough state to distinguish resolved skills from unresolved ones without opening raw YAML
- **THEN** the operator can tell whether the role's skill tree is fully backed by the current repository catalog or still contains manual references

#### Scenario: Review context shows inherited or template-derived skill provenance
- **WHEN** the operator reviews a draft whose skills came from a template, inheritance, or explicit edits in the current workspace
- **THEN** the review context identifies which skills are inherited, template-derived, or explicitly added in the current draft
- **THEN** the operator can understand the effective skill tree before saving without leaving the current authoring flow

### Requirement: Dead code cleanup removes unused role form dialog
The codebase SHALL NOT contain the legacy `role-form-dialog.tsx` component if it has no active import references. Before removal, a codebase-wide search SHALL confirm the component is unused.

#### Scenario: Legacy role form dialog confirmed unused and removed
- **WHEN** a codebase search finds zero import references to `role-form-dialog`
- **THEN** `role-form-dialog.tsx` and `role-form-dialog.test.tsx` are deleted
- **AND** no runtime or build errors result from the removal

### Requirement: Role workspace exposes skill compatibility impact before save
The system SHALL surface role-skill compatibility cues throughout the existing authoring flow so operators can understand not only whether a skill resolves from the repository catalog, but also what dependencies it brings in, what tool capabilities it declares, and whether the current role configuration fully covers those needs. These cues MUST remain visible in the Skills section, draft summary, role library, and review context without requiring raw YAML inspection.

#### Scenario: Skills section shows dependency and tool-demand details for a selected skill
- **WHEN** the operator selects or types a skill path in the Skills section and that skill resolves from the authoritative catalog
- **THEN** the workspace shows the skill's direct dependency paths and declared tool requirements alongside its label and provenance cues
- **THEN** any compatibility warning or blocking state for the current role configuration is visible from the same authoring flow

#### Scenario: Summary and library react to compatibility changes
- **WHEN** the operator changes skill rows, toggles `auto_load`, or edits the role's tool configuration in a way that changes skill compatibility
- **THEN** the draft summary and review context update to reflect the current direct-versus-transitive skill impact and blocking versus warning state
- **THEN** the role library can distinguish a fully compatible skill-backed role from a role that still contains compatibility warnings or blockers

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

### Requirement: Role surfaces expose downstream reference governance before deletion
The role library, role workspace, and delete confirmation flow SHALL surface authoritative downstream reference governance for the currently selected role before destructive actions proceed. At minimum, the operator MUST be able to distinguish blocking current consumers from advisory historical references without leaving the role management flow.

#### Scenario: Delete review shows blocking member and workflow consumers
- **WHEN** the operator selects a role that is currently referenced by project agent members or installed workflow/plugin bindings and initiates delete from the role management surface
- **THEN** the UI shows the blocking consumers grouped by surface before the delete is confirmed
- **THEN** the delete affordance explains that those bindings must be updated or removed first
- **THEN** the operator does not need to infer blockers from a later generic API failure

#### Scenario: Delete review allows cleanup when only advisory history remains
- **WHEN** the operator initiates delete for a role whose current governance view contains only advisory historical references
- **THEN** the UI shows that historical context will remain visible after deletion
- **THEN** the delete action remains available because no blocking current consumer exists

