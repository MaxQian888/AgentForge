## MODIFIED Requirements

### Requirement: Operators can edit roles through a structured manifest-aware form
The system SHALL provide a role management workspace that edits the normalized role manifest through a documentation-aligned authoring layout instead of a single undifferentiated long form. The workspace MUST organize the current role API contract and advanced PRD-backed role sections into clearly labeled authoring groups that match the operator mental model from `docs/role-authoring-guide.md`, including template or inheritance setup, identity, advanced identity, capabilities, knowledge, security, collaboration, triggers, preview, and save actions. The workspace MUST keep navigation between those groups clear while editing and MUST block save when required fields, nested advanced fields, or supported list inputs are invalid.

#### Scenario: Edit an existing advanced role through grouped authoring sections
- **WHEN** the operator opens an existing role for editing
- **THEN** the workspace loads the role into clearly separated authoring groups with stable section labels instead of one undifferentiated scroll surface
- **THEN** the operator can move between those groups without losing track of where template, inheritance, preview, and save actions live

#### Scenario: Save is blocked by invalid structured data
- **WHEN** the operator enters invalid required metadata, malformed advanced nested config, or malformed supported list values in the role workspace
- **THEN** the system shows inline validation feedback in the relevant authoring group before submission
- **THEN** the workspace SHALL NOT submit a partial or invalid manifest payload to the role API

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

#### Scenario: Operator launches preview or sandbox from the current workspace context
- **WHEN** the operator is editing a role draft and wants to validate it before saving
- **THEN** the preview and sandbox actions remain visible from the same authoring context as the guidance and YAML surfaces
- **THEN** the operator does not need to leave the current authoring flow to find validation entry points

## ADDED Requirements

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
