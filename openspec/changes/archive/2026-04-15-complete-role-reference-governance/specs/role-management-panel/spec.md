## ADDED Requirements

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

