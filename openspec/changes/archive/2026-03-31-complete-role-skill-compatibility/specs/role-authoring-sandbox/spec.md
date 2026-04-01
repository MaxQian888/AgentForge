## MODIFIED Requirements

### Requirement: Preview and sandbox explain role-skill resolution without conflating it with runtime readiness
The system SHALL include role-skill resolution and compatibility context in preview and sandbox authoring feedback so operators can see which configured skill references are resolved by the authoritative repository catalog, which are inherited or template-derived, which transitive skills will load through dependency closure, and what declared tool requirements apply to the effective role-skill tree. This feedback MUST distinguish authoring-level resolution warnings from runtime readiness blockers such as missing auto-load skills or incompatible auto-load tool requirements.

#### Scenario: Preview shows effective skill-tree resolution and compatibility for an unsaved child draft
- **WHEN** the operator previews an unsaved draft whose effective skill tree includes inherited skills, template-copied skills, newly added manual references, and auto-load dependencies
- **THEN** the preview result shows the effective role skills after inheritance or template application together with any transitive loaded skills
- **THEN** the authoring flow can indicate for each effective skill whether it is catalog-resolved, inherited, template-derived, unresolved, and whether its declared tool requirements are currently compatible with the effective role

#### Scenario: Sandbox separates warning-only inventory gaps from blocking compatibility failures
- **WHEN** the operator runs sandbox for a valid draft that includes both a non-auto-load unresolved manual skill reference and an auto-load skill whose declared tool requirements are not covered by the effective role tool inventory
- **THEN** the sandbox result reports the unresolved non-auto-load skill as warning-only authoring or inventory context
- **THEN** the auto-load compatibility failure is returned as a blocking readiness issue before any probe is executed
