## ADDED Requirements

### Requirement: Repo-local skills resolve into normalized runtime bundles
The system SHALL resolve role skill references against repo-local `skills/**/SKILL.md` assets into deterministic runtime bundles before execution-profile construction. Each resolved bundle MUST preserve the canonical role-facing path plus the runtime-facing metadata needed for loading, including display metadata, instruction body, declared dependency metadata, and whether the skill was requested directly or loaded through dependency closure.

#### Scenario: Auto-load skill resolves from repo-local assets
- **WHEN** a role declares `skills/react` with `auto_load: true` and the repository contains `skills/react/SKILL.md`
- **THEN** the runtime skill resolver returns a normalized bundle for `skills/react` with canonical path, parsed metadata, and instruction content
- **THEN** execution-profile construction uses that normalized bundle instead of treating the role skill as a path-only string

#### Scenario: Skill dependencies resolve once in deterministic order
- **WHEN** a resolved skill declares required skills in its metadata and those required skills are available in the same repo-local skill tree
- **THEN** the system resolves the dependency closure once per canonical skill path in deterministic order
- **THEN** the runtime bundle records whether each loaded skill came from the role manifest directly or from skill dependency resolution

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
The system SHALL treat unresolved or invalid auto-load skill references, or unresolved dependencies of an auto-load skill, as blocking runtime diagnostics for preview, sandbox, and agent spawn. Unresolved non-auto-load skill references SHALL remain preserved in the canonical role definition but SHALL surface as warning-level diagnostics instead of silent drops.

#### Scenario: Missing auto-load skill blocks execution-facing projection
- **WHEN** a role declares `skills/react` with `auto_load: true` and that skill cannot be resolved from the supported repo-local skill roots
- **THEN** preview or sandbox readiness marks the skill issue as blocking for execution-facing projection
- **THEN** agent spawn refuses to start execution with a role profile that would otherwise silently omit that auto-load skill

#### Scenario: Missing on-demand skill remains warning-only
- **WHEN** a role declares `skills/future-experiment` with `auto_load: false` and the current repository does not resolve that skill
- **THEN** preview or sandbox preserves the unresolved reference in the role's effective skill tree
- **THEN** the unresolved skill is surfaced as a warning-level inventory gap instead of a blocking execution error
