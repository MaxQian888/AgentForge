## ADDED Requirements

### Requirement: Preview and sandbox explain role-skill resolution without conflating it with runtime readiness
The system SHALL include role-skill resolution context in preview and sandbox authoring feedback so operators can see which configured skill references are resolved by the authoritative repository catalog, which are inherited or template-derived, and which remain unresolved manual references. This feedback MUST distinguish authoring-level skill-resolution warnings from runtime readiness blockers because the current execution profile does not auto-load role skills directly.

#### Scenario: Preview shows effective skill-tree resolution for an unsaved child draft
- **WHEN** the operator previews an unsaved draft whose effective skill tree includes inherited skills, template-copied skills, and newly added manual references
- **THEN** the preview result shows the effective role skills after inheritance or template application
- **THEN** the authoring flow can indicate for each effective skill whether it is catalog-resolved, inherited, template-derived, or unresolved

#### Scenario: Unresolved skill references surface as authoring warnings instead of runtime blockers
- **WHEN** the operator runs preview or sandbox for a valid draft that includes one or more unresolved manual skill references
- **THEN** the result returns authoring feedback that those skill references are unresolved in the current repository catalog
- **THEN** the flow does not report that condition as a runtime readiness blocker unless some separate runtime prerequisite is actually missing
