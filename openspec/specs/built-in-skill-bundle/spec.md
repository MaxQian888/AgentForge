# built-in-skill-bundle Specification

## Purpose
Define the official repository-owned built-in skill inventory, its market-facing metadata, and the verification rules that keep the built-in skill marketplace surface aligned with real repo-local skill packages.

## Requirements
### Requirement: Official built-in skill bundle is explicitly declared
The repository SHALL maintain an explicit built-in skill bundle that lists the official repo-owned skill packages exposed as built-in skills in the current checkout. Each bundle entry MUST identify the built-in skill id, the package root under `skills/`, and the market-facing metadata required to render provenance-aware marketplace cards and detail views. Only bundle entries that resolve to a valid repo-local skill package MAY be treated as official built-in skills.

#### Scenario: Built-in skill bundle exposes official repo-owned skills
- **WHEN** the current checkout includes official repo-owned skills such as `skills/react` or `skills/testing`
- **THEN** the built-in skill bundle lists those skills explicitly instead of requiring the marketplace workspace to infer official membership from a directory scan
- **THEN** each listed skill can be mapped back to a real `skills/<id>/SKILL.md` package root in the current repository

#### Scenario: Non-bundled skill packages do not become official built-ins automatically
- **WHEN** a repo-local skill package exists under `skills/` but is not declared in the built-in skill bundle
- **THEN** the system MUST NOT surface that package as an official built-in skill in marketplace-specific built-in sections
- **THEN** the package may remain available to repo-local authoring or runtime discovery without being promoted to the official built-in marketplace surface

### Requirement: Built-in skill bundle remains verifiable against real skill packages
The system SHALL provide a verification path that checks built-in skill bundle entries against the real repo-local skill packages they declare. Verification MUST fail when a declared built-in skill package is missing, cannot be resolved to the canonical `skills/<id>` path, or lacks the minimum package data required to generate a marketplace-facing preview.

#### Scenario: Bundle verification succeeds for previewable built-in skills
- **WHEN** every built-in skill bundle entry resolves to a valid skill package with a readable `SKILL.md`
- **THEN** verification succeeds without requiring the marketplace workspace to discover drift at runtime
- **THEN** the verified built-in skills remain eligible for built-in marketplace rendering

#### Scenario: Bundle verification rejects drifted built-in skill metadata
- **WHEN** a built-in skill bundle entry points at a missing package root or a package that no longer produces a valid canonical skill path
- **THEN** verification fails explicitly for that built-in skill entry
- **THEN** the system MUST NOT silently continue treating that entry as a truthful built-in marketplace asset
