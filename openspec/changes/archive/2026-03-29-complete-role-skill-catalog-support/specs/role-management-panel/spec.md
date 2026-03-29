## ADDED Requirements

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
