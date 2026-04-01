# role-skill-runtime Specification

## Purpose
Define how repo-local role skills resolve into runtime-ready skill bundles, how auto-load versus on-demand skill policies affect execution-facing projection, and how blocking versus warning diagnostics are derived before preview, sandbox, or agent execution.
## Requirements
### Requirement: Repo-local skills resolve into normalized runtime bundles
The system SHALL resolve role skill references against repo-local `skills/**/SKILL.md` assets into deterministic runtime bundles before execution-profile construction. Each resolved bundle MUST preserve the canonical role-facing path plus the runtime-facing metadata needed for loading and compatibility evaluation, including display metadata, instruction body, direct dependency metadata, declared tool requirements, and whether the skill was requested directly or loaded through dependency closure.

#### Scenario: Auto-load skill resolves from repo-local assets
- **WHEN** a role declares `skills/react` with `auto_load: true` and the repository contains `skills/react/SKILL.md`
- **THEN** the runtime skill resolver returns a normalized bundle for `skills/react` with canonical path, parsed metadata, instruction content, and declared tool requirements
- **THEN** execution-profile construction uses that normalized bundle instead of treating the role skill as a path-only string

#### Scenario: Skill dependencies resolve once in deterministic order
- **WHEN** a resolved skill declares required skills in its metadata and those required skills are available in the same repo-local skill tree
- **THEN** the system resolves the dependency closure once per canonical skill path in deterministic order
- **THEN** the runtime bundle records both the direct dependency metadata and whether each loaded skill came from the role manifest directly or from skill dependency resolution

### Requirement: Runtime projection distinguishes loaded skills from on-demand inventory
The system SHALL project auto-load role skills into execution-ready context while preserving non-auto-load role skills as available inventory metadata. The projection MUST make it explicit which skills were fully loaded into runtime prompt context and which skills remain available but unloaded.

#### Scenario: Mixed skill tree produces loaded and available projections
- **WHEN** a role declares both auto-load and non-auto-load skills
- **THEN** the normalized execution profile includes prompt-ready context for the resolved auto-load skills
- **THEN** the normalized execution profile also includes summarized metadata for the non-auto-load skills without embedding their full instruction bodies

#### Scenario: Inherited auto-load skills remain part of the loaded projection
- **WHEN** a child role inherits an auto-load skill from its parent and adds an additional non-auto-load skill of its own
- **THEN** the execution-facing skill projection reflects the fully merged effective skill tree after inheritance
- **THEN** the inherited auto-load skill is treated as loaded runtime context and the appended non-auto-load skill remains available inventory

### Requirement: Auto-load failures block runtime projection while on-demand gaps warn
The system SHALL treat unresolved or invalid auto-load skill references, unresolved dependencies of an auto-load skill, or unmet declared tool requirements for auto-load skill closure as blocking runtime diagnostics for preview, sandbox, and agent or workflow execution. Unresolved non-auto-load skill references or unmet declared tool requirements for non-auto-load skills SHALL remain preserved in the canonical role definition but SHALL surface as warning-level diagnostics instead of silent drops.

#### Scenario: Missing auto-load skill blocks execution-facing projection
- **WHEN** a role declares `skills/react` with `auto_load: true` and that skill cannot be resolved from the supported repo-local skill roots
- **THEN** preview or sandbox readiness marks the skill issue as blocking for execution-facing projection
- **THEN** agent spawn refuses to start execution with a role profile that would otherwise silently omit that auto-load skill

#### Scenario: Auto-load skill with unmet tool requirement blocks execution
- **WHEN** an auto-load skill or one of its auto-loaded dependencies declares a required tool capability that is not covered by the role's effective tool inventory after canonical normalization
- **THEN** the execution profile includes a blocking compatibility diagnostic for that skill path
- **THEN** preview, sandbox, and runtime launch surfaces report that incompatibility before any execution begins

#### Scenario: On-demand tool mismatch remains warning-only
- **WHEN** a role declares `skills/testing` with `auto_load: false` and that skill's declared tool requirements are not covered by the current role tool inventory
- **THEN** preview or sandbox preserves the unresolved compatibility state in the role's available skill inventory
- **THEN** the incompatibility is surfaced as a warning-level inventory gap instead of a blocking execution error

