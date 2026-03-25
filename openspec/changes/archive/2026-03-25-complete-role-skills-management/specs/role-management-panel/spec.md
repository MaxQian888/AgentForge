## ADDED Requirements

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
