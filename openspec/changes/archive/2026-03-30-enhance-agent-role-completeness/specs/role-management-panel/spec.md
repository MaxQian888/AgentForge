## MODIFIED Requirements

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

## ADDED Requirements

### Requirement: Dead code cleanup removes unused role form dialog
The codebase SHALL NOT contain the legacy `role-form-dialog.tsx` component if it has no active import references. Before removal, a codebase-wide search SHALL confirm the component is unused.

#### Scenario: Legacy role form dialog confirmed unused and removed
- **WHEN** a codebase search finds zero import references to `role-form-dialog`
- **THEN** `role-form-dialog.tsx` and `role-form-dialog.test.tsx` are deleted
- **AND** no runtime or build errors result from the removal
